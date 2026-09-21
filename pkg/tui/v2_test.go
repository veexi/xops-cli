package tui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

func newV2TestModel(t *testing.T) *Model {
	t.Helper()
	cfg := newFormCredentialTestConfiguration("")
	node, _ := cfg.Nodes.Get(formCredentialTestNodeID)
	node.Tags = []string{"prod"}
	cfg.Nodes.Set(formCredentialTestNodeID, node)
	m, err := NewModel(newTestRepository(t, cfg), WithContext(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeTUITestResource(t, &m) })
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return &m
}

func TestV2ListKeysAndPaste(t *testing.T) {
	m := newV2TestModel(t)
	space := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	m.Update(space)
	if !m.list.SelectedItem().(*nodeItem).selected {
		t.Fatal("space did not select the node")
	}
	m.Update(tea.KeyReleaseMsg{Code: tea.KeySpace})
	if !m.list.SelectedItem().(*nodeItem).selected {
		t.Fatal("key release toggled the selection")
	}
	m.Update(space)
	if m.list.SelectedItem().(*nodeItem).selected {
		t.Fatal("space did not deselect the node")
	}
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.PasteMsg{Content: "server"})
	if m.list.FilterState() != list.Filtering || m.list.FilterValue() != "server" {
		t.Fatalf("filter did not receive pasted text: %q", m.list.FilterValue())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.list.SettingFilter() {
		t.Fatal("escape did not close filtering")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c did not produce a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestV2FormInputAndNavigation(t *testing.T) {
	m := newV2TestModel(t)
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.state != viewForm || m.form == nil {
		t.Fatal("new-node form did not open")
	}
	m.Update(tea.PasteMsg{Content: "test-alias"})
	if m.formState.alias != "test-alias" {
		t.Fatalf("form paste = %q", m.formState.alias)
	}
	// Huh emits a command to advance focus; run it as the event loop does.
	_, cmd := m.updateForm(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd == nil {
		t.Fatal("down did not schedule field navigation")
	}
	m.Update(cmd())
	m.Update(tea.PasteMsg{Content: "test-user"})
	if m.formState.user != "test-user" {
		t.Fatalf("next field input = %q", m.formState.user)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != viewList {
		t.Fatal("escape did not return to the list")
	}
}

func TestV2MonitorSpaceTogglesPause(t *testing.T) {
	m := monitorModel{}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !m.paused || cmd != nil {
		t.Fatal("space did not pause monitoring")
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if m.paused || cmd == nil {
		t.Fatal("space did not resume monitoring")
	}
}

func TestV2LogInputPasteAndResize(t *testing.T) {
	size := tea.WindowSizeMsg{Width: 80, Height: 24}
	selector := newLogSelectModel(t.Context(), "node", nil, size)
	selector.isManual = true
	selector, _ = selector.Update(tea.PasteMsg{Content: "/var/log/example.log"})
	_, cmd := selector.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("manual log path was not accepted")
	}
	if msg, ok := cmd().(logFileSelectedMsg); !ok || msg.file != "/var/log/example.log" {
		t.Fatalf("selected log = %#v", msg)
	}

	streamer := newLogStreamerModel(t.Context(), 1, nil, "example.log", size)
	defer closeTUITestResource(t, &streamer)
	streamer.lines = []string{"info ready", "error failed"}
	streamer.filterOn = true
	streamer, _ = streamer.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	streamer, _ = streamer.Update(tea.PasteMsg{Content: "error"})
	if streamer.textInput.Value() != "error" || strings.Contains(streamer.viewport.View(), "info ready") {
		t.Fatal("pasted search did not filter logs")
	}
	if !strings.Contains(streamer.viewport.View(), "error failed") {
		t.Fatal("matching log line disappeared")
	}
	streamer, _ = streamer.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	if streamer.viewport.Width() != 60 || streamer.viewport.Height() != 9 {
		t.Fatal("log viewport did not resize")
	}
	streamer, _ = streamer.Update(tea.WindowSizeMsg{Width: 0, Height: 1})
	if streamer.viewport.Width() != 1 || streamer.viewport.Height() != 1 {
		t.Fatal("small terminal dimensions were not bounded")
	}
}

func TestV2ViewsKeepAlternateScreen(t *testing.T) {
	m := newV2TestModel(t)
	m.initForm("")
	m.rebuildTagSelectForm()
	m.logSelect = newLogSelectModel(t.Context(), "node", nil, m.lastSize)
	m.logStreamer = newLogStreamerModel(t.Context(), 1, nil, "example.log", m.lastSize)
	for _, state := range []viewState{viewList, viewForm, viewTagSelect, viewMonitor, viewLogSelect, viewLogStream} {
		m.state = state
		view := m.View()
		if !view.AltScreen || view.Content == "" {
			t.Fatalf("state %v did not render in the alternate screen", state)
		}
	}
}

func TestV2TagMultiSelectSpace(t *testing.T) {
	m := newV2TestModel(t)
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if m.state != viewTagSelect || m.tagForm == nil {
		t.Fatal("tag form did not open")
	}
	m.Update(huh.NextField())
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if len(m.selectedTags) != 1 || m.selectedTags[0] != "prod" {
		t.Fatalf("tag selection = %v", m.selectedTags)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != viewList {
		t.Fatal("escape did not close tag selection")
	}
}

func TestV2FormsUseDiscoveredBackground(t *testing.T) {
	for _, dark := range []bool{false, true} {
		m := newV2TestModel(t)
		background := color.White
		if dark {
			background = color.Black
		}
		m.Update(tea.BackgroundColorMsg{Color: background})
		m.initForm("")
		m.rebuildTagSelectForm()
		for _, form := range []*huh.Form{m.form, m.tagForm} {
			var called, gotDark bool
			form.WithTheme(huh.ThemeFunc(func(isDark bool) *huh.Styles {
				called, gotDark = true, isDark
				return huh.ThemeCharm(isDark)
			}))
			form.View()
			if !called || gotDark != dark {
				t.Fatalf("form background = %v, want dark=%v", gotDark, dark)
			}
		}
	}
}
