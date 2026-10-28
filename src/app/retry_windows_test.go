//go:build windows

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/platform"
	"github.com/DragonBlack18/DeployCraft/src/runner"
)

// This executes a real Windows EXE compiled from test-only source, NOT a
// third-party installer. Backoffs are injected so no 15s waits are necessary.
func TestWindowsNativeTransientRetryE2E(t *testing.T) {
	kit := t.TempDir()
	payloadDir := filepath.Join(kit, "payloads", "retry-fixture")
	if err := os.MkdirAll(payloadDir, 0700); err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(payloadDir, "retry-fixture.exe")
	build := exec.Command("go", "build", "-trimpath", "-o", installer, "../../tools/fake-installer")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v\n%s", err, out)
	}
	payload, err := os.ReadFile(installer)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(payload)
	marker := filepath.Join(t.TempDir(), "detected.txt")
	attemptFile := filepath.Join(t.TempDir(), "attempts.txt")
	arg := func(s string) json.RawMessage {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(data)
	}
	pkg := domain.Package{
		ID: "retry-fixture", Name: "Local retry fixture", Version: "1.0", Schema: 1,
		Platform: domain.Platform{OS: "windows", Arch: []string{"x64"}, MinBuild: 17763},
		Payload:  domain.Payload{Kind: "file", Path: "payloads/retry-fixture/retry-fixture.exe", Size: int64(len(payload)), SHA256: hex.EncodeToString(hash[:])},
		Install: domain.Install{
			Driver: "exe", TimeoutSec: 10, SuccessCodes: []int{0}, RetryCodes: []int{1618},
			Args: []json.RawMessage{
				arg("--write-file"), arg(marker),
				arg("--attempt-file"), arg(attemptFile),
				arg("--retry-until"), arg("2"),
			},
		},
		Detect: domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}},
	}
	engine := &InstallEngine{
		Catalog: domain.Catalog{Packages: map[string]domain.Package{pkg.ID: pkg}},
		KitRoot: kit, StateRoot: t.TempDir(),
		Registry: platform.NewRegistry(), Paths: platform.NewPaths(), Process: runner.New(),
		IsAdmin: func() (bool, error) { return true, nil }, WindowsBuild: platform.WindowsBuild,
		postCheckTimeout: 250 * time.Millisecond,
	}
	var delays []time.Duration
	engine.retryWait = func(ctx context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}
	report, err := engine.InstallOne(context.Background(), pkg.ID, InstallOptions{Confirmed: true, LabUnsignedCatalog: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode != domain.ExitOK || report.Results[0].Outcome != domain.OutcomeInstalled {
		t.Fatalf("actual Windows retry failed: %+v", report)
	}
	raw, err := os.ReadFile(attemptFile)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 || len(delays) != 2 || delays[0] != 15*time.Second || delays[1] != 30*time.Second {
		t.Fatalf("wrong retries: attempts=%d delays=%v", attempts, delays)
	}
	second, err := engine.InstallOne(context.Background(), pkg.ID, InstallOptions{Confirmed: true, LabUnsignedCatalog: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Results[0].Outcome != domain.OutcomeSkipped {
		t.Fatalf("idempotency failed: %+v", second)
	}
	raw, err = os.ReadFile(attemptFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "3" {
		t.Fatalf("started installer again: %s", raw)
	}
}
