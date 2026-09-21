package tui

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/goleak"
)

func TestProgramSizeWatcherReportsChangesAndStops(t *testing.T) {
	baseline := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, baseline)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	var width atomic.Int32
	width.Store(80)
	var queries atomic.Int32
	events := make(chan tea.Msg, 8)
	stop := pollProgramSize(ctx, func() (int, int, error) {
		queries.Add(1)
		return int(width.Load()), 24, nil
	}, func(msg tea.Msg) {
		select {
		case events <- msg:
		case <-ctx.Done():
		}
	})
	defer stop()
	assertSize := func(w int) {
		t.Helper()
		select {
		case msg := <-events:
			if msg != (tea.WindowSizeMsg{Width: w, Height: 24}) {
				t.Fatalf("size event = %#v, want width=%d", msg, w)
			}
		case <-ctx.Done():
			t.Fatal("size watcher did not deliver a size")
		}
	}
	assertSize(80)
	width.Store(120)
	assertSize(120)
	width.Store(60)
	assertSize(60)
	stop()
	stop() // cleanup remains safe after an explicit shutdown
	before := queries.Load()
	time.Sleep(150 * time.Millisecond)
	if queries.Load() != before {
		t.Fatal("size watcher continued querying after it was joined")
	}
	select {
	case msg := <-events:
		t.Fatalf("unexpected duplicate size: %#v", msg)
	default:
	}
}

func TestProgramSizeWatcherCancellationAndErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "query failure"}[fail], func(t *testing.T) {
			baseline := goleak.IgnoreCurrent()
			defer goleak.VerifyNone(t, baseline)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			sentinel := errors.New("console unavailable")
			events := make(chan tea.Msg, 1)
			stop := pollProgramSize(ctx, func() (int, int, error) {
				if fail {
					return 0, 0, sentinel
				}
				return 80, 24, nil
			}, func(msg tea.Msg) { events <- msg })
			defer stop()
			select {
			case msg := <-events:
				if fail {
					failure, ok := msg.(programSizeErrorMsg)
					if !ok || !errors.Is(failure.err, sentinel) {
						t.Fatalf("query error was lost: %#v", msg)
					}
					runner := &programModel{}
					_, cmd := runner.Update(failure)
					if !errors.Is(runner.err, sentinel) || cmd == nil {
						t.Fatal("size error did not stop the program with context")
					}
					if _, ok := cmd().(tea.QuitMsg); !ok {
						t.Fatal("size error did not request shutdown")
					}
				}
			case <-time.After(time.Second):
				t.Fatal("watcher did not query the console")
			}
			cancel()
			stop()
		})
	}
}
