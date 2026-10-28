package app

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/planner"
	"github.com/DragonBlack18/DeployCraft/src/trust"
)

func TestPinnedCatalogKeyAllowsSignedInstallAndRejectsTampering(t *testing.T) {
	e, proc, _ := newFixture(t)
	e.Catalog.Index.CatalogVersion = "2026.10.1"
	index := domain.Index{
		Schema: 1, CatalogVersion: "2026.10.1",
		Packages: []string{"packages/example.json"}, Profiles: []domain.Profile{},
	}
	dir := filepath.Join(e.KitRoot, "catalog", "packages")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{
		filepath.Join("catalog", "catalog.json"):             index,
		filepath.Join("catalog", "packages", "example.json"): e.Catalog.Packages["example"],
	} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(e.KitRoot, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	privPEM, pubPEM, err := trust.GeneratePEM()
	if err != nil {
		t.Fatal(err)
	}
	priv, err := trust.ParsePrivatePEM(privPEM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := trust.ParsePublicPEM(pubPEM)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := trust.Sign(e.KitRoot, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.KitRoot, "catalog", "catalog.sig"), sig, 0600); err != nil {
		t.Fatal(err)
	}
	e.TrustedCatalogPublicKey = ed25519.PublicKey(pub)
	sel := planner.Selection{IDs: []string{"example"}}
	report, err := e.RunBatch(context.Background(), sel, BatchOptions{InstallOptions: InstallOptions{Confirmed: true}}, "")
	if err != nil || report.ExitCode != 0 || proc.starts != 1 {
		t.Fatalf("signed installation failed: %+v, %v", report, err)
	}
	b, err := os.ReadFile(filepath.Join(e.KitRoot, "catalog", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.KitRoot, "catalog", "catalog.json"), append(b, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.RunBatch(context.Background(), sel, BatchOptions{InstallOptions: InstallOptions{Confirmed: true, LabUnsignedCatalog: true}}, ""); err == nil {
		t.Fatal("LAB flag bypassed a pinned public key")
	}
	if proc.starts != 1 {
		t.Fatal("tampered catalog executed a new process")
	}
}
