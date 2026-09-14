package adapter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/wentf9/xops-cli/pkg/models"
)

func TestInventoryVerificationNeverPersistsDraft(t *testing.T) {
	setTestHome(t)
	host, port, stop := startAdapterPrivilegeSSHServer(t, "verified-secret", "")
	t.Cleanup(stop)
	for _, password := range []string{"verified-secret", "wrong-secret"} {
		t.Run(password, func(t *testing.T) {
			repo, _, path, before := pendingAdapterRepository(t)
			preview, err := repo.PreviewConnection("draft", models.Node{}, models.Host{Address: host, Port: uint16(port)}, models.Identity{User: "root", AuthType: "password", Password: password})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err = VerifyConnection(ctx, preview, "draft", nil)
			if (err == nil) != (password == "verified-secret") {
				t.Fatalf("verification result: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || repo.Snapshot().Nodes.Count() != 0 {
				t.Fatal("verification saved draft metadata or credentials")
			}
		})
	}
}
