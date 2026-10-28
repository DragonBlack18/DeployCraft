package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/detect"
	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/drivers"
	"github.com/DragonBlack18/DeployCraft/src/journal"
	"github.com/DragonBlack18/DeployCraft/src/verify"
)

type InstallEngine struct {
	Catalog             domain.Catalog
	KitRoot             string
	StateRoot           string
	Registry            domain.Registry
	Paths               domain.Paths
	Process             domain.Process
	IsAdmin             func() (bool, error)
	WindowsBuild        func() (int, error)
	postCheckTimeout    time.Duration
	labPlatformOverride bool // used only by package-internal tests
}
type PreflightError struct {
	Code   string
	Reason string
}

func (e *PreflightError) Error() string { return e.Code + ": " + e.Reason }

type InstallOptions struct {
	LabUnsignedCatalog bool
	Confirmed          bool
	DryRun             bool
}
type PackageReport struct {
	RunID    string          `json:"runId"`
	Results  []domain.Result `json:"results"`
	ExitCode int             `json:"exitCode"`
}

func (e *InstallEngine) Status(id string) (domain.Detection, error) {
	p, ok := e.Catalog.Packages[id]
	if !ok {
		return domain.Detection{}, fmt.Errorf("E-PLN-001: pacote desconhecido: %s", id)
	}
	return detect.NewScan(e.Registry, e.Paths).Detect(p), nil
}
func (e *InstallEngine) preflight(p domain.Package, opt InstallOptions) error {
	if runtime.GOOS != "windows" && !e.labPlatformOverride {
		return &PreflightError{Code: "E-PRE-002", Reason: "instalação real disponível apenas em Windows"}
	}
	if !opt.LabUnsignedCatalog {
		return &PreflightError{Code: "E-VER-004", Reason: "catálogo ainda não assinado; execução permitida apenas em VM de laboratório com --lab-unsigned-catalog"}
	}
	if !opt.Confirmed && !opt.DryRun {
		return &PreflightError{Code: "E-PLN-003", Reason: "confirmação explícita --yes necessária"}
	}
	if e.IsAdmin == nil {
		return &PreflightError{Code: "E-PRE-001", Reason: "adaptador de UAC ausente"}
	}
	admin, err := e.IsAdmin()
	if err != nil {
		return &PreflightError{Code: "E-PRE-001", Reason: err.Error()}
	}
	if !admin {
		return &PreflightError{Code: "E-PRE-001", Reason: "o token do processo não está elevado"}
	}
	if !e.labPlatformOverride && (runtime.GOARCH != "amd64" || p.Platform.OS != "windows") {
		return &PreflightError{Code: "E-PRE-002", Reason: "Windows x64 obrigatório"}
	}
	if p.Platform.MinBuild > 0 {
		if e.WindowsBuild == nil {
			return &PreflightError{Code: "E-PRE-002", Reason: "sem detector da build do Windows"}
		}
		build, err := e.WindowsBuild()
		if err != nil {
			return &PreflightError{Code: "E-PRE-002", Reason: err.Error()}
		}
		if build < p.Platform.MinBuild {
			return &PreflightError{Code: "E-PRE-002", Reason: fmt.Sprintf("build %d menor que %d", build, p.Platform.MinBuild)}
		}
	}
	if len(p.DependsOn) > 0 || len(p.ConflictsWith) > 0 {
		return &PreflightError{Code: "E-PLN-002", Reason: "dependências/conflitos exigem planner (Fase 2)"}
	}
	if p.Install.Driver != "exe" && p.Install.Driver != "msi" {
		return &PreflightError{Code: "E-DRV-002", Reason: "driver ainda não implementado na Fase 1"}
	}
	return nil
}

// InstallOne executes at most one package. It must be invoked only after
// explicit human approval in an isolated Windows test VM until signed kits ship.
// It never downloads payloads and never modifies the source kit.
func (e *InstallEngine) InstallOne(ctx context.Context, id string, opt InstallOptions) (PackageReport, error) {
	report := PackageReport{}
	p, ok := e.Catalog.Packages[id]
	if !ok {
		return report, &PreflightError{Code: "E-PLN-001", Reason: "pacote não encontrado: " + id}
	}
	if err := e.preflight(p, opt); err != nil {
		return report, err
	}
	if e.Registry == nil || e.Paths == nil || e.Process == nil {
		return report, fmt.Errorf("E-PRE-002: adaptadores do motor incompletos")
	}
	before := detect.NewScan(e.Registry, e.Paths).Detect(p)
	if before.State == domain.StateUnknown {
		return report, &PreflightError{Code: "E-PRE-006", Reason: "detecção indeterminada; instalação bloqueada: " + strings.Join(before.Evidence, "; ")}
	}
	idRun, err := journal.NewRunID()
	if err != nil {
		return report, err
	}
	report.RunID = idRun
	store := journal.New(e.StateRoot)
	if !opt.DryRun {
		if err := store.Append(journal.Event{RunID: idRun, Event: "run.started"}); err != nil {
			return report, err
		}
		if err := store.Append(journal.Event{RunID: idRun, Event: "plan.ready", Package: id, Action: "install"}); err != nil {
			return report, err
		}
	}
	result := domain.Result{PackageID: id}
	var elapsed time.Duration
	start := time.Now()
	if before.State == domain.StateInstalled {
		result.Outcome = domain.OutcomeSkipped
		result.Detail = "versão igual ou superior já instalada"
	} else if opt.DryRun {
		result.Outcome = domain.OutcomeSkipped
		result.Detail = "dry-run: planejado, sem executar"
	} else {
		result = e.execute(ctx, store, idRun, p)
	}
	elapsed = time.Since(start)
	if result.DurationMs == 0 {
		result.DurationMs = elapsed.Milliseconds()
	}
	report.Results = []domain.Result{result}
	report.ExitCode = domain.ExitCodeFor(report.Results)
	if opt.DryRun {
		return report, nil
	}
	if err := store.Append(journal.Event{RunID: idRun, Event: "step.finished", Package: id, Outcome: string(result.Outcome), Reason: string(result.Reason), Code: result.Code, DurationMs: result.DurationMs, ExitCode: result.ExitCode}); err != nil {
		return report, err
	}
	status := "done"
	if result.Outcome == domain.OutcomeInstalledReboot {
		status = "reboot"
	}
	if err := store.WriteSnapshot(journal.Snapshot{RunID: idRun, Status: status, Packages: map[string]string{id: string(result.Outcome)}}); err != nil {
		return report, err
	}
	if err := store.Append(journal.Event{RunID: idRun, Event: "run.finished", ExitCode: &report.ExitCode}); err != nil {
		return report, err
	}
	if err := store.WriteReport(idRun, report); err != nil {
		return report, err
	}
	return report, nil
}
func (e *InstallEngine) execute(ctx context.Context, store *journal.Store, runID string, p domain.Package) (result domain.Result) {
	result = domain.Result{PackageID: p.ID}
	fail := func(code string, reason domain.FailReason, detail string) domain.Result {
		result.Outcome = domain.OutcomeFailed
		result.Code = code
		result.Reason = reason
		result.Detail = detail
		return result
	}
	start := time.Now()
	defer func() { result.DurationMs = time.Since(start).Milliseconds() }()
	if err := store.Append(journal.Event{RunID: runID, Event: "step.started", Package: p.ID, Version: p.Version, Action: "install"}); err != nil {
		return fail("E-RUN-002", domain.ReasonExitCode, "journal indisponível: "+err.Error())
	}
	stagingRoot := filepath.Join(e.StateRoot, "staging")
	if err := os.MkdirAll(stagingRoot, 0700); err != nil {
		return fail("E-PRE-003", domain.ReasonExitCode, err.Error())
	}
	runStage, err := os.MkdirTemp(stagingRoot, "run-")
	if err != nil {
		return fail("E-PRE-003", domain.ReasonExitCode, err.Error())
	}
	defer os.RemoveAll(runStage) // this uniquely created path is owned by this execution
	file, err := verify.Stage(e.KitRoot, runStage, p)
	if err != nil {
		var integrity *verify.IntegrityError
		if errors.As(err, &integrity) {
			return fail(integrity.Code, domain.ReasonIntegrity, integrity.Error())
		}
		return fail("E-VER-003", domain.ReasonIntegrity, err.Error())
	}
	logDir := filepath.Join(e.StateRoot, "logs", runID)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fail("E-PRE-003", domain.ReasonExitCode, err.Error())
	}
	logPath := filepath.Join(logDir, p.ID+".log")
	d, err := drivers.New(p.Install.Driver)
	if err != nil {
		return fail("E-DRV-002", domain.ReasonExitCode, err.Error())
	}
	cmd, err := d.Prepare(p, file, domain.RunContext{RunID: runID, KitRoot: e.KitRoot, Staging: runStage, LogPath: logPath})
	if err != nil {
		return fail("E-DRV-002", domain.ReasonExitCode, err.Error())
	}
	// Only the masked LogLine is ever recorded. No argv or environment in logs.
	if err := os.WriteFile(filepath.Join(logDir, p.ID+".command.txt"), []byte(cmd.LogLine+"\n"), 0600); err != nil {
		return fail("E-PRE-003", domain.ReasonExitCode, err.Error())
	}
	process, err := e.Process.Start(cmd)
	if err != nil {
		return fail("E-RUN-002", domain.ReasonExitCode, err.Error())
	}
	timeout := time.Duration(p.Install.TimeoutSec) * time.Second
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	exitCode, err := process.Wait(stepCtx)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fail("E-RUN-001", domain.ReasonTimeout, "tempo limite da instalação esgotado ou cancelado")
	}
	if err != nil {
		return fail("E-RUN-003", domain.ReasonExitCode, err.Error())
	}
	result.ExitCode = &exitCode
	verdict, _ := d.Interpret(exitCode, p)
	switch verdict {
	case domain.VerdictSuccess, domain.VerdictSuccessReboot:
	case domain.VerdictRetry:
		return fail("E-RUN-003", domain.ReasonExitCode, "repetição automática de código transitório será implementada na Fase 4")
	default:
		return fail("E-RUN-003", domain.ReasonExitCode, fmt.Sprintf("instalador retornou código %d", exitCode))
	}
	wait := e.postCheckTimeout
	if wait <= 0 {
		wait = 60 * time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		// Recreate registry index after installer completion: never use the old snapshot.
		state := detect.NewScan(e.Registry, e.Paths).Detect(p)
		if state.State == domain.StateInstalled {
			result.Outcome = domain.OutcomeInstalled
			if verdict == domain.VerdictSuccessReboot {
				result.Outcome = domain.OutcomeInstalledReboot
			}
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fail("E-RUN-004", domain.ReasonNotDetected, "código de sucesso, mas software não confirmado após instalação")
		}
		select {
		case <-ctx.Done():
			return fail("E-RUN-004", domain.ReasonNotDetected, "cancelado durante pós-detecção")
		case <-time.After(500 * time.Millisecond):
		}
	}
}
