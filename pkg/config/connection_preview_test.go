package config

import (
	"testing"

	"github.com/wentf9/xops-cli/pkg/models"
)

func TestConnectionPreviewIsolatesSharedReferences(t *testing.T) {
	cfg := cloneConfiguration(nil)
	cfg.Hosts.Set("shared-host", models.Host{Address: "192.0.2.1", Port: 22})
	cfg.Identities.Set("shared-identity", models.Identity{User: "original", Password: "saved-secret", AuthType: "password"})
	for _, id := range []string{"jump", "target"} {
		cfg.Nodes.Set(id, models.Node{HostRef: "shared-host", IdentityRef: "shared-identity"})
	}
	repo, err := newTestRepositoryWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.PreviewConnection("target", models.Node{HostRef: "shared-host", IdentityRef: "shared-identity", ProxyJump: "jump"},
		models.Host{Address: "192.0.2.2", Port: 2222}, models.Identity{User: "draft", Password: "new-secret", AuthType: "password"})
	if err != nil {
		t.Fatal(err)
	}
	_, jumpHost, jumpIdentity, err := preview.Resolve("jump")
	if err != nil || jumpHost.Port != 22 || jumpIdentity.Password != "saved-secret" {
		t.Fatalf("draft changed jump host: %v", err)
	}
	_, host, identity, err := repo.Resolve("target")
	if err != nil || host.Port != 22 || identity.User != "original" {
		t.Fatalf("draft changed saved target: %v", err)
	}
	_, targetHost, targetIdentity, err := preview.Resolve("target")
	if err != nil || targetHost.Port != 2222 || targetIdentity.User != "draft" {
		t.Fatalf("preview lost candidate changes: %v", err)
	}
}
