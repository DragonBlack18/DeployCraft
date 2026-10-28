package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func TestVerifyDetectsTampering(t *testing.T) {
	p := examplePackage("alpha")
	root := writeCatalog(t, []domain.Package{p}, nil)
	dir := filepath.Join(root, "payloads", "alpha")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "alpha.exe")
	original := []byte("ok")
	h := sha256.Sum256(original)
	p.Payload.SHA256 = hex.EncodeToString(h[:])
	p.Payload.Size = int64(len(original))
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	c := domain.Catalog{Packages: map[string]domain.Package{"alpha": p}}
	if issues := Verify(root, c); len(issues) != 0 {
		t.Fatalf("arquivo válido recusado: %v", issues)
	}
	if err := os.WriteFile(file, []byte("no"), 0600); err != nil {
		t.Fatal(err)
	}
	issues := Verify(root, c)
	if len(issues) != 1 || issues[0].Code != "E-VER-001" {
		t.Fatalf("esperado hash mismatch: %v", issues)
	}
	if err := os.WriteFile(file, []byte("long"), 0600); err != nil {
		t.Fatal(err)
	}
	issues = Verify(root, c)
	if len(issues) != 1 || issues[0].Code != "E-VER-002" {
		t.Fatalf("esperado size mismatch: %v", issues)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	issues = Verify(root, c)
	if len(issues) != 1 || issues[0].Code != "E-VER-003" {
		t.Fatalf("esperado missing payload: %v", issues)
	}
}
