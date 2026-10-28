//go:build windows

package platform

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsBuildIsSupported(t *testing.T) {
	build, err := WindowsBuild()
	if err != nil {
		t.Fatal(err)
	}
	if build < 17763 {
		t.Fatalf("unexpected Windows build: %d", build)
	}
}
func TestWindowsRegistryReadKnownValue(t *testing.T) {
	r := NewRegistry()
	value, ok, err := r.ReadString(
		`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || value == "" {
		t.Fatal("expected Windows ProductName from actual registry")
	}
	_, ok, err = r.ReadString(
		`HKCU\Software\DeployCraftNonexistentTestKey`, "NoValue")
	if err != nil || ok {
		t.Fatalf("missing HKCU registry value should be absent, ok=%v err=%v", ok, err)
	}
}
func TestWindowsRegistryEnumeratesKeys(t *testing.T) {
	r := NewRegistry()
	names, err := r.ListSubKeys(`HKLM\SOFTWARE\Microsoft`)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, name := range names {
		if strings.EqualFold(name, "Windows NT") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Windows NT subkey in real HKLM registry")
	}
}
func TestWindowsPathsExpandPercentVariables(t *testing.T) {
	root := os.Getenv("ProgramFiles")
	if root == "" {
		t.Skip("ProgramFiles missing")
	}
	got := NewPaths().Expand(`%ProgramFiles%\DeployCraft`)
	if got != root+`\DeployCraft` {
		t.Fatalf("bad environment expansion: %q", got)
	}
}
func TestGlobalInstallerMutexPreventsConcurrentExecution(t *testing.T) {
	unlock, err := AcquireInstallerLock()
	if err != nil {
		t.Fatal(err)
	}
	_, err = AcquireInstallerLock()
	if err == nil {
		unlock()
		t.Fatal("second installer acquired active Global mutex")
	}
	unlock()
	unlock2, err := AcquireInstallerLock()
	if err != nil {
		t.Fatalf("mutex not released: %v", err)
	}
	unlock2()
}
