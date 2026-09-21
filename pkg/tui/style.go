package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/list"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// formTheme keeps Charm's accents while restoring its readable option colors.
// Huh v2.0.3 reverses the light/dark foreground for ordinary select options.
func formTheme(isDark bool) *huh.Styles {
	theme := huh.ThemeCharm(isDark)
	foreground := lipgloss.LightDark(isDark)(lipgloss.Color("235"), lipgloss.Color("252"))
	for _, field := range []*huh.FieldStyles{&theme.Focused, &theme.Blurred} {
		field.Option = field.Option.Foreground(foreground)
		field.UnselectedOption = field.UnselectedOption.Foreground(foreground)
	}
	return theme
}

var (
	appStyle = lipgloss.NewStyle().Padding(0, 2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // Gray

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")). // Green
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")). // Red
			Bold(true)

	// 定义基础颜色
	selectedColor = lipgloss.Color("5") // Magenta (光标选中行)
)

// applyListTheme updates styles without rebuilding items, filtering or selection.
func applyListTheme(model *list.Model, isDark bool) {
	model.Styles = list.DefaultStyles(isDark)
	model.Help.Styles = help.DefaultStyles(isDark)
	model.FilterInput.SetStyles(model.Styles.Filter)
	model.Paginator.ActiveDot = model.Styles.ActivePaginationDot.String()
	model.Paginator.InactiveDot = model.Styles.InactivePaginationDot.String()
}

func (m *Model) hasDarkBackground() bool {
	return m.backgroundColor == nil || m.backgroundColor.IsDark()
}

func (m *Model) applyNodeListTheme() {
	isDark := m.hasDarkBackground()
	applyListTheme(&m.list, isDark)
	m.list.SetDelegate(newNodeDelegate(isDark))
}

func (m *logSelectModel) applyTheme(isDark bool) {
	applyListTheme(&m.list, isDark)
	delegate := list.NewDefaultDelegate()
	delegate.Styles = list.NewDefaultItemStyles(isDark)
	m.list.SetDelegate(delegate)
}
