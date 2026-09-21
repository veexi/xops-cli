package tui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/wentf9/xops-cli/pkg/i18n"
)

func (m *Model) handleFormVerification(msg configurationMutationMsg) (tea.Model, tea.Cmd) {
	if msg.err == nil {
		save := m.formVerifiedSave
		m.formVerifiedSave = nil
		return m, m.beginConfigurationMutation(configurationMutationForm, msg.nodeID, 0, save)
	}
	if errors.Is(msg.err, context.Canceled) {
		m.formVerifiedSave = nil
		m.status = errorStyle.Render(i18n.T("inventory_save_declined"))
		_, cmd := m.initForm("")
		return m, cmd
	}
	m.formVerifyErr = msg.err
	m.status = errorStyle.Render(i18n.Tf("inventory_verify_not_saved", map[string]any{"Node": msg.nodeID, "Error": msg.err})) +
		"\n" + i18n.T("inventory_confirm_save_unverified") + " [y/N]"
	return m, nil
}

func (m *Model) updateVerificationConfirmation(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch key.String() {
	case "y", "Y":
		save, nodeID := m.formVerifiedSave, m.formVerifyNodeID
		m.formVerifyErr, m.formVerifiedSave = nil, nil
		return *m, m.beginConfigurationMutation(configurationMutationForm, nodeID, 0, save)
	case "n", "N", "enter", "esc":
		m.formVerifyErr, m.formVerifiedSave = nil, nil
		m.status = i18n.T("inventory_save_declined")
		return m.initForm("")
	default:
		return *m, nil
	}
}
