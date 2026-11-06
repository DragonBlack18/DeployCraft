//go:build windows

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

var errJobTerminated = errors.New("E-RUN-001: grupo de processos encerrado")

// One owner monitors native handles. done publishes the immutable wait result.
// No child inherits the job handle, so owner death triggers kill-on-close.
type windowsRunning struct {
	mu            sync.Mutex
	job, process  syscall.Handle
	tail          *tailBuffer
	output        *os.File
	outputDone    chan struct{}
	done          chan struct{}
	code          int
	err, abortErr error
}

func windowsEnvironment(overrides []string) ([]uint16, error) {
	values := map[string]string{}
	for _, entry := range append(os.Environ(), overrides...) {
		if strings.ContainsRune(entry, 0) {
			return nil, fmt.Errorf("E-DRV-002: ambiente contém NUL")
		}
		// Preserve Windows pseudo-variables such as =C:=C:\\directory.
		split := strings.Index(entry[minimum(1, len(entry)):], "=") + 1
		if split < 1 || split >= len(entry) {
			return nil, fmt.Errorf("E-DRV-002: variável de ambiente inválida")
		}
		values[strings.ToUpper(entry[:split])] = entry
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	block := make([]uint16, 0)
	for _, key := range keys {
		block = append(block, utf16.Encode([]rune(values[key]))...)
		block = append(block, 0)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	return append(block, 0), nil
}
func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (nativeProcess) Start(command domain.Command) (domain.Running, error) {
	return startWindows(command, newJob)
}

func startWindows(command domain.Command, makeJob func() (syscall.Handle, error)) (domain.Running, error) {
	if runtime.GOARCH != "amd64" {
		return nil, fmt.Errorf("E-RUN-002: Job Object requer Windows x64")
	}
	if command.Exe == "" || command.RawCmdLine != "" {
		return nil, fmt.Errorf("E-DRV-002: comando vazio ou rawCmdLine indisponível")
	}
	executable, err := exec.LookPath(command.Exe)
	if err != nil {
		return nil, fmt.Errorf("E-RUN-002: executável indisponível: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	application, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, len(command.Args)+1)
	parts = append(parts, syscall.EscapeArg(executable))
	for _, arg := range command.Args {
		if strings.ContainsRune(arg, 0) {
			return nil, fmt.Errorf("E-DRV-002: argumento contém NUL")
		}
		parts = append(parts, syscall.EscapeArg(arg))
	}
	line, err := syscall.UTF16FromString(strings.Join(parts, " "))
	if err != nil {
		return nil, err
	}
	var cwd *uint16
	if command.Cwd != "" {
		abs, err := filepath.Abs(command.Cwd)
		if err != nil {
			return nil, err
		}
		cwd, err = syscall.UTF16PtrFromString(abs)
		if err != nil {
			return nil, err
		}
	}
	environment, err := windowsEnvironment(command.Env)
	if err != nil {
		return nil, err
	}
	job, err := makeJob()
	if err != nil {
		return nil, err
	}
	owned := false
	defer func() {
		if !owned {
			syscall.CloseHandle(job)
		}
	}()

	sa := syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), InheritHandle: 1}
	var readHandle, writeHandle syscall.Handle
	if err := syscall.CreatePipe(&readHandle, &writeHandle, &sa, 0); err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(writeHandle)
	if err := syscall.SetHandleInformation(readHandle, syscall.HANDLE_FLAG_INHERIT, 0); err != nil {
		syscall.CloseHandle(readHandle)
		return nil, err
	}
	output := os.NewFile(uintptr(readHandle), "DeployCraft-output")
	defer func() {
		if !owned {
			output.Close()
		}
	}()
	nul, _ := syscall.UTF16PtrFromString("NUL")
	stdin, err := syscall.CreateFile(nul, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, &sa, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(stdin)
	attrs, err := newAttributes(job, stdin, writeHandle)
	if err != nil {
		return nil, err
	}
	defer attrs.close()
	si := startupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = syscall.STARTF_USESTDHANDLES
	si.StdInput = stdin
	si.StdOutput = writeHandle
	si.StdErr = writeHandle
	si.Attributes = attrs.pointer()
	var pi syscall.ProcessInformation
	flags := uint32(extendedStartupInfoPresent | createSuspended | createNoWindow | createUnicodeEnvironment)
	// JOB_LIST binds the process at creation; there is no uncontained start.
	// HANDLE_LIST passes only stdin/stdout, never the Job Object handle.
	err = syscall.CreateProcess(application, &line[0], nil, nil, true, flags, &environment[0], cwd, &si.StartupInfo, &pi)
	runtime.KeepAlive(attrs)
	runtime.KeepAlive(environment)
	runtime.KeepAlive(line)
	if err != nil {
		return nil, fmt.Errorf("E-RUN-002: criação dentro do Job Object falhou: %w", err)
	}
	defer syscall.CloseHandle(pi.Thread)
	// Testable invariant: failure here terminates the already-contained process.
	count, _, resumeErr := resumeThread.Call(uintptr(pi.Thread))
	if uint32(count) == 0xffffffff {
		_ = stopJob(job)
		syscall.CloseHandle(pi.Process)
		return nil, windowsCallError("ResumeThread", resumeErr)
	}
	r := &windowsRunning{job: job, process: pi.Process, tail: &tailBuffer{}, output: output,
		outputDone: make(chan struct{}), done: make(chan struct{}), code: -1}
	owned = true
	go func() { _, _ = io.Copy(r.tail, output); close(r.outputDone) }()
	go r.monitor()
	return r, nil
}
func (r *windowsRunning) Tail() string { return r.tail.String() }
func (r *windowsRunning) Kill() error  { return r.abort(errJobTerminated) }
func (r *windowsRunning) abort(cause error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.job == 0 {
		return nil
	}
	if r.abortErr == nil {
		r.abortErr = cause
	}
	if err := stopJob(r.job); err != nil {
		r.abortErr = errors.Join(r.abortErr, err)
		// Kill-on-close is a last resort; do not report successful completion.
		syscall.CloseHandle(r.job)
		r.job = 0
		return err
	}
	return nil
}
func (r *windowsRunning) monitor() {
	var resultErr error
	var code uint32
	for {
		r.mu.Lock()
		if r.job == 0 {
			resultErr = r.abortErr
			r.mu.Unlock()
			break
		}
		count, err := activeProcesses(r.job)
		r.mu.Unlock()
		if err != nil {
			resultErr = err
			_ = r.abort(err)
			break
		}
		if count == 0 {
			state, err := syscall.WaitForSingleObject(r.process, 0)
			if err != nil {
				resultErr = err
				break
			}
			if state == syscall.WAIT_OBJECT_0 {
				resultErr = syscall.GetExitCodeProcess(r.process, &code)
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	r.mu.Lock()
	if r.abortErr != nil {
		resultErr = errors.Join(resultErr, r.abortErr)
	}
	if r.job != 0 {
		syscall.CloseHandle(r.job)
		r.job = 0
	}
	syscall.CloseHandle(r.process)
	r.process = 0
	r.mu.Unlock()
	// Ordinarily all writers are closed once ActiveProcesses is zero. Bound
	// draining if an external broker duplicated a pipe beyond our job boundary.
	select {
	case <-r.outputDone:
	case <-time.After(2 * time.Second):
		resultErr = errors.Join(resultErr, errors.New("E-RUN-003: saída de processo não encerrou"))
	}
	_ = r.output.Close()
	r.code = int(code)
	r.err = resultErr
	if resultErr != nil {
		r.code = -1
	}
	close(r.done)
}

// Wait is repeatable/concurrent; it never calls an OS wait twice on a released
// handle. Cancellation is destructive to the whole associated job, by contract.
func (r *windowsRunning) Wait(ctx context.Context) (int, error) {
	if ctx == nil {
		return -1, errors.New("E-RUN-002: contexto ausente")
	}
	select {
	case <-r.done:
		return r.code, r.err
	default:
	}
	select {
	case <-r.done:
		return r.code, r.err
	case <-ctx.Done():
		killErr := r.abort(ctx.Err())
		select {
		case <-r.done:
			return -1, errors.Join(ctx.Err(), killErr, r.err)
		case <-time.After(10 * time.Second):
			return -1, errors.Join(ctx.Err(), killErr, errors.New("E-RUN-003: encerramento da árvore não foi confirmado"))
		}
	}
}
