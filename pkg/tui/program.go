package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// Run owns terminal input through shutdown and every SSH/unlock handoff.
// Input is borrowed; only its cancellable duplicate is closed. A nil input
// selects stdin or opens the controlling terminal when stdin is redirected.
func Run(ctx context.Context, model *Model, input *os.File, output io.Writer) (retErr error) {
	if input == nil {
		input = os.Stdin
		if !term.IsTerminal(input.Fd()) {
			ttyIn, ttyOut, err := tea.OpenTTY()
			if err != nil {
				return fmt.Errorf("open TUI terminal: %w", err)
			}
			defer func() {
				closeErr := ttyIn.Close()
				if ttyOut != ttyIn {
					closeErr = errors.Join(closeErr, ttyOut.Close())
				}
				if closeErr != nil {
					retErr = errors.Join(retErr, fmt.Errorf("close TUI terminal: %w", closeErr))
				}
			}()
			input = ttyIn
		}
	}
	return runProgram(ctx, model, input, output)
}

type terminalExecutor func(tea.ExecCommand, tea.ExecCallback) tea.Cmd

type terminalModel interface {
	tea.Model
	setTerminalExecutor(terminalExecutor)
}

func (m *Model) setTerminalExecutor(execute terminalExecutor) { m.terminalExec = execute }

func (m *Model) execTerminal(command tea.ExecCommand, callback tea.ExecCallback) tea.Cmd {
	if m.terminalExec != nil {
		return m.terminalExec(command, callback)
	}
	return tea.Exec(command, callback)
}

type programModel struct {
	model terminalModel
	ctx   context.Context
	file  *os.File
	saved *term.State
	input *programInput
	err   error
}

func runProgram(ctx context.Context, model terminalModel, input *os.File, output io.Writer) (retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	runner := &programModel{model: model, ctx: ctx, file: input}
	if term.IsTerminal(input.Fd()) {
		saved, err := term.GetState(input.Fd())
		if err != nil {
			return fmt.Errorf("capture TUI terminal state: %w", err)
		}
		runner.saved = saved
	}
	defer func() { retErr = errors.Join(retErr, runner.stopInput()) }()
	if err := runner.startInput(); err != nil {
		return err
	}
	model.setTerminalExecutor(runner.execute)
	defer model.setTerminalExecutor(nil)
	// An empty, non-terminal reader keeps Bubble Tea's decoder inert and
	// immediately joinable, including during Exec. The owned decoder delivers
	// events as commands so it never blocks on Program.Send during a handoff.
	options := []tea.ProgramOption{
		tea.WithContext(ctx), tea.WithInput(strings.NewReader("")), tea.WithOutput(output),
	}
	if width, height, err := term.GetSize(input.Fd()); err == nil {
		options = append(options, tea.WithWindowSize(width, height))
	}
	program := tea.NewProgram(runner, options...)
	stopResize := watchProgramSize(ctx, output, program.Send)
	defer stopResize()
	_, err := program.Run()
	return errors.Join(err, runner.err)
}

func (m *programModel) startInput() error {
	if err := m.ctx.Err(); err != nil {
		return err
	}
	if m.saved != nil {
		if err := makeProgramRaw(m.file.Fd()); err != nil {
			return fmt.Errorf("enable TUI terminal input: %w", err)
		}
	}
	reader, err := duplicateProgramInput(m.file)
	if err != nil {
		return fmt.Errorf("open TUI input: %w", err)
	}
	m.input = startProgramInput(m.ctx, reader, os.Getenv("TERM"))
	return nil
}

func (m *programModel) stopInput() error {
	var inputErr, restoreErr error
	if m.input != nil {
		inputErr = m.input.Close()
		m.input = nil
	}
	if m.saved != nil {
		if err := term.Restore(m.file.Fd(), m.saved); err != nil {
			restoreErr = fmt.Errorf("restore TUI terminal state: %w", err)
		}
	}
	return errors.Join(inputErr, restoreErr)
}

func (m *programModel) Init() tea.Cmd {
	return tea.Batch(m.model.Init(), m.input.next(), tea.RequestBackgroundColor)
}

func (m *programModel) View() tea.View { return m.model.View() }

type programExecResult struct{ message tea.Msg }

func (m *programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case programSizeErrorMsg:
		m.err = errors.Join(m.err, msg.err)
		return m, tea.Quit
	case programInputMsg:
		if msg.input != m.input {
			return m, nil // input queued before a terminal handoff
		}
		if !msg.open {
			m.err = errors.Join(m.err, msg.input.err)
			return m, tea.Quit
		}
		// Return the native event through Bubble Tea's event loop so capability,
		// resize, mouse and keyboard reports receive its standard translation.
		return m, tea.Sequence(func() tea.Msg { return msg.event }, m.input.next())
	case programExecResult:
		_, cmd := m.model.Update(msg.message)
		if m.input == nil {
			return m, tea.Batch(cmd, tea.Quit)
		}
		return m, tea.Batch(cmd, m.input.next())
	default:
		_, cmd := m.model.Update(msg)
		return m, cmd
	}
}

func (m *programModel) execute(command tea.ExecCommand, callback tea.ExecCallback) tea.Cmd {
	return tea.Exec(&programExec{owner: m, command: command}, func(err error) tea.Msg {
		return programExecResult{message: callback(err)}
	})
}

type programExec struct {
	owner   *programModel
	command tea.ExecCommand
}

func (c *programExec) SetStdin(io.Reader)    { c.command.SetStdin(c.owner.file) }
func (c *programExec) SetStdout(w io.Writer) { c.command.SetStdout(w) }
func (c *programExec) SetStderr(w io.Writer) { c.command.SetStderr(w) }

func (c *programExec) Run() error {
	if err := c.owner.stopInput(); err != nil {
		c.owner.err = errors.Join(c.owner.err, err)
		return err
	}
	runErr := c.command.Run()
	if err := c.owner.startInput(); err != nil {
		c.owner.err = errors.Join(c.owner.err, err)
		return errors.Join(runErr, err)
	}
	return runErr
}
