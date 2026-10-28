package journal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"reflect"
)

// StoredSelection/Options record what the operator approved. No secret is persisted.
type StoredSelection struct {
	Profile string   `json:"profile,omitempty"`
	IDs     []string `json:"ids,omitempty"`
}
type StoredOptions struct {
	Force       bool `json:"force"`
	StopOnError bool `json:"stopOnError"`
}

// RunHistory is recovered from append-only events rather than from a cursor.
type RunHistory struct {
	Start        Event
	Results      map[string]Event
	LastExitCode *int
}

var runIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ReadRun ignores only a truncated final line. Any corrupt complete line
// or missing start metadata fails closed, even when another run owns the line.
func (s *Store) ReadRun(runID string) (RunHistory, error) {
	h := RunHistory{Results: map[string]Event{}}
	if !runIDPattern.MatchString(runID) {
		return h, fmt.Errorf("E-RES-001: runId inválido")
	}
	f, err := os.Open(filepath.Join(s.Root, "state", "journal.jsonl"))
	if err != nil {
		return h, fmt.Errorf("E-RES-001: journal indisponível: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return h, err
	}
	if info.Size() > 50<<20 {
		return h, fmt.Errorf("E-RES-003: journal acima do limite")
	}
	reader := bufio.NewReader(f)
	seenStart := false
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) == 0 && readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != io.EOF {
			return h, readErr
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			if readErr == io.EOF {
				break
			}
			return h, fmt.Errorf("E-RES-003: journal inválido: %w", err)
		}
		if ev.RunID != runID {
			if readErr == io.EOF {
				break
			}
			continue
		}
		switch ev.Event {
		case "run.started":
			if ev.Selection == nil || ev.Options == nil || ev.Catalog == "" ||
				ev.KitRoot == "" || !digestPattern.MatchString(ev.CatalogDigest) {
				return h, fmt.Errorf("E-RES-003: run.started incompleto ou sem catalogDigest")
			}
			if !seenStart {
				if ev.Resumed {
					return h, fmt.Errorf("E-RES-003: resume sem execução original")
				}
				h.Start = ev
				seenStart = true
			} else if !ev.Resumed || (h.LastExitCode != nil && *h.LastExitCode == 0) ||
				ev.KitRoot != h.Start.KitRoot || ev.Catalog != h.Start.Catalog ||
				ev.CatalogDigest != h.Start.CatalogDigest ||
				!reflect.DeepEqual(ev.Selection, h.Start.Selection) ||
				ev.Options.StopOnError != h.Start.Options.StopOnError {
				return h, fmt.Errorf("E-RES-003: metadados divergentes em run.started retomado")
			}
		case "step.finished":
			if !seenStart || ev.Package == "" {
				return h, fmt.Errorf("E-RES-003: resultado sem execução ou pacote")
			}
			if h.LastExitCode != nil && *h.LastExitCode == 0 {
				return h, fmt.Errorf("E-RES-003: eventos após conclusão terminal")
			}
			switch ev.Outcome {
			case "skipped", "installed", "installed_reboot", "failed":
			default:
				return h, fmt.Errorf("E-RES-003: outcome inválido para %s: %q", ev.Package, ev.Outcome)
			}
			h.Results[ev.Package] = ev
		case "run.finished":
			if !seenStart || ev.ExitCode == nil {
				return h, fmt.Errorf("E-RES-003: término sem início ou código")
			}
			if h.LastExitCode != nil && *h.LastExitCode == 0 {
				return h, fmt.Errorf("E-RES-003: término duplicado após sucesso")
			}
			h.LastExitCode = ev.ExitCode
		}
		if readErr == io.EOF {
			break
		}
	}
	if !seenStart {
		return h, fmt.Errorf("E-RES-001: runId não encontrado")
	}
	return h, nil
}
