//go:build !windows

package tui

import (
	"context"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/charmbracelet/x/termios"
	"github.com/wentf9/xops-cli/internal/terminal"
)

// Bubble Tea already watches SIGWINCH on Unix.
func watchProgramSize(context.Context, io.Writer, func(tea.Msg)) func() {
	return func() {}
}

func duplicateProgramInput(file *os.File) (terminal.PromptInput, error) {
	return terminal.DuplicatePromptInput(file)
}

// Bubble Tea has no physical input and therefore renders for cooked output.
// Preserve newline processing while disabling echo and line editing.
func makeProgramRaw(fd uintptr) error {
	if _, err := term.MakeRaw(fd); err != nil {
		return err
	}
	state, err := termios.GetTermios(int(fd))
	if err != nil {
		return err
	}
	// Darwin uses uint64 speeds; Linux uses uint32.
	//nolint:unconvert
	return termios.SetTermios(int(fd), uint32(state.Ispeed), uint32(state.Ospeed), nil, nil,
		map[termios.O]bool{termios.OPOST: true, termios.ONLCR: true}, nil, nil)
}
