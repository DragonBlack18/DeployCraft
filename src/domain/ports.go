package domain

import (
	"context"
	"io"
)

// Detection records evidence without converting failed probes into "absent".
type Detection struct {
	State    State
	Version  string
	Evidence []string
}

// Command carries actual arguments separately from masked logging text.
type Command struct {
	Exe        string
	Args       []string
	Cwd        string
	Env        []string
	RawCmdLine string
	LogLine    string
}

type RunContext struct {
	RunID   string
	KitRoot string
	Staging string
	LogPath string
	Secrets map[string]string
	DryRun  bool
}

type Verdict int

const (
	VerdictSuccess Verdict = iota
	VerdictSuccessReboot
	VerdictRetry
	VerdictFailure
)

type Driver interface {
	Prepare(pkg Package, file string, ctx RunContext) (Command, error)
	Interpret(exitCode int, pkg Package) (Verdict, FailReason)
}

type Registry interface {
	ReadString(key, value string) (text string, ok bool, err error)
	ListSubKeys(key string) ([]string, error)
}

type Paths interface {
	ProgramData() string
	Temp() string
	Expand(text string) string
}

type Elevation interface { IsAdmin() (bool, error) }
type Disk interface { FreeBytes(path string) (uint64, error) }
type Reboot interface { Pending() (bool, error) }

// Implementations must enforce allowed paths and atomic-write semantics.
type FileSystem interface {
	Open(path string) (io.ReadCloser, error)
	ReadFile(path string) ([]byte, error)
	WriteFileAtomic(path string, data []byte) error
	MkdirAll(path string) error
	RemoveAll(path string) error
}

type Running interface {
	Wait(ctx context.Context) (exitCode int, err error)
	Kill() error
}
type Process interface { Start(command Command) (Running, error) }
type PayloadSource interface { Open(relPath string) (io.ReadCloser, error) }
