//go:build windows

package platform

import (
	"fmt"
	"syscall"
	"unsafe"
)
type osVersionInfo struct{
	Size uint32
	Major uint32
	Minor uint32
	Build uint32
	PlatformID uint32
	ServicePack [128]uint16
}
func WindowsBuild()(int,error){
	v:=osVersionInfo{Size:uint32(unsafe.Sizeof(osVersionInfo{}))}
	ntdll:=syscall.NewLazyDLL("ntdll.dll")
	ret,_,err:=ntdll.NewProc("RtlGetVersion").Call(uintptr(unsafe.Pointer(&v)))
	if ret!=0{return 0,fmt.Errorf("RtlGetVersion failed (NTSTATUS %#x): %v",ret,err)}
	return int(v.Build),nil
}
func AcquireInstallerLock()(func(),error){
	name,err:=syscall.UTF16PtrFromString(`Global\DeployCraftInstaller`)
	if err!=nil{return nil,err}
	kernel:=syscall.NewLazyDLL("kernel32.dll")
	handle,_,lastErr:=kernel.NewProc("CreateMutexW").Call(0,0,uintptr(unsafe.Pointer(name)))
	if handle==0{return nil,fmt.Errorf("E-PRE-005: CreateMutexW falhou: %v",lastErr)}
	if lastErr==syscall.ERROR_ALREADY_EXISTS {
		syscall.CloseHandle(syscall.Handle(handle))
		return nil,fmt.Errorf("E-PRE-005: outra instância do DeployCraft está ativa")
	}
	return func(){_ = syscall.CloseHandle(syscall.Handle(handle))},nil
}
