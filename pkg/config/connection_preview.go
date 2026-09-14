package config

import (
	"fmt"

	"github.com/wentf9/xops-cli/pkg/models"
)

// PreviewConnection builds an isolated, read-only connection provider for an
// inventory draft. Neither verification nor credential recording can save it.
// Private references keep the draft from changing a saved jump host's identity.
func (r *Repository) PreviewConnection(nodeID string, node models.Node, host models.Host, identity models.Identity) (*Provider, error) {
	if r == nil || nodeID == "" {
		return nil, fmt.Errorf("connection preview requires repository and node ID")
	}
	cfg := r.Snapshot()
	node.HostRef = privateHostReference(cfg, nodeID)
	node.IdentityRef = privateIdentityReference(cfg, nodeID)
	cfg.Nodes.Set(nodeID, cloneNode(node))
	cfg.Hosts.Set(node.HostRef, cloneHost(host))
	cfg.Identities.Set(node.IdentityRef, cloneIdentity(identity))
	if err := validateConfiguration(cfg); err != nil {
		return nil, fmt.Errorf("validate connection preview: %w", err)
	}
	return newProvider(cfg, r.provider.openSSH), nil
}
