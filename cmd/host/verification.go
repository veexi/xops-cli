package host

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wentf9/xops-cli/cmd/utils"
	"github.com/wentf9/xops-cli/internal/terminal"
	"github.com/wentf9/xops-cli/pkg/adapter"
	"github.com/wentf9/xops-cli/pkg/config"
	"github.com/wentf9/xops-cli/pkg/i18n"
)

type inventoryVerifier func(context.Context, *config.Repository, string, config.ConnectionSnapshot) error

func verifyInventoryNode(ctx context.Context, repo *config.Repository, nodeID string, bundle config.ConnectionSnapshot) error {
	preview, err := repo.PreviewConnection(nodeID, bundle.Node, bundle.Host, bundle.Identity)
	if err != nil {
		return err
	}
	registry, err := utils.GetCredentialRegistry(repo.Snapshot())
	if err != nil {
		return fmt.Errorf("initialize verification credentials: %w", err)
	}
	return adapter.VerifyConnection(ctx, preview, nodeID, registry)
}

func confirmFailedVerification(cmd *cobra.Command, nodeID string, verifyErr error) error {
	message := i18n.Tf("inventory_verify_not_saved", map[string]any{"Node": nodeID, "Error": verifyErr})
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), message); err != nil {
		return errors.Join(verifyErr, fmt.Errorf("report verification failure: %w", err))
	}
	if err := cmd.Context().Err(); err != nil {
		return errors.Join(verifyErr, err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()
	prompt := i18n.T("inventory_confirm_save_unverified") + " [y/N]: "
	answer, err := terminal.NewPrompter(cmd.InOrStdin(), cmd.ErrOrStderr()).ReadLine(ctx, prompt)
	if err != nil {
		return errors.Join(verifyErr, fmt.Errorf("confirm saving unverified node (use --skip-verify for unattended addition): %w", err))
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("%s: %w", i18n.T("inventory_save_declined"), verifyErr)
	}
	return nil
}

func verifyAndConfirmAddedNode(cmd *cobra.Command, repo *config.Repository, nodeID string, bundle config.ConnectionSnapshot, auth utils.HostInfo, verify inventoryVerifier) error {
	draft := importDraft{bundle: bundle, addr: auth}
	if err := verify(cmd.Context(), repo, nodeID, draft.verificationBundle()); err != nil {
		return confirmFailedVerification(cmd, nodeID, err)
	}
	return nil
}
