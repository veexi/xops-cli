//go:build windows && integration

package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
)

const navigationHelperEnv = "XOPS_TUI_NAVIGATION_HELPER"

type navigationOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (o *navigationOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.Write(p)
}

func (o *navigationOutput) text() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.String()
}

func TestWindowsTUINavigationConPTY(t *testing.T) {
	if marker := os.Getenv(navigationHelperEnv); marker != "" {
		runNavigationHelper(t, marker)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	console, output := newNavigationConsole(t)

	marker := filepath.Join(t.TempDir(), "progress")
	pid, handle, err := console.Spawn(os.Args[0],
		[]string{os.Args[0], "-test.run=^TestWindowsTUINavigationConPTY$", "-test.timeout=15s"},
		&syscall.ProcAttr{Env: append(os.Environ(), navigationHelperEnv+"="+marker)})
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(errors.Join(err, windows.TerminateProcess(windows.Handle(handle), 1), windows.CloseHandle(windows.Handle(handle))))
	}
	if err := windows.CloseHandle(windows.Handle(handle)); err != nil {
		t.Error(err)
	}
	type result struct {
		state *os.ProcessState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		state, err := process.Wait()
		done <- result{state, err}
	}()
	finished := false
	defer func() {
		if !finished {
			if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
			<-done
		}
	}()

	// Each phase uses a real Windows console input stream and the production
	// owned decoder. F12 checkpoints assert the model value before switching.
	for phase := range 4 {
		waitNavigationStage(t, ctx, marker, strconv.Itoa(phase), output)
		text := "one two\x1b[1;5DX\x1b[1;5CY\x1b[24~"
		if phase == 0 {
			text = "/" + text
		}
		if _, err := io.WriteString(console, text); err != nil {
			t.Fatal(err)
		}
	}
	waitNavigationStage(t, ctx, marker, "4", output)
	select {
	case result := <-done:
		finished = true
		if result.err != nil || !result.state.Success() {
			t.Fatalf("ConPTY child failed: %v: %s", result.err, output.text())
		}
	case <-ctx.Done():
		t.Fatalf("ConPTY child did not exit: %s", output.text())
	}
}

type navigationModel struct {
	*Model
	marker string
	phase  int
	err    error
}

func (m *navigationModel) progress() {
	if err := os.WriteFile(m.marker, []byte(strconv.Itoa(m.phase)), 0600); err != nil {
		m.err = err
	}
}

func (m *navigationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.Code == tea.KeyF12 {
		var value string
		switch m.phase {
		case 0:
			value = m.list.FilterValue()
		case 1:
			value = m.formState.alias
		case 2:
			value = m.logSelect.textInput.Value()
		case 3:
			value = m.logStreamer.textInput.Value()
		}
		if value != "one XtwoY" {
			m.err = fmt.Errorf("phase %d word navigation produced %q", m.phase, value)
			return m, tea.Quit
		}
		m.phase++
		var cmd tea.Cmd
		switch m.phase {
		case 1:
			m.list.ResetFilter()
			_, cmd = m.handleNew()
		case 2:
			m.state = viewLogSelect
			m.logSelect = newLogSelectModel(m.ctx, "test", nil, m.lastSize)
			m.logSelect.fetching = false
			m.logSelect.isManual = true
		case 3:
			m.state = viewLogStream
			m.logStreamer = newLogStreamerModel(m.ctx, 1, nil, "test.log", m.lastSize)
			m.logStreamer.isSearching = true
			cmd = m.logStreamer.textInput.Focus()
		case 4:
			cmd = tea.Quit
		}
		m.progress()
		return m, cmd
	}
	_, cmd := m.Model.Update(msg)
	if _, ok := msg.(tea.WindowSizeMsg); ok && m.phase == 0 {
		m.progress()
	}
	return m, cmd
}

func runNavigationHelper(t *testing.T, marker string) {
	model := &navigationModel{Model: newV2TestModel(t), marker: marker}
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	if err := runProgram(ctx, model, os.Stdin, os.Stdout); err != nil {
		t.Fatal(err)
	}
	if model.err != nil {
		t.Fatal(model.err)
	}
	if model.phase != 4 {
		t.Fatalf("only completed %d navigation phases", model.phase)
	}
}

func newNavigationConsole(t *testing.T) (*conpty.ConPty, *navigationOutput) {
	t.Helper()
	console, err := conpty.New(100, 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	output := &navigationOutput{}
	readDone := make(chan error, 1)
	// Closing the owned ConPTY after the child exits unblocks this reader.
	go func() {
		_, err := io.Copy(output, console)
		readDone <- err
	}()
	t.Cleanup(func() {
		closeTUITestResource(t, console)
		select {
		case err := <-readDone:
			if err != nil && !errors.Is(err, os.ErrClosed) && !errors.Is(err, windows.ERROR_BROKEN_PIPE) {
				t.Errorf("read ConPTY output: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("ConPTY output reader did not exit")
		}
	})

	return console, output
}

func waitNavigationStage(t *testing.T, ctx context.Context, marker, want string, output *navigationOutput) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(marker)
		if err == nil && string(data) == want {
			return
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for stage %s: %s", want, output.text())
		case <-ticker.C:
		}
	}
}
