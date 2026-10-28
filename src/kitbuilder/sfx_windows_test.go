//go:build windows

package kitbuilder

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the actual compiled Windows SFX CLI, but do not execute third-party
// installers (the payload in this test is intentionally nonexecutable).
func TestWindowsSFXListDoesNotExtractPayloads(t *testing.T) {
	root := fixtureRoot(t)
	sourceRoot,err:=filepath.Abs(filepath.Join("..",".."))
	if err!=nil{t.Fatal(err)}
	addExample(t, root)
	keys := t.TempDir()
	private := filepath.Join(keys, "private.pem")
	public := filepath.Join(keys, "public.pem")
	if err := GenerateSigningKeys(root, private, public); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "DeployCraft-SFX.exe")
	if _, err := BuildSFX(BuildOptions{Root: root, SourceRoot: sourceRoot, Destination: dest, PrivateKeyPath: private}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dest, "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("SFX CLI did not start: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "example") {
		t.Fatalf("embedded catalog not listed: %s", string(out))
	}
}
