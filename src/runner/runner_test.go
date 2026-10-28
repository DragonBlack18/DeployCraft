package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func executableFixture(t *testing.T) domain.Command {
	t.Helper()
	if runtime.GOOS == "windows" {
		return domain.Command{Exe: "cmd.exe", Args: []string{"/C", "echo deploycraft-test"}}
	}
	return domain.Command{Exe: "/bin/echo", Args: []string{"deploycraft-test"}}
}
func TestRunsProcessWithoutShell(t *testing.T) {
	proc, err := New().Start(executableFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	code, err := proc.Wait(context.Background())
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	tailer, ok := proc.(interface{ Tail() string })
	if !ok || !strings.Contains(tailer.Tail(), "deploycraft-test") {
		t.Fatal("missing output tail")
	}
}
func TestExitCodeIsPreserved(t *testing.T) {
	var cmd domain.Command
	if runtime.GOOS == "windows" {
		cmd = domain.Command{Exe: "cmd.exe", Args: []string{"/C", "exit /B 17"}}
	} else {
		cmd = domain.Command{Exe: "/bin/false"}
	}
	proc, err := New().Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	got, err := proc.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && got != 17 || runtime.GOOS != "windows" && got != 1 {
		t.Fatalf("wrong exit code %d", got)
	}
}
func TestNoShellInterpolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX literal-argument test")
	}
	marker := filepath.Join(t.TempDir(), "owned")
	proc, err := New().Start(domain.Command{Exe: "/bin/echo", Args: []string{"$(touch " + marker + ")"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proc.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("shell expression was expanded")
	}
}
func TestTimeoutKillsDirectProcess(t *testing.T) {
	var cmd domain.Command
	if runtime.GOOS == "windows" {
		cmd = domain.Command{Exe: "powershell.exe", Args: []string{"-NoProfile", "-Command", "Start-Sleep -Seconds 8"}}
	} else {
		cmd = domain.Command{Exe: "/bin/sleep", Args: []string{"8"}}
	}
	proc, err := New().Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err = proc.Wait(ctx)
	if err == nil {
		t.Fatal("expected timeout/cancellation")
	}
}
