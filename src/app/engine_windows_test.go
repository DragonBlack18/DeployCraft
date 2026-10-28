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
	"testing"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/platform"
	"github.com/DragonBlack18/DeployCraft/src/runner"
)

// This test launches a REAL Windows process whose source code belongs to this
// repository. It never installs a third-party application or changes HKLM.
func TestWindowsNativeProcessE2E(t *testing.T) {
	kit := t.TempDir()
	payloadDir := filepath.Join(kit, "payloads", "fake-installer")
	if err := os.MkdirAll(payloadDir, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(payloadDir, "fake-installer.exe")
	build := exec.Command("go", "build", "-trimpath", "-o", executable, "../../tools/fake-installer")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("cannot build the local test fixture: %v\n%s", err, output)
	}
	payload, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	manifest := domain.Package{
		Schema:   1,
		ID:       "fake-installer",
		Name:     "Safe local integration fixture",
		Version:  "1.0",
		Platform: domain.Platform{OS: "windows", Arch: []string{"x64"}, MinBuild: 17763},
		Payload: domain.Payload{
			Kind:   "file",
			Path:   "payloads/fake-installer/fake-installer.exe",
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(len(payload)),
		},
		Install: domain.Install{
			Driver:       "exe",
			TimeoutSec:   10,
			SuccessCodes: []int{0},
		},
	}
	encodedArg := func(s string) json.RawMessage {
		out, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(out)
	}
	newEngine := func(p domain.Package) *InstallEngine {
		return &InstallEngine{
			Catalog:      domain.Catalog{Packages: map[string]domain.Package{p.ID: p}},
			KitRoot:      kit,
			StateRoot:    t.TempDir(),
			Registry:     platform.NewRegistry(),
			Paths:        platform.NewPaths(),
			Process:      runner.New(),
			IsAdmin:      func() (bool, error) { return true, nil },
			WindowsBuild: platform.WindowsBuild,
			postCheckTimeout: 200 * time.Millisecond,
		}
	}
	lab := InstallOptions{LabUnsignedCatalog: true, Confirmed: true}

	t.Run("installed_then_skipped", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "confirmed.txt")
		p := manifest
		p.Install.Args = []json.RawMessage{encodedArg("--write-file"), encodedArg(marker)}
		p.Detect = domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}}
		e := newEngine(p)
		report, err := e.InstallOne(context.Background(), p.ID, lab)
		if err != nil {
			t.Fatal(err)
		}
		if report.ExitCode != 0 || report.Results[0].Outcome != domain.OutcomeInstalled {
			t.Fatalf("native install not confirmed: %+v", report)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("native process did not create marker: %v", err)
		}
		second, err := e.InstallOne(context.Background(), p.ID, lab)
		if err != nil {
			t.Fatal(err)
		}
		if second.Results[0].Outcome != domain.OutcomeSkipped {
			t.Fatalf("idempotent second run failed: %+v", second)
		}
	})
	t.Run("exit_zero_without_detection_is_failure", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "not-created.txt")
		p := manifest
		p.Install.Args = []json.RawMessage{encodedArg("--exit"), encodedArg("0")}
		p.Detect = domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}}
		e := newEngine(p)
		report, err := e.InstallOne(context.Background(), p.ID, lab)
		if err != nil {
			t.Fatal(err)
		}
		if report.Results[0].Code != "E-RUN-004" || report.Results[0].Reason != domain.ReasonNotDetected {
			t.Fatalf("false success: %+v", report)
		}
	})
	t.Run("nonzero_exit_is_failure", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "not-created.txt")
		p := manifest
		p.Install.Args = []json.RawMessage{encodedArg("--exit"), encodedArg("17")}
		p.Detect = domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}}
		e := newEngine(p)
		report, err := e.InstallOne(context.Background(), p.ID, lab)
		if err != nil {
			t.Fatal(err)
		}
		if report.Results[0].Code != "E-RUN-003" || report.Results[0].ExitCode == nil || *report.Results[0].ExitCode != 17 {
			t.Fatalf("exit code lost: %+v", report)
		}
	})
	t.Run("timeout_kills_real_process", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "not-created.txt")
		p := manifest
		p.Install.Args = []json.RawMessage{encodedArg("--sleep"), encodedArg("5s")}
		p.Install.TimeoutSec = 1
		p.Detect = domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}}
		e := newEngine(p)
		report, err := e.InstallOne(context.Background(), p.ID, lab)
		if err != nil {
			t.Fatal(err)
		}
		if report.Results[0].Code != "E-RUN-001" || report.Results[0].Reason != domain.ReasonTimeout {
			t.Fatalf("timeout was not reported: %+v", report)
		}
	})
}
