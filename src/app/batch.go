package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/detect"
	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/journal"
	"github.com/DragonBlack18/DeployCraft/src/planner"
)

// BatchOptions permits multiple packages, but unsigned kits remain LAB-only.
type BatchOptions struct {
	InstallOptions
	Force       bool
	StopOnError bool
}

// PlanSelection detects once per catalog and delegates all ordering to the pure planner.
func (e *InstallEngine) PlanSelection(sel planner.Selection, opt planner.Options) (domain.Plan, error) {
	if e.Registry == nil || e.Paths == nil {
		return domain.Plan{}, fmt.Errorf("E-PRE-002: detector indisponível")
	}
	scan := detect.NewScan(e.Registry, e.Paths)
	states := map[string]domain.Detection{}
	for id, p := range e.Catalog.Packages {
		states[id] = scan.Detect(p)
	}
	return planner.Build(e.Catalog, sel, states, opt)
}

// RunBatch records one runID across packages and process restarts. Resume re-plans
// using current detections; it never trusts a saved step cursor.
func (e *InstallEngine) RunBatch(ctx context.Context, sel planner.Selection, opt BatchOptions, resumeID string) (PackageReport, error) {
	report := PackageReport{Results: []domain.Result{}}
	if opt.DryRun {
		return report, fmt.Errorf("E-PLN-003: use plan for dry-run")
	}
	if !opt.Confirmed {
		return report, &PreflightError{Code: "E-PLN-003", Reason: "--yes obrigatório em execução LAB"}
	}
	if !opt.LabUnsignedCatalog {
		return report, &PreflightError{Code: "E-VER-004", Reason: "kit sem catálogo assinado: somente VM LAB com --lab-unsigned-catalog"}
	}
	if e.Process == nil || e.Registry == nil || e.Paths == nil {
		return report, &PreflightError{Code: "E-PRE-002", Reason: "adaptadores incompletos"}
	}
	if e.KitRoot == "" || e.StateRoot == "" {
		return report, &PreflightError{Code: "E-PRE-003", Reason: "caminho do kit ou estado ausente"}
	}
	store := journal.New(e.StateRoot)
	previously := map[string]journal.Event{}
	resumed := resumeID != ""
	if resumed {
		history, err := store.ReadRun(resumeID)
		if err != nil {
			return report, err
		}
		if history.LastExitCode != nil && *history.LastExitCode == domain.ExitOK {
			return report, fmt.Errorf("E-RES-003: execução %s já foi concluída", resumeID)
		}
		if filepath.Clean(history.Start.KitRoot) != filepath.Clean(e.KitRoot) {
			return report, &PreflightError{Code: "E-RES-002", Reason: "o kit original não está no mesmo caminho"}
		}
		if history.Start.Catalog != e.Catalog.Index.CatalogVersion {
			return report, &PreflightError{Code: "E-RES-003", Reason: "versão do catálogo diferente da execução original"}
		}
		sel = planner.Selection{Profile: history.Start.Selection.Profile, IDs: append([]string(nil), history.Start.Selection.IDs...)}
		opt.StopOnError = history.Start.Options.StopOnError
		opt.Force = false // Never force previously completed packages during resume.
		previously = history.Results
		report.RunID = resumeID
	}
	plan, err := e.PlanSelection(sel, planner.Options{Force: opt.Force, UpgradeOutdated: true})
	if err != nil {
		return report, err
	}
	// Preflight ALL pending packages (including integrity hashes) before execution.
	selected := domain.Catalog{Packages: map[string]domain.Package{}}
	for _, phase := range plan.Phases {
		for _, step := range phase.Steps {
			if step.Action == domain.ActionSkip {
				continue
			}
			p := e.Catalog.Packages[step.PackageID]
			if err := e.preflight(p, opt.InstallOptions, true); err != nil {
				return report, err
			}
			if p.Install.RawCmdLine != "" {
				return report, &PreflightError{Code: "E-DRV-002", Reason: "rawCmdLine ainda não implementado"}
			}
			for _, arg := range p.Install.Args {
				if len(arg) > 0 && arg[0] == '{' {
					return report, &PreflightError{Code: "E-PLN-003", Reason: "secrets serão implementados na Fase 4"}
				}
			}
			selected.Packages[p.ID] = p
		}
	}
	if issues := catalog.Verify(e.KitRoot, selected); len(issues) > 0 {
		first := issues[0]
		return report, &PreflightError{Code: first.Code, Reason: first.PackageID + ": " + first.Problem}
	}
	if !resumed {
		report.RunID, err = journal.NewRunID()
		if err != nil {
			return report, err
		}
	}
	start := journal.Event{
		RunID: report.RunID, Event: "run.started", Resumed: resumed,
		KitRoot: e.KitRoot, Catalog: e.Catalog.Index.CatalogVersion,
		Selection: &journal.StoredSelection{Profile: sel.Profile, IDs: append([]string(nil), sel.IDs...)},
		Options:   &journal.StoredOptions{Force: opt.Force, StopOnError: opt.StopOnError},
	}
	if err := store.Append(start); err != nil {
		return report, err
	}
	if err := store.Append(journal.Event{RunID: report.RunID, Event: "plan.ready", Detail: fmt.Sprintf("%d fases", len(plan.Phases))}); err != nil {
		return report, err
	}
	results := map[string]domain.Result{}
	blocked := map[string]bool{}
	for id, ev := range previously {
		results[id] = domain.Result{PackageID: id, Outcome: domain.Outcome(ev.Outcome), Code: ev.Code,
			Reason: domain.FailReason(ev.Reason), ExitCode: ev.ExitCode, DurationMs: ev.DurationMs, Detail: ev.Detail}
	}
	hasFailed := false
	needsBarrierResume := false
	for index, phase := range plan.Phases {
		for _, step := range phase.Steps {
			id := step.PackageID
			p := e.Catalog.Packages[id]
			result := domain.Result{PackageID: id}
			reason := ""
			for _, dep := range p.DependsOn {
				if blocked[dep] || results[dep].Outcome == domain.OutcomeFailed {
					reason = "dependência " + dep + " falhou"
					break
				}
			}
			switch {
			case reason != "":
				result.Outcome = domain.OutcomeSkipped
				result.Detail = reason
				blocked[id] = true
			case step.Action == domain.ActionSkip:
				if prior, ok := results[id]; ok && (prior.Outcome == domain.OutcomeInstalled || prior.Outcome == domain.OutcomeInstalledReboot) {
					result = prior
					if resumed && prior.Outcome == domain.OutcomeInstalledReboot {
						result.Outcome = domain.OutcomeInstalled
					}
				} else {
					result.Outcome = domain.OutcomeSkipped
					result.Detail = step.Reason
				}
			default:
				latest := detect.NewScan(e.Registry, e.Paths).Detect(p)
				if latest.State == domain.StateUnknown {
					result = domain.Result{PackageID: id, Outcome: domain.OutcomeFailed, Code: "E-PRE-006", Reason: domain.ReasonNotDetected, Detail: "detecção mudou para desconhecida"}
				} else if latest.State == domain.StateInstalled && !opt.Force {
					result.Outcome = domain.OutcomeSkipped
					result.Detail = "já instalado na confirmação imediatamente anterior"
				} else {
					result = e.execute(ctx, store, report.RunID, p)
				}
			}
			results[id] = result
			if result.Outcome == domain.OutcomeFailed {
				hasFailed = true
				blocked[id] = true
			}
			if err := store.Append(journal.Event{RunID: report.RunID, Event: "step.finished", Package: id,
				Outcome: string(result.Outcome), Reason: string(result.Reason), Code: result.Code,
				DurationMs: result.DurationMs, ExitCode: result.ExitCode, Detail: result.Detail}); err != nil {
				return report, err
			}
			if err := store.WriteSnapshot(journal.Snapshot{RunID: report.RunID, Status: "running", Packages: snapshotStates(results)}); err != nil {
				return report, err
			}
			if result.Outcome == domain.OutcomeFailed && opt.StopOnError {
				break
			}
		}
		if hasFailed && opt.StopOnError {
			break
		}
		if phase.RebootAfter && index < len(plan.Phases)-1 {
			last := phase.Steps[len(phase.Steps)-1]
			if results[last.PackageID].Outcome == domain.OutcomeInstalledReboot {
				needsBarrierResume = true
				if err := store.Append(journal.Event{RunID: report.RunID, Event: "reboot.required", Package: last.PackageID}); err != nil {
					return report, err
				}
				break
			}
		}
	}
	// Preserve deterministic plan order; map iteration must never affect reports.
	added := map[string]bool{}
	for _, phase := range plan.Phases {
		for _, step := range phase.Steps {
			if result, ok := results[step.PackageID]; ok && !added[step.PackageID] {
				report.Results = append(report.Results, result)
				added[step.PackageID] = true
			}
		}
	}
	report.ExitCode = domain.ExitCodeFor(report.Results)
	if needsBarrierResume {
		report.ExitCode = domain.ExitReboot
	}
	status := "done"
	if needsBarrierResume {
		status = "reboot"
	}
	if err := store.WriteSnapshot(journal.Snapshot{RunID: report.RunID, Status: status, Packages: snapshotStates(results)}); err != nil {
		return report, err
	}
	if err := store.Append(journal.Event{RunID: report.RunID, Event: "run.finished", ExitCode: &report.ExitCode}); err != nil {
		return report, err
	}
	if err := store.WriteReport(report.RunID, report); err != nil {
		return report, err
	}
	return report, nil
}
func snapshotStates(results map[string]domain.Result) map[string]string {
	out := make(map[string]string, len(results))
	for id, r := range results {
		out[id] = string(r.Outcome)
	}
	return out
}
