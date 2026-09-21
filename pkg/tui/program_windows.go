//go:build windows

package tui

import (
	"context"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/wentf9/xops-cli/internal/terminal"
)

// The interactive input driver consumes keyboard records only. Windows has no
// SIGWINCH listener in Bubble Tea, so observe the output console separately.
func watchProgramSize(ctx context.Context, output io.Writer, send func(tea.Msg)) func() {
	file, ok := output.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(file.Fd()) {
		return func() {}
	}
	return pollProgramSize(ctx, func() (int, int, error) { return term.GetSize(file.Fd()) }, send)
}

func duplicateProgramInput(file *os.File) (terminal.PromptInput, error) {
	return terminal.DuplicateInteractiveInput(file)
}

func makeProgramRaw(fd uintptr) error {
	_, err := term.MakeRaw(fd)
	return err
}
