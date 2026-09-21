//go:build linux

package tui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"golang.org/x/sys/unix"
)

func TestTUIEnterUsesOwnedSessionAndCurrentRepository(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeTUITestResource(t, master) })
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(filepath.Join("/dev/pts", strconv.Itoa(number)), os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeTUITestResource(t, slave) })
	before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	ui := &terminalTestInteraction{password: "verified-password"}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	model, repo := terminalTestModel(t, newMemoryCredentialStore(), ui, WithContext(ctx))
	if _, cmd := model.handleEnter(); cmd == nil {
		t.Fatal("Enter did not schedule a terminal action")
	}
	action := model.terminalConnection
	var output bytes.Buffer
	action.SetStdin(slave)
	action.SetStdout(&output)
	action.SetStderr(&output)
	if err := action.Run(); err != nil {
		t.Fatal(err)
	}
	model.handleTerminalConnection(terminalConnectionResult{action: action})
	after, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil || *before != *after {
		t.Fatalf("terminal not restored: %v", err)
	}
	snapshot, err := repo.ResolveConnection(formCredentialTestNodeID)
	if err != nil || snapshot.Identity.LoginPasswordRef == nil || snapshot.Identity.Password != "" {
		t.Fatal("Enter did not update the same repository")
	}
	if model.terminalConnection != nil || model.listRevision != repo.View().Revision {
		t.Fatal("TUI returned with stale references")
	}
	if output.String() != "tui-shell\n" {
		t.Fatalf("unexpected shell output: %q", output.String())
	}
}

// terminalRoundTripModel drives a real Bubble Tea event loop through SSH and
// quits once the terminal action has returned to the node list.
type terminalRoundTripModel struct {
	*Model
	result *terminalConnectionResult
}

func (m *terminalRoundTripModel) Init() tea.Cmd {
	return func() tea.Msg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
}

func (m *terminalRoundTripModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.Model.Update(msg)
	if result, ok := msg.(terminalConnectionResult); ok {
		m.result = &result
		return m, tea.Quit
	}
	return m, cmd
}

func TestV2ProgramRestoresTerminalAfterSSH(t *testing.T) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTUITestResource(t, master)
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(filepath.Join("/dev/pts", strconv.Itoa(number)), os.O_RDWR|unix.O_NOCTTY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTUITestResource(t, slave)
	before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	model, _ := terminalTestModel(t, newMemoryCredentialStore(), &terminalTestInteraction{password: "verified-password"}, WithContext(ctx))
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	roundTrip := &terminalRoundTripModel{Model: model}
	var output bytes.Buffer
	if err := runProgram(ctx, roundTrip, slave, &output); err != nil {
		t.Fatal(err)
	}
	if roundTrip.result == nil || roundTrip.result.err != nil {
		t.Fatalf("SSH round trip failed: %#v", roundTrip.result)
	}
	if model.state != viewList || model.terminalConnection != nil {
		t.Fatal("SSH did not return to the node list")
	}
	after, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil || *before != *after {
		t.Fatalf("Bubble Tea did not restore terminal attributes: %v", err)
	}
	if !strings.Contains(output.String(), "tui-shell") {
		t.Fatal("SSH session output is missing")
	}
	for _, sequence := range []string{"\x1b[?1049h", "\x1b[?1049l"} {
		if strings.Count(output.String(), sequence) < 2 {
			t.Fatalf("alternate screen was not released and restored around SSH: %q", sequence)
		}
	}
}
