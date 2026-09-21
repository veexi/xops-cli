package tui

import (
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/wentf9/xops-cli/internal/terminal"
)

// programInput owns both the physical read and the decoder. Stopping drains
// events until StreamEvents returns: its channel sends are not cancellable.
type programInput struct {
	ctx      context.Context
	cancel   context.CancelFunc
	reader   terminal.PromptInput
	events   chan uv.Event
	err      error // published by closing events
	once     sync.Once
	closeErr error
}

type programInputMsg struct {
	input *programInput
	event uv.Event
	open  bool
}

func startProgramInput(parent context.Context, reader terminal.PromptInput, termType string) *programInput {
	ctx, cancel := context.WithCancel(parent)
	input := &programInput{ctx: ctx, cancel: cancel, reader: reader, events: make(chan uv.Event)}
	go func() {
		defer close(input.events)
		input.err = uv.NewTerminalReader(reader, termType).StreamEvents(ctx, input.events)
	}()
	return input
}

func (i *programInput) next() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-i.ctx.Done():
			return nil
		case event, open := <-i.events:
			return programInputMsg{input: i, event: event, open: open}
		}
	}
}

func (i *programInput) Close() error {
	i.once.Do(func() {
		i.cancel()
		interruptErr := i.reader.Interrupt()
		// Also releases a decoder already blocked sending a burst or partial
		// escape sequence after the program stopped consuming input.
		for range i.events {
		}
		i.closeErr = errors.Join(interruptErr, i.reader.Close(), i.err)
	})
	if i.closeErr != nil {
		return fmt.Errorf("close TUI input: %w", i.closeErr)
	}
	return nil
}
