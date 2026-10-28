// Package domain defines DeployCraft's stable, platform-independent contracts.
// It must not access the filesystem, registry, network, or process APIs.
package domain

import "encoding/json"

type Platform struct {
	OS       string   `json:"os"`
	Arch     []string `json:"arch"`
	MinBuild int      `json:"minBuild,omitempty"`
}
type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Payload struct {
	Kind   string      `json:"kind"`
	Path   string      `json:"path"`
	SHA256 string      `json:"sha256,omitempty"`
	Size   int64       `json:"size,omitempty"`
	Entry  string      `json:"entry,omitempty"`
	Files  []FileEntry `json:"files,omitempty"`
}
type Shortcut struct {
	Name     string `json:"name"`
	Target   string `json:"target"`
	Location string `json:"location,omitempty"`
}
type UninstallEntry struct {
	DisplayName string `json:"displayName"`
}
type Install struct {
	Driver         string            `json:"driver"`
	Args           []json.RawMessage `json:"args,omitempty"`
	RawCmdLine     string            `json:"rawCmdLine,omitempty"`
	Dir            string            `json:"dir,omitempty"`
	TimeoutSec     int               `json:"timeoutSec"`
	SuccessCodes   []int             `json:"successCodes"`
	RebootCodes    []int             `json:"rebootCodes,omitempty"`
	RetryCodes     []int             `json:"retryCodes,omitempty"`
	RebootBarrier  bool              `json:"rebootBarrier,omitempty"`
	Target         string            `json:"target,omitempty"`
	AddToPath      bool              `json:"addToPath,omitempty"`
	Shortcuts      []Shortcut        `json:"shortcuts,omitempty"`
	UninstallEntry *UninstallEntry   `json:"uninstallEntry,omitempty"`
}
type DetectRule struct {
	Type        string          `json:"type"`
	DisplayName string          `json:"displayName,omitempty"`
	ProductCode string          `json:"productCode,omitempty"`
	Path        string          `json:"path,omitempty"`
	MinVersion  string          `json:"minVersion,omitempty"`
	Key         string          `json:"key,omitempty"`
	Value       string          `json:"value,omitempty"`
	Equals      json.RawMessage `json:"equals,omitempty"`
	AtLeast     *int64          `json:"atLeast,omitempty"`
	Command     string          `json:"command,omitempty"`
	Args        []string        `json:"args,omitempty"`
	Matches     string          `json:"matches,omitempty"`
	TimeoutSec  int             `json:"timeoutSec,omitempty"`
	Name        string          `json:"name,omitempty"`
}
type Detect struct {
	Match string       `json:"match"`
	Rules []DetectRule `json:"rules"`
}
type Uninstall struct {
	Driver  string            `json:"driver"`
	Command string            `json:"command,omitempty"`
	Args    []json.RawMessage `json:"args,omitempty"`
}
type Provenance struct {
	Source     string `json:"source,omitempty"`
	VendorHash string `json:"vendorHash,omitempty"`
	AddedOn    string `json:"addedOn,omitempty"`
}
type Package struct {
	Schema        int         `json:"schema"`
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Version       string      `json:"version"`
	Publisher     string      `json:"publisher,omitempty"`
	Platform      Platform    `json:"platform"`
	Payload       Payload     `json:"payload"`
	Install       Install     `json:"install"`
	Detect        Detect      `json:"detect"`
	Uninstall     *Uninstall  `json:"uninstall,omitempty"`
	DependsOn     []string    `json:"dependsOn,omitempty"`
	ConflictsWith []string    `json:"conflictsWith,omitempty"`
	Provenance    *Provenance `json:"provenance,omitempty"`
}
type Profile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Extends  string   `json:"extends,omitempty"`
	Packages []string `json:"packages"`
	Atomic   bool     `json:"atomic,omitempty"`
}
type Index struct {
	Schema         int       `json:"schema"`
	CatalogVersion string    `json:"catalogVersion"`
	Packages       []string  `json:"packages"`
	Profiles       []Profile `json:"profiles"`
}
type Catalog struct {
	Index    Index
	Packages map[string]Package
	Profiles map[string]Profile
}

// Future execution contracts; these models are data, not OS actions.
type State string

const (
	StateAbsent    State = "ausente"
	StateInstalled State = "instalado"
	StateOutdated  State = "desatualizado"
	StateUnknown   State = "desconhecido"
)

type Action string

const (
	ActionInstall Action = "instalar"
	ActionUpdate  Action = "atualizar"
	ActionSkip    Action = "pular"
)

type Step struct {
	PackageID string `json:"packageId"`
	Action    Action `json:"action"`
	Reason    string `json:"reason"`
}
type Phase struct {
	Steps       []Step `json:"steps"`
	RebootAfter bool   `json:"rebootAfter"`
}
type Plan struct {
	Phases []Phase `json:"phases"`
}
type Outcome string

const (
	OutcomeSkipped         Outcome = "skipped"
	OutcomeInstalled       Outcome = "installed"
	OutcomeInstalledReboot Outcome = "installed_reboot"
	OutcomeFailed          Outcome = "failed"
)

type FailReason string

const (
	ReasonIntegrity   FailReason = "integrity"
	ReasonTimeout     FailReason = "timeout"
	ReasonExitCode    FailReason = "exit_code"
	ReasonNotDetected FailReason = "not_detected"
)

type Result struct {
	PackageID  string     `json:"package"`
	Outcome    Outcome    `json:"outcome"`
	Reason     FailReason `json:"reason,omitempty"`
	Code       string     `json:"code,omitempty"`
	ExitCode   *int       `json:"exitCode,omitempty"`
	DurationMs int64      `json:"durationMs"`
	Detail     string     `json:"detail,omitempty"`
}

const (
	ExitOK        = 0
	ExitPartial   = 1
	ExitUsage     = 2
	ExitCatalog   = 3
	ExitPrecheck  = 4
	ExitIntegrity = 5
	ExitReboot    = 10
)

func ExitCodeFor(results []Result) int {
	failed, reboot := false, false
	for _, r := range results {
		if r.Outcome == OutcomeFailed && r.Reason == ReasonIntegrity {
			return ExitIntegrity
		}
		if r.Outcome == OutcomeFailed {
			failed = true
		}
		if r.Outcome == OutcomeInstalledReboot {
			reboot = true
		}
	}
	if failed {
		return ExitPartial
	}
	if reboot {
		return ExitReboot
	}
	return ExitOK
}
