package tui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/wentf9/xops-cli/pkg/i18n"
)

func TestListThemesFollowBackgroundAndRefresh(t *testing.T) {
	cfg := newFormCredentialTestConfiguration("")
	node, _ := cfg.Nodes.Get(formCredentialTestNodeID)
	node.Alias = []string{"second"}
	cfg.Nodes.Set("second", node)
	model, err := NewModel(newTestRepository(t, cfg), WithContext(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	defer closeTUITestResource(t, &model)
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	for _, background := range []color.Color{color.White, color.Black, color.White} {
		msg := tea.BackgroundColorMsg{Color: background}
		model.Update(msg)
		assertListTheme(t, model.list, msg.IsDark())

		// Refreshes after edits and SSH must not revert to the dark defaults.
		model.refreshList()
		model.updateList(model.lastSize)
		assertListTheme(t, model.list, msg.IsDark())

		// Connection completion creates the log list after background discovery.
		model.Update(logScannerConnectedMsg{nodeID: "node"})
		model.logSelect, _ = model.logSelect.Update(logScanResultMsg{files: []string{"/var/log/one.log"}})
		assertListTheme(t, model.logSelect.list, msg.IsDark())

		// A background change while the log list is open updates both lists.
		opposite := tea.BackgroundColorMsg{Color: color.White}
		if !msg.IsDark() {
			opposite.Color = color.Black
		}
		model.Update(opposite)
		assertListTheme(t, model.logSelect.list, opposite.IsDark())
		assertListTheme(t, model.list, opposite.IsDark())
	}
}

func assertListTheme(t *testing.T, model list.Model, dark bool) {
	t.Helper()
	want := lipgloss.Color("#1a1a1a")
	sequence := "38;2;26;26;26"
	if dark {
		want = lipgloss.Color("#dddddd")
		sequence = "38;2;221;221;221"
	}
	if model.Styles.StatusBarActiveFilter.GetForeground() != want {
		t.Fatalf("list chrome has wrong palette for dark=%v", dark)
	}
	view := model.View()
	if !strings.Contains(view, sequence) {
		t.Fatalf("normal list title has wrong palette for dark=%v: %q", dark, view)
	}
}

func TestListThemePreservesSelectionAndFilter(t *testing.T) {
	model := newV2TestModel(t)
	model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	selected := model.list.SelectedItem()
	model.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	model.Update(tea.PasteMsg{Content: "user"})
	model.Update(tea.BackgroundColorMsg{Color: color.White})
	if model.list.FilterValue() != "user" || !model.list.SettingFilter() {
		t.Fatal("background discovery reset the filter")
	}
	if !selected.(*nodeItem).selected {
		t.Fatal("background discovery cleared the checked node")
	}
	delegate := newNodeDelegate(false)
	if delegate.Styles.SelectedTitle.GetForeground() != selectedColor {
		t.Fatal("node cursor lost its custom highlight")
	}
}

func TestTagFormRendersReadableOptions(t *testing.T) {
	for _, dark := range []bool{true, false} {
		t.Run(map[bool]string{true: "dark", false: "light"}[dark], func(t *testing.T) {
			model := newTagColorTestModel(t)
			background := color.White
			normal, selected := "235", "#02BA84"
			if dark {
				background = color.Black
				normal, selected = "252", "#02BF87"
			}
			model.Update(tea.BackgroundColorMsg{Color: background})
			model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
			model.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
			if model.state != viewTagSelect {
				t.Fatal("tag form did not open")
			}
			// The action select is focused; the tag multiselect is blurred.
			assertRenderedOptionColor(t, model.View().Content, i18n.T("tui_tag_remove"), normal)
			model.Update(huh.NextField())
			// Now the tag multiselect is focused and the action select blurred.
			assertRenderedOptionColor(t, model.View().Content, i18n.T("tui_tag_remove"), normal)
			unselectedView := model.View().Content
			model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
			if len(model.selectedTags) != 1 {
				t.Fatal("tag was not selected")
			}
			assertRenderedOptionColor(t, unselectedView, model.selectedTags[0], normal)
			assertRenderedOptionColor(t, model.View().Content, model.selectedTags[0], selected)
			model.rebuildTagSelectForm()
			assertRenderedOptionColor(t, model.View().Content, i18n.T("tui_tag_remove"), normal)
		})
	}
}

func TestTagFormUpdatesRenderedColorsWhenBackgroundChanges(t *testing.T) {
	model := newTagColorTestModel(t)
	model.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	model.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	for _, dark := range []bool{true, false, true} {
		background, foreground := color.White, "235"
		if dark {
			background, foreground = color.Black, "252"
		}
		model.Update(tea.BackgroundColorMsg{Color: background})
		assertRenderedOptionColor(t, model.View().Content, i18n.T("tui_tag_remove"), foreground)
	}
}

// Assert the actual ANSI-colored label in the rendered form, not just the
// theme's background flag or style configuration.
func assertRenderedOptionColor(t *testing.T, view, label, foreground string) {
	t.Helper()
	expected := lipgloss.NewStyle().Foreground(lipgloss.Color(foreground)).Render(label)
	if !strings.Contains(view, expected) {
		t.Fatalf("option %q missing rendered foreground %s: %q", label, foreground, view)
	}
}

func newTagColorTestModel(t *testing.T) *Model {
	t.Helper()
	cfg := newFormCredentialTestConfiguration("")
	node, _ := cfg.Nodes.Get(formCredentialTestNodeID)
	node.Tags = []string{"prod", "staging"}
	cfg.Nodes.Set(formCredentialTestNodeID, node)
	model, err := NewModel(newTestRepository(t, cfg), WithContext(t.Context()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeTUITestResource(t, &model) })
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return &model
}
