package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/journal"
	"github.com/DragonBlack18/DeployCraft/src/planner"
)

func TestPlanSelectionDoesNotWriteState(t *testing.T) {
	e, _, _ := newFixture(t)
	p, err := e.PlanSelection(planner.Selection{IDs: []string{"example"}}, planner.Options{UpgradeOutdated: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Phases) != 1 || len(p.Phases[0].Steps) != 1 || p.Phases[0].Steps[0].Action != domain.ActionInstall {
		t.Fatalf("unexpected preview %+v", p)
	}
	if _, err := os.Stat(filepath.Join(e.StateRoot, "state")); !os.IsNotExist(err) {
		t.Fatalf("dry planning created state: %v", err)
	}
}
func TestRunBatchInstallsAndSkipsSecondPass(t *testing.T) {
	e, runner, _ := newFixture(t)
	e.Catalog.Index.CatalogVersion = "2026.10.1"
	sel := planner.Selection{IDs: []string{"example"}}
	opt := BatchOptions{InstallOptions: labOptions()}
	first, err := e.RunBatch(context.Background(), sel, opt, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ExitCode != 0 || first.Results[0].Outcome != domain.OutcomeInstalled {
		t.Fatalf("first run %+v", first)
	}
	second, err := e.RunBatch(context.Background(), sel, opt, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.Results[0].Outcome != domain.OutcomeSkipped || runner.starts != 1 {
		t.Fatalf("reinstalled: %+v, starts=%d", second, runner.starts)
	}
	h, err := journal.New(e.StateRoot).ReadRun(first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if h.LastExitCode == nil || *h.LastExitCode != 0 {
		t.Fatal("journal did not close run")
	}
}
func TestRunBatchProtectsUnconfirmedAndUnsigned(t *testing.T) {
	e, runner, _ := newFixture(t)
	e.Catalog.Index.CatalogVersion = "2026.10.1"
	sel := planner.Selection{IDs: []string{"example"}}
	for _, opt := range []BatchOptions{
		{InstallOptions: InstallOptions{LabUnsignedCatalog: true}},
		{InstallOptions: InstallOptions{Confirmed: true}},
	} {
		_, err := e.RunBatch(context.Background(), sel, opt, "")
		if err == nil {
			t.Fatal("unsafe action allowed")
		}
	}
	if runner.starts != 0 {
		t.Fatal("process started before approval")
	}
}

type groupProc struct {
	markers map[string]string
	codes   map[string]int
	started []string
}
type groupRun struct {
	marker string
	code   int
}

func (g *groupProc) Start(cmd domain.Command) (domain.Running, error) {
	name := filepath.Base(cmd.Exe)
	g.started = append(g.started, name)
	marker, ok := g.markers[name]
	if !ok {
		return nil, errors.New("unexpected process " + name)
	}
	return &groupRun{marker: marker, code: g.codes[name]}, nil
}
func (g *groupRun) Wait(context.Context) (int, error) {
	if g.code == 0 || g.code == 3010 {
		if err := os.WriteFile(g.marker, []byte("ok"), 0600); err != nil {
			return -1, err
		}
	}
	return g.code, nil
}
func (g *groupRun) Kill() error { return nil }

func groupFixture(t *testing.T, reboot bool) (*InstallEngine, *groupProc, planner.Selection) {
	t.Helper()
	kit := t.TempDir()
	names := []string{"base", "app", "unrelated"}
	markers := map[string]string{}
	codes := map[string]int{}
	packages := map[string]domain.Package{}
	for _, id := range names {
		rel := "payloads/" + id + "/" + id + ".exe"
		dir := filepath.Join(kit, "payloads", id)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data := []byte("test payload " + id)
		if err := os.WriteFile(filepath.Join(dir, id+".exe"), data, 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		marker := filepath.Join(t.TempDir(), id+".installed")
		markers[id+".exe"] = marker
		codes[id+".exe"] = 0
		p := domain.Package{
			ID: id, Name: id, Version: "1.0", Schema: 1,
			Platform: domain.Platform{OS: "windows", Arch: []string{"x64"}},
			Payload:  domain.Payload{Kind: "file", Path: rel, SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data))},
			Install:  domain.Install{Driver: "exe", TimeoutSec: 3, SuccessCodes: []int{0}, RebootCodes: []int{3010}},
			Detect:   domain.Detect{Match: "all", Rules: []domain.DetectRule{{Type: "file", Path: marker}}},
		}
		if id == "app" {
			p.DependsOn = []string{"base"}
		}
		if id == "base" && reboot {
			p.Install.RebootBarrier = true
			codes[id+".exe"] = 3010
		}
		packages[id] = p
	}
	proc := &groupProc{markers: markers, codes: codes, started: []string{}}
	e := &InstallEngine{
		Catalog: domain.Catalog{Index: domain.Index{CatalogVersion: "2026.10.1"}, Packages: packages, Profiles: map[string]domain.Profile{
			"full": {ID: "full", Name: "Full", Packages: []string{"app", "unrelated"}},
		}},
		KitRoot: kit, StateRoot: t.TempDir(), Registry: testRegistry{}, Paths: testPaths{},
		Process: proc, IsAdmin: func() (bool, error) { return true, nil },
		WindowsBuild:        func() (int, error) { return 26100, nil },
		labPlatformOverride: true,
	}
	return e, proc, planner.Selection{Profile: "full"}
}
func TestGroupExecutesDependenciesInOrder(t *testing.T) {
	e, proc, sel := groupFixture(t, false)
	r, err := e.RunBatch(context.Background(), sel, BatchOptions{InstallOptions: labOptions()}, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode != 0 || len(r.Results) != 3 {
		t.Fatalf("bad report %+v", r)
	}
	var posBase, posApp = -1, -1
	for i, n := range proc.started {
		if n == "base.exe" {
			posBase = i
		}
		if n == "app.exe" {
			posApp = i
		}
	}
	if posBase < 0 || posApp < 0 || posBase >= posApp {
		t.Fatalf("dependency order broken: %v", proc.started)
	}
}
func TestFailingDependencyBlocksDependentButNotUnrelated(t *testing.T) {
	e, proc, sel := groupFixture(t, false)
	proc.codes["base.exe"] = 17
	r, err := e.RunBatch(context.Background(), sel, BatchOptions{InstallOptions: labOptions()}, "")
	if err != nil {
		t.Fatal(err)
	}
	if r.ExitCode != 1 {
		t.Fatalf("expected partial failure: %+v", r)
	}
	var baseFailed, appBlocked, unrelatedInstalled bool
	for _, v := range r.Results {
		if v.PackageID == "base" {
			baseFailed = v.Outcome == domain.OutcomeFailed
		}
		if v.PackageID == "app" {
			appBlocked = v.Outcome == domain.OutcomeSkipped && strings.Contains(v.Detail, "dependência")
		}
		if v.PackageID == "unrelated" {
			unrelatedInstalled = v.Outcome == domain.OutcomeInstalled
		}
	}
	if !baseFailed || !appBlocked || !unrelatedInstalled {
		t.Fatalf("bad dependency failure handling: %+v", r)
	}
}
func TestRebootResumeReplansWithoutReinstalling(t *testing.T) {
	e, proc, sel := groupFixture(t, true)
	opt := BatchOptions{InstallOptions: labOptions()}
	first, err := e.RunBatch(context.Background(), sel, opt, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ExitCode != domain.ExitReboot {
		t.Fatalf("expected reboot exit10: %+v", first)
	}
	if len(proc.started) != 1 || proc.started[0] != "base.exe" {
		t.Fatalf("post-reboot-dependent started too early: %v", proc.started)
	}
	second, err := e.RunBatch(context.Background(), planner.Selection{}, opt, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if second.RunID != first.RunID || second.ExitCode != 0 {
		t.Fatalf("resume failed %+v", second)
	}
	if len(proc.started) != 3 {
		t.Fatalf("incorrect number of native process starts: %v", proc.started)
	}
	for _, name := range proc.started[1:] {
		if name == "base.exe" {
			t.Fatalf("resumed reinstall of already installed barrier: %v", proc.started)
		}
	}
	if _, err := e.RunBatch(context.Background(), planner.Selection{}, opt, first.RunID); err == nil {
		t.Fatal("completed run accepted another resume")
	}
}
func TestResumeRefusesDifferentKitAndCatalog(t *testing.T) {
	e, _, sel := groupFixture(t, true)
	opt := BatchOptions{InstallOptions: labOptions()}
	one, err := e.RunBatch(context.Background(), sel, opt, "")
	if err != nil {
		t.Fatal(err)
	}
	other := *e
	other.KitRoot = t.TempDir()
	if _, err := other.RunBatch(context.Background(), planner.Selection{}, opt, one.RunID); err == nil {
		t.Fatal("resumed against different kit")
	}
	other = *e
	other.Catalog.Index.CatalogVersion = "2027.01.1"
	if _, err := other.RunBatch(context.Background(), planner.Selection{}, opt, one.RunID); err == nil {
		t.Fatal("resumed with different catalogVersion")
	}
}
func TestAllHashesCheckedBeforeAnyInstallerStarts(t *testing.T) {
	e, proc, sel := groupFixture(t, false)
	changed := filepath.Join(e.KitRoot, "payloads", "app", "app.exe")
	if err := os.WriteFile(changed, []byte("tampered payload"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := e.RunBatch(context.Background(), sel, BatchOptions{InstallOptions: labOptions()}, "")
	if err == nil {
		t.Fatal("tampered payload accepted")
	}
	if len(proc.started) != 0 {
		t.Fatalf("first installer ran before verifying later payload: %v", proc.started)
	}
}

func TestBatchStopOnErrorLeavesUnrelatedUntouched(t *testing.T) {
	e, proc, selection := groupFixture(t, false)
	proc.codes["base.exe"] = 17
	report, err := e.RunBatch(context.Background(), selection, BatchOptions{
		InstallOptions: labOptions(),
		StopOnError:    true,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode != domain.ExitPartial || len(proc.started) != 1 || proc.started[0] != "base.exe" {
		t.Fatalf("stop-on-error ignored: report=%+v, processes=%v", report, proc.started)
	}
}

func TestResumeAfterForceDoesNotRepeatAlreadyInstalledBarrier(t *testing.T) {
	e, proc, selection := groupFixture(t, true)
	options := BatchOptions{InstallOptions: labOptions(), Force: true}
	first, err := e.RunBatch(context.Background(), selection, options, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ExitCode != domain.ExitReboot {
		t.Fatalf("expected reboot barrier: %+v", first)
	}
	second, err := e.RunBatch(context.Background(), planner.Selection{}, options, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ExitCode != domain.ExitOK {
		t.Fatalf("resume failed: %+v", second)
	}
	baseStarts := 0
	for _, name := range proc.started {
		if name == "base.exe" {
			baseStarts++
		}
	}
	if baseStarts != 1 {
		t.Fatalf("force flag was reused during resume: %v", proc.started)
	}
}
