package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

type programSizeErrorMsg struct{ err error }

// pollProgramSize emits the initial console size and subsequent changes without
// consuming keyboard records. send must unblock when the program exits; callers
// stop and join this watcher after Run returns, including startup failure.
func pollProgramSize(parent context.Context, query func() (int, int, error), send func(tea.Msg)) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var previous tea.WindowSizeMsg
		for {
			if ctx.Err() != nil {
				return
			}
			width, height, err := query()
			if err != nil {
				send(programSizeErrorMsg{err: fmt.Errorf("read TUI console size: %w", err)})
				return
			}
			size := tea.WindowSizeMsg{Width: width, Height: height}
			if width > 0 && height > 0 && size != previous {
				send(size)
				previous = size
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return sync.OnceFunc(func() {
		cancel()
		<-done
	})
}
