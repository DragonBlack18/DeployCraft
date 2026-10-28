// Package app defines use cases that UI adapters may call in future phases.
// No installation implementation is claimed at this stage.
package app

import (
	"context"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type Selection struct {
	Profile string
	IDs     []string
}
type Options struct {
	DryRun      bool
	Force       bool
	StopOnError bool
	Portable    bool
}
type PackageStatus struct {
	ID         string
	Name       string
	KitVersion string
	Detection  domain.Detection
}
type VerifyIssue struct {
	PackageID string
	Path      string
	Code      string
	Problem   string
}
type EventKind string

const (
	EventPlanReady    EventKind = "PlanoPronto"
	EventStepStarted  EventKind = "PassoIniciado"
	EventStepProgress EventKind = "PassoProgresso"
	EventStepDone     EventKind = "PassoConcluido"
	EventRebootNeeded EventKind = "ReinicioNecessario"
	EventRunDone      EventKind = "ExecucaoConcluida"
)
type Event struct {
	Kind      EventKind
	RunID     string
	PackageID string
	Action    domain.Action
	Reason    string
	Elapsed   time.Duration
	Tail      string
	Result    *domain.Result
}
type Report struct {
	RunID    string
	Results  []domain.Result
	ExitCode int
}

// Service is the only API a future desktop interface will import.
type Service interface {
	List(ctx context.Context) ([]PackageStatus, error)
	Plan(ctx context.Context, sel Selection, opts Options) (domain.Plan, error)
	Execute(ctx context.Context, plan domain.Plan, opts Options, emit func(Event)) (Report, error)
	Uninstall(ctx context.Context, ids []string, emit func(Event)) (Report, error)
	Verify(ctx context.Context) ([]VerifyIssue, error)
}
