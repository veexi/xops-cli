package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/wentf9/xops-cli/pkg/adapter"
	"github.com/wentf9/xops-cli/pkg/config"
)

func TestNewNodeFormVerificationPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		verifyErr    error
		skip, saved  bool
	}{
		{name: "verified", saved: true},
		{name: "skip", skip: true, saved: true},
		{name: "failed_no", verifyErr: errors.New("refused"), answer: "n"},
		{name: "failed_default_no", verifyErr: errors.New("refused"), answer: "enter"},
		{name: "failed_escape", verifyErr: errors.New("refused"), answer: "esc"},
		{name: "failed_yes", verifyErr: errors.New("refused"), answer: "y", saved: true},
		{name: "timeout_yes", verifyErr: context.DeadlineExceeded, answer: "y", saved: true},
		{name: "canceled", verifyErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTestRepository(t, nil)
			m := &Model{repository: repo, ctx: t.Context(), state: viewForm,
				lastSize:  tea.WindowSizeMsg{Width: 100, Height: 30},
				formState: &nodeFormState{user: "root", address: "192.0.2.1", port: "2222", authType: "password", password: "draft-secret", skipVerify: tc.skip},
			}
			calls := 0
			m.connectionConfig.verifyConnection = func(_ context.Context, preview *config.Provider, id string, _ adapter.CredentialResolver) error {
				calls++
				if repo.Snapshot().Nodes.Count() != 0 {
					t.Error("form saved before verification")
				}
				_, host, identity, err := preview.Resolve(id)
				if err != nil || host.Port != 2222 || identity.Password != "draft-secret" {
					t.Errorf("invalid connection preview: %v", err)
				}
				return tc.verifyErr
			}
			cmd := m.saveFormCmd()
			if cmd == nil {
				t.Fatal("missing form command")
			}
			_, next := m.Update(cmd())
			if !tc.skip && tc.verifyErr == nil {
				if next == nil {
					t.Fatal("verified node did not proceed to save")
				}
				m.Update(next())
			} else if tc.answer != "" {
				assertFormVerificationPrompt(t, m)
				key := tea.KeyPressMsg{Code: []rune(tc.answer)[0], Text: tc.answer}
				if tc.answer == "enter" {
					key = tea.KeyPressMsg{Code: tea.KeyEnter}
				}
				if tc.answer == "esc" {
					key = tea.KeyPressMsg{Code: tea.KeyEscape}
				}
				_, save := m.updateVerificationConfirmation(key)
				if tc.saved {
					if save == nil {
						t.Fatal("confirmation did not start saving")
					}
					m.Update(save())
				}
			}
			if (repo.Snapshot().Nodes.Count() == 1) != tc.saved {
				t.Fatalf("saved nodes = %d, expected saved=%v", repo.Snapshot().Nodes.Count(), tc.saved)
			}
			if (calls == 0) != tc.skip {
				t.Fatalf("verification calls = %d", calls)
			}
		})
	}
}

func assertFormVerificationPrompt(t *testing.T, m *Model) {
	t.Helper()
	if m.repository.Snapshot().Nodes.Count() != 0 || m.formVerifyErr == nil || !strings.Contains(m.status, "[y/N]") {
		t.Fatalf("failure did not request confirmation without saving: %s", m.status)
	}
	before := m.status
	m.Update(tickMsg{generation: m.statusGeneration})
	if m.status != before {
		t.Fatal("confirmation disappeared while waiting for input")
	}
}
