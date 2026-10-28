package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func TestStagingVerifyAndTamper(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "payloads", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "install.exe")
	data := []byte("fake fixture - not executable")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	p := domain.Package{ID: "test", Payload: domain.Payload{Kind: "file", Path: "payloads/test/install.exe", Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}, Install: domain.Install{Driver: "exe"}}
	stage := filepath.Join(t.TempDir(), "staging")
	copied, err := Stage(root, stage, p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(copied)
	if err != nil || string(b) != string(data) {
		t.Fatalf("wrong copied bytes: %v", err)
	}
	if err := os.WriteFile(source, []byte(strings.Repeat("x", len(data))), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = Stage(root, filepath.Join(t.TempDir(), "run2"), p)
	var integrity *IntegrityError
	if !errors.As(err, &integrity) || integrity.Code != "E-VER-001" {
		t.Fatalf("expected E-VER-001, got %v", err)
	}
}
func TestPayloadSymlinkOutsideKitRejected(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "payloads", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.exe")
	if err := os.WriteFile(outside, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "setup.exe")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := safeSource(root, "payloads/test/setup.exe")
	if err == nil {
		t.Fatal("external symlink accepted")
	}
}
func TestDirectoryPayloadStaging(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "payloads", "multi")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("msi fixture")
	if err := os.WriteFile(filepath.Join(src, "setup.msi"), data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	p := domain.Package{ID: "multi", Payload: domain.Payload{Kind: "dir", Path: "payloads/multi", Entry: "setup.msi", Files: []domain.FileEntry{{Path: "setup.msi", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data))}}}, Install: domain.Install{Driver: "msi"}}
	entry, err := Stage(root, filepath.Join(t.TempDir(), "run"), p)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "setup.msi" {
		t.Fatalf("wrong entry %s", entry)
	}
}
