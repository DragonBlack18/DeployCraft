//go:build windows

package runner

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	jobExtendedLimitInformation   = 9
	jobBasicAccountingInformation = 1
	jobKillOnClose                = 0x2000
	attributeHandleList           = 0x00020002
	attributeJobList              = 0x0002000d
	extendedStartupInfoPresent    = 0x00080000
	createSuspended               = 0x00000004
	createNoWindow                = 0x08000000
	createUnicodeEnvironment      = 0x00000400
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	createJob        = kernel32.NewProc("CreateJobObjectW")
	setJob           = kernel32.NewProc("SetInformationJobObject")
	queryJob         = kernel32.NewProc("QueryInformationJobObject")
	terminateJob     = kernel32.NewProc("TerminateJobObject")
	initAttributes   = kernel32.NewProc("InitializeProcThreadAttributeList")
	updateAttributes = kernel32.NewProc("UpdateProcThreadAttribute")
	deleteAttributes = kernel32.NewProc("DeleteProcThreadAttributeList")
	resumeThread     = kernel32.NewProc("ResumeThread")
)

// Native Windows x64 ABI; production rejects other architectures before use.
type jobBasicLimits struct {
	PerProcessTime, PerJobTime           int64
	LimitFlags                           uint32
	MinimumWorkingSet, MaximumWorkingSet uintptr
	ActiveProcessLimit                   uint32
	Affinity                             uintptr
	PriorityClass, SchedulingClass       uint32
}
type jobExtendedLimits struct {
	Basic                                                      jobBasicLimits
	IO                                                         [6]uint64
	ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory uintptr
}
type jobAccounting struct {
	Times                                                            [4]int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}
type startupInfoEx struct {
	syscall.StartupInfo
	Attributes *uintptr
}

func windowsCallError(operation string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		err = syscall.EINVAL
	}
	return fmt.Errorf("E-RUN-002: %s: %w", operation, err)
}
func newJob() (syscall.Handle, error) {
	h, _, err := createJob.Call(0, 0) // Unnamed, non-inheritable, private to this run.
	if h == 0 {
		return 0, windowsCallError("CreateJobObjectW", err)
	}
	info := jobExtendedLimits{}
	info.Basic.LimitFlags = jobKillOnClose // No breakaway flags are allowed.
	ok, _, err := setJob.Call(h, jobExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if ok == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return 0, windowsCallError("SetInformationJobObject", err)
	}
	return syscall.Handle(h), nil
}
func activeProcesses(job syscall.Handle) (uint32, error) {
	var stats jobAccounting
	ok, _, err := queryJob.Call(uintptr(job), jobBasicAccountingInformation, uintptr(unsafe.Pointer(&stats)), unsafe.Sizeof(stats), 0)
	if ok == 0 {
		return 0, windowsCallError("QueryInformationJobObject", err)
	}
	return stats.ActiveProcesses, nil
}
func stopJob(job syscall.Handle) error {
	ok, _, err := terminateJob.Call(uintptr(job), 1)
	if ok == 0 {
		return windowsCallError("TerminateJobObject", err)
	}
	return nil
}

// Attributes and their values must remain alive until the attribute list is
// deleted, not merely until UpdateProcThreadAttribute has returned.
type processAttributes struct {
	storage []uintptr
	jobs    [1]syscall.Handle
	handles [2]syscall.Handle
}

func (a *processAttributes) pointer() *uintptr { return &a.storage[0] }
func (a *processAttributes) close() {
	deleteAttributes.Call(uintptr(unsafe.Pointer(a.pointer())))
	runtime.KeepAlive(a)
}
func newAttributes(job, stdin, stdout syscall.Handle) (*processAttributes, error) {
	var size uintptr
	_, _, err := initAttributes.Call(0, 2, 0, uintptr(unsafe.Pointer(&size)))
	if err != syscall.ERROR_INSUFFICIENT_BUFFER || size == 0 || size > 65536 {
		return nil, windowsCallError("attribute size", err)
	}
	unit := unsafe.Sizeof(uintptr(0))
	a := &processAttributes{storage: make([]uintptr, (size+unit-1)/unit), jobs: [1]syscall.Handle{job}, handles: [2]syscall.Handle{stdin, stdout}}
	ptr := uintptr(unsafe.Pointer(a.pointer()))
	ok, _, err := initAttributes.Call(ptr, 2, 0, uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return nil, windowsCallError("InitializeProcThreadAttributeList", err)
	}
	ok, _, err = updateAttributes.Call(ptr, 0, attributeJobList, uintptr(unsafe.Pointer(&a.jobs[0])), unsafe.Sizeof(a.jobs), 0, 0)
	if ok == 0 {
		a.close()
		return nil, windowsCallError("job-list attribute", err)
	}
	ok, _, err = updateAttributes.Call(ptr, 0, attributeHandleList, uintptr(unsafe.Pointer(&a.handles[0])), unsafe.Sizeof(a.handles), 0, 0)
	if ok == 0 {
		a.close()
		return nil, windowsCallError("handle-list attribute", err)
	}
	return a, nil
}
