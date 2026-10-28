package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRunAndTruncatedTail(t *testing.T) {
	s := New(t.TempDir())
	id, err := NewRunID()
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{
		{RunID: id, Event: "run.started", KitRoot: "C:/kit", Catalog: "2026.10.1", Selection: &StoredSelection{Profile: "dev"}, Options: &StoredOptions{StopOnError: true}},
		{RunID: id, Event: "step.finished", Package: "base", Outcome: "installed_reboot", DurationMs: 20},
		{RunID: id, Event: "run.finished", ExitCode: intPtr(10)},
	}
	for _, ev := range events {
		if err := s.Append(ev); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.OpenFile(filepath.Join(s.Root, "state", "journal.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"event":"run.started","runId":"` + id + `","selection":`)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	h, err := s.ReadRun(id)
	if err != nil {
		t.Fatal(err)
	}
	if h.Start.Selection.Profile != "dev" || h.Results["base"].Outcome != "installed_reboot" || h.LastExitCode == nil || *h.LastExitCode != 10 {
		t.Fatalf("wrong recovery %+v", h)
	}
	if _, err := s.ReadRun(strings.Repeat("f", 32)); err == nil {
		t.Fatal("unknown run accepted")
	}
}
func TestCorruptedCompleteEventFailsClosed(t *testing.T) {
	s := New(t.TempDir())
	id, err := NewRunID()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Event{RunID: id, Event: "run.started", Selection: &StoredSelection{}, Options: &StoredOptions{}, Catalog: "1", KitRoot: "C:/kit"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Root, "state", "journal.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("{not json}\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadRun(id); err == nil {
		t.Fatal("corrupt complete line accepted")
	}
}
func TestRunStartedMustHaveMetadata(t *testing.T) {
	s := New(t.TempDir())
	id, err := NewRunID()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Event{RunID: id, Event: "run.started"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadRun(id); err == nil {
		t.Fatal("missing metadata accepted")
	}
}
func intPtr(x int) *int { return &x }
