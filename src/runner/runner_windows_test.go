//go:build windows

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func helperCommand(t *testing.T, mode, dir string, extra ...string) domain.Command {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"-test.run=^TestRunnerWindowsHelperProcess$", "--", mode, dir}, extra...)
	return domain.Command{Exe: exe, Args: args, Env: []string{"DEPLOYCRAFT_JOB_HELPER=1"}}
}
func waitForFile(t *testing.T, name string) []byte {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(name)
		if err == nil && len(b) > 0 {
			return b
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper did not signal readiness: %s", name)
	return nil
}
func treeHandles(t *testing.T, dir string) []syscall.Handle {
	t.Helper()
	var handles []syscall.Handle
	for _, name := range []string{"parent", "child", "grandchild"} {
		raw := waitForFile(t, filepath.Join(dir, name+".pid"))
		pid, err := strconv.Atoi(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
		if err != nil {
			t.Fatal(err)
		}
		handles = append(handles, h)
		t.Cleanup(func() { syscall.CloseHandle(h) })
	}
	return handles
}
func assertExited(t *testing.T, handles []syscall.Handle) {
	t.Helper()
	for _, h := range handles {
		state, err := syscall.WaitForSingleObject(h, 3000)
		if err != nil || state != syscall.WAIT_OBJECT_0 {
			t.Fatalf("owned process survived: state=%d err=%v", state, err)
		}
	}
}
func TestJobCancelsParentChildAndGrandchild(t *testing.T) {
	for _, mode := range []string{"deadline", "cancel", "kill"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			p, err := New().Start(helperCommand(t, "parent", dir))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Kill()
			handles := treeHandles(t, dir)
			if mode == "deadline" {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				_, err = p.Wait(ctx)
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline not preserved: %v", err)
				}
			} else if mode == "cancel" {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err = p.Wait(ctx)
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel not preserved: %v", err)
				}
			} else {
				if err = p.Kill(); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err = p.Wait(ctx)
				if !errors.Is(err, errJobTerminated) {
					t.Fatalf("kill not reported: %v", err)
				}
			}
			assertExited(t, handles)
			if err := p.Kill(); err != nil {
				t.Fatalf("Kill after completion: %v", err)
			}
		})
	}
}
func TestJobWaitsForChildAfterLauncherExitsAndIsRepeatable(t *testing.T) {
	dir := t.TempDir()
	p, err := New().Start(helperCommand(t, "launcher", dir))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := p.Wait(ctx)
			if err != nil || code != 17 {
				t.Errorf("code=%d err=%v", code, err)
			}
		}()
	}
	wg.Wait()
	if _, err := os.Stat(filepath.Join(dir, "child.done")); err != nil {
		t.Fatal("Wait returned before child finished")
	}
	code, err := p.Wait(context.Background())
	if err != nil || code != 17 {
		t.Fatal("repeat Wait changed result")
	}
	if !strings.Contains(p.(interface{ Tail() string }).Tail(), "child-finished") {
		t.Fatal("child stdout was lost")
	}
}
func TestJobOwnerDeathTerminatesDescendants(t *testing.T) {
	dir := t.TempDir()
	command := helperCommand(t, "owner", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owner := exec.CommandContext(ctx, command.Exe, command.Args...)
	owner.Env = append(os.Environ(), command.Env...)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer owner.Process.Kill()
	handles := treeHandles(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "owner.release"), []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal("owner must exit abruptly")
	}
	assertExited(t, handles)
}
func TestJobPreservesArgumentsEnvironmentAndWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	args := []string{"with spaces", `quoted"value`, `C:\folder with space\`, "", "& echo not-a-shell", "olá"}
	cmd := helperCommand(t, "echo", dir, args...)
	cmd.Cwd = dir
	cmd.Env = append(cmd.Env, "DEPLOYCRAFT_CASE=old", "deploycraft_case=new")
	p, err := New().Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Kill()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, err := p.Wait(ctx)
	if err != nil || code != 0 {
		t.Fatalf("echo code=%d err=%v", code, err)
	}
	var got struct {
		Args       []string
		Value, Cwd string
	}
	raw, err := os.ReadFile(filepath.Join(dir, "echo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, got.Args) || got.Value != "new" || !strings.EqualFold(got.Cwd, dir) {
		t.Fatalf("argument/environment corruption: %+v", got)
	}
}
func TestJobCreationFailureNeverRunsPayload(t *testing.T) {
	dir := t.TempDir()
	want := errors.New("job creation refused")
	_, err := startWindows(helperCommand(t, "echo", dir), func() (syscall.Handle, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatalf("creation failure not propagated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo.json")); !os.IsNotExist(err) {
		t.Fatal("payload executed without containment")
	}
}
func TestJobABIAndEnvironmentValidation(t *testing.T) {
	if unsafe.Sizeof(jobExtendedLimits{}) != 144 || unsafe.Sizeof(jobAccounting{}) != 48 || unsafe.Sizeof(startupInfoEx{}) != 112 {
		t.Fatal("Windows x64 structure ABI mismatch")
	}
	for _, item := range []string{"no_separator", "=invalid", "A=bad\x00value", ""} {
		if _, err := windowsEnvironment([]string{item}); err == nil {
			t.Fatalf("invalid env accepted: %q", item)
		}
	}
}

// Own-source disposable fixture: never launches a third-party installer.
func TestRunnerWindowsHelperProcess(t *testing.T) {
	if os.Getenv("DEPLOYCRAFT_JOB_HELPER") != "1" {
		return
	}
	split := -1
	for i, s := range os.Args {
		if s == "--" {
			split = i
			break
		}
	}
	if split < 0 || len(os.Args) < split+3 {
		os.Exit(99)
	}
	mode, dir := os.Args[split+1], os.Args[split+2]
	must := func(err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
	}
	spawn := func(mode string) {
		child := exec.Command(os.Args[0], "-test.run=^TestRunnerWindowsHelperProcess$", "--", mode, dir)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		must(child.Start())
	}
	switch mode {
	case "parent", "child", "grandchild":
		must(os.WriteFile(filepath.Join(dir, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600))
		if mode == "parent" {
			spawn("child")
		}
		if mode == "child" {
			spawn("grandchild")
		}
		time.Sleep(30 * time.Second)
	case "launcher":
		spawn("short-child")
		os.Exit(17)
	case "short-child":
		time.Sleep(400 * time.Millisecond)
		fmt.Println("child-finished")
		must(os.WriteFile(filepath.Join(dir, "child.done"), []byte("done"), 0600))
	case "echo":
		cwd, err := os.Getwd()
		must(err)
		out, err := json.Marshal(struct {
			Args       []string
			Value, Cwd string
		}{os.Args[split+3:], os.Getenv("DEPLOYCRAFT_CASE"), cwd})
		must(err)
		must(os.WriteFile(filepath.Join(dir, "echo.json"), out, 0600))
	case "owner":
		_, err := New().Start(domain.Command{Exe: os.Args[0], Args: []string{"-test.run=^TestRunnerWindowsHelperProcess$", "--", "parent", dir}})
		must(err)
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(dir, "owner.release")); err == nil {
				os.Exit(23)
			}
			time.Sleep(10 * time.Millisecond)
		}
		os.Exit(97)
	default:
		os.Exit(96)
	}
	os.Exit(0)
}
