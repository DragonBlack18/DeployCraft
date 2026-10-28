package drivers

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func TestMSICommandAndExitCodes(t *testing.T) {
	d, err := New("msi")
	if err != nil {
		t.Fatal(err)
	}
	p := domain.Package{Install: domain.Install{Driver: "msi",
		Args:         []json.RawMessage{json.RawMessage(`"ALLUSERS=1"`)},
		SuccessCodes: []int{0}, RebootCodes: []int{3010}, RetryCodes: []int{1618}}}
	file := filepath.Join(t.TempDir(), "installer.msi")
	log := filepath.Join(t.TempDir(), "package.log")
	cmd, err := d.Prepare(p, file, domain.RunContext{LogPath: log, RunID: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Exe != "msiexec.exe" || len(cmd.Args) != 7 || cmd.Args[0] != "/i" || cmd.Args[2] != "/qn" || cmd.Args[6] != "ALLUSERS=1" {
		t.Fatalf("invalid MSI command: %+v", cmd)
	}
	if v, _ := d.Interpret(0, p); v != domain.VerdictSuccess {
		t.Fatal("0")
	}
	if v, _ := d.Interpret(3010, p); v != domain.VerdictSuccessReboot {
		t.Fatal("3010")
	}
	if v, _ := d.Interpret(1618, p); v != domain.VerdictRetry {
		t.Fatal("1618")
	}
	if v, _ := d.Interpret(1603, p); v != domain.VerdictFailure {
		t.Fatal("1603")
	}
}
func TestSecretIsMasked(t *testing.T) {
	d, _ := New("exe")
	p := domain.Package{Install: domain.Install{Driver: "exe", Args: []json.RawMessage{
		json.RawMessage(`{"value":"/SERIAL={secret}","secret":"license"}`),
	}}}
	secret := "confidencial-EXAMPLE-123"
	file := filepath.Join(t.TempDir(), "setup.exe")
	cmd, err := d.Prepare(p, file, domain.RunContext{Secrets: map[string]string{"license": secret}, LogPath: "log.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd.Args[0], secret) {
		t.Fatal("secret not supplied")
	}
	if strings.Contains(cmd.LogLine, secret) || !strings.Contains(cmd.LogLine, "***") {
		t.Fatalf("secret leaked into LogLine: %s", cmd.LogLine)
	}
}
func TestRawCommandLineRejectedBeforeExecution(t *testing.T) {
	d, _ := New("exe")
	p := domain.Package{Install: domain.Install{Driver: "exe", RawCmdLine: `/S /D=C:\App`}}
	_, err := d.Prepare(p, filepath.Join(t.TempDir(), "setup.exe"), domain.RunContext{})
	if err == nil {
		t.Fatal("raw command line incorrectly accepted")
	}
}
func TestUnknownMarkerRejected(t *testing.T) {
	d, _ := New("exe")
	p := domain.Package{Install: domain.Install{Driver: "exe", Args: []json.RawMessage{json.RawMessage(`"{other}"`)}}}
	_, err := d.Prepare(p, filepath.Join(t.TempDir(), "setup.exe"), domain.RunContext{})
	if err == nil {
		t.Fatal("unrecognized marker incorrectly accepted")
	}
}
