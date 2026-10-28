package detect

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type fakeRegistry struct {
	keys   map[string][]string
	values map[string]string
	err    error
	lists  int
}

func (f *fakeRegistry) ListSubKeys(key string) ([]string, error) {
	f.lists++
	if f.err != nil {
		return nil, f.err
	}
	return f.keys[key], nil
}
func (f *fakeRegistry) ReadString(key, value string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	x, ok := f.values[key+"/"+value]
	return x, ok, nil
}

type fakePaths struct{}

func (fakePaths) ProgramData() string    { return "/tmp" }
func (fakePaths) Temp() string           { return "/tmp" }
func (fakePaths) Expand(s string) string { return s }
func pkgWith(rules ...domain.DetectRule) domain.Package {
	return domain.Package{ID: "sample", Version: "2.0",
		Detect: domain.Detect{Match: "any", Rules: rules}}
}
func TestRegistryInstalledOutdatedAbsent(t *testing.T) {
	const root = `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	f := &fakeRegistry{
		keys: map[string][]string{root: {"{TEST}"}},
		values: map[string]string{
			root + `\{TEST}` + "/DisplayName":    "Example Editor",
			root + `\{TEST}` + "/DisplayVersion": "2.2.0",
		},
	}
	p := pkgWith(domain.DetectRule{Type: "registry-uninstall", DisplayName: "Example*"})
	s := NewScan(f, fakePaths{})
	if got := s.Detect(p).State; got != domain.StateInstalled {
		t.Fatalf("want installed, got %s", got)
	}
	f.values[root+`\{TEST}`+"/DisplayVersion"] = "1.2"
	s = NewScan(f, fakePaths{})
	if got := s.Detect(p).State; got != domain.StateOutdated {
		t.Fatalf("want outdated, got %s", got)
	}
	p.Detect.Rules[0].DisplayName = "Nonexistent*"
	s = NewScan(f, fakePaths{})
	if got := s.Detect(p).State; got != domain.StateAbsent {
		t.Fatalf("want absent, got %s", got)
	}
}
func TestRegistryCache(t *testing.T) {
	f := &fakeRegistry{keys: map[string][]string{}, values: map[string]string{}}
	s := NewScan(f, fakePaths{})
	p := pkgWith(domain.DetectRule{Type: "registry-uninstall", DisplayName: "Anything*"})
	s.Detect(p)
	s.Detect(p)
	if f.lists != len(uninstallRoots) {
		t.Fatalf("registry scanned %d times; want %d", f.lists, len(uninstallRoots))
	}
}
func TestRegistryErrorsAreUnknown(t *testing.T) {
	f := &fakeRegistry{err: fmt.Errorf("access denied")}
	p := pkgWith(domain.DetectRule{Type: "registry-uninstall", DisplayName: "Example*"})
	got := NewScan(f, fakePaths{}).Detect(p)
	if got.State != domain.StateUnknown || !strings.Contains(strings.Join(got.Evidence, ""), "access denied") {
		t.Fatalf("unexpected detection %+v", got)
	}
}
func TestUnsupportedRuleIsUnknown(t *testing.T) {
	p := pkgWith(domain.DetectRule{Type: "command", Command: "arbitrary", Matches: "x"})
	f := &fakeRegistry{}
	if got := NewScan(f, fakePaths{}).Detect(p).State; got != domain.StateUnknown {
		t.Fatalf("unsupported command must not be absent or installed: %s", got)
	}
}
func TestRegistryValueCompare(t *testing.T) {
	f := &fakeRegistry{values: map[string]string{"HKLM\\APP/Build": "5"}}
	min := int64(6)
	p := pkgWith(domain.DetectRule{Type: "registry-value", Key: "HKLM\\APP", Value: "Build", AtLeast: &min})
	if got := NewScan(f, fakePaths{}).Detect(p).State; got != domain.StateOutdated {
		t.Fatalf("got %s", got)
	}
}
