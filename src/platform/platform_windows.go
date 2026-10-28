//go:build windows

// Package platform isolates the Windows APIs used by the DeployCraft motor.
package platform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type registryAdapter struct{}
type pathsAdapter struct{}

// NewRegistry returns a Windows registry adapter; no shell processes are used.
func NewRegistry() domain.Registry { return registryAdapter{} }
func NewPaths() domain.Paths       { return pathsAdapter{} }

func (pathsAdapter) ProgramData() string { return os.Getenv("ProgramData") }
func (pathsAdapter) Temp() string        { return os.TempDir() }
func (pathsAdapter) Expand(text string) string {
	return os.Expand(text, func(key string) string { return os.Getenv(key) })
}

func splitKey(key string) (syscall.Handle, string, error) {
	up := strings.ToUpper(key)
	switch {
	case strings.HasPrefix(up, "HKLM\\"):
		return syscall.HKEY_LOCAL_MACHINE, key[5:], nil
	case strings.HasPrefix(up, "HKCU\\"):
		return syscall.HKEY_CURRENT_USER, key[5:], nil
	}
	return 0, "", fmt.Errorf("unsupported registry root: %s", key)
}
func openKey(key string) (syscall.Handle, error) {
	root, path, err := splitKey(key)
	if err != nil { return 0, err }
	utf, err := syscall.UTF16PtrFromString(path)
	if err != nil { return 0, err }
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(root, utf, 0, syscall.KEY_READ|syscall.KEY_WOW64_64KEY, &h); err != nil {
		return 0, err
	}
	return h, nil
}

func (registryAdapter) ReadString(key, value string) (string, bool, error) {
	h, err := openKey(key)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) { return "", false, nil }
	if err != nil { return "", false, err }
	defer syscall.RegCloseKey(h)
	name, err := syscall.UTF16PtrFromString(value)
	if err != nil { return "", false, err }
	var kind, n uint32
	err = syscall.RegQueryValueEx(h, name, nil, &kind, nil, &n)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) { return "", false, nil }
	if err != nil && !errors.Is(err, syscall.ERROR_MORE_DATA) { return "", false, err }
	if n > 16<<20 { return "", false, fmt.Errorf("registry value too large: %s", key) }
	if n == 0 { return "", true, nil }
	buf := make([]byte, n)
	if err := syscall.RegQueryValueEx(h, name, nil, &kind, &buf[0], &n); err != nil { return "", false, err }
	switch kind {
	case syscall.REG_DWORD:
		if n < 4 { return "", false, fmt.Errorf("invalid DWORD: %s", key) }
		return fmt.Sprint(binary.LittleEndian.Uint32(buf)), true, nil
	case syscall.REG_QWORD:
		if n < 8 { return "", false, fmt.Errorf("invalid QWORD: %s", key) }
		return fmt.Sprint(binary.LittleEndian.Uint64(buf)), true, nil
	case syscall.REG_SZ, syscall.REG_EXPAND_SZ:
		u := make([]uint16, 0, int(n)/2)
		for i := 0; i+1 < int(n); i += 2 { u = append(u, binary.LittleEndian.Uint16(buf[i:])) }
		text := string(utf16.Decode(u))
		return strings.TrimRight(text, "\x00"), true, nil
	}
	return "", false, fmt.Errorf("unsupported registry value type %d", kind)
}
func (registryAdapter) ListSubKeys(key string) ([]string, error) {
	h, err := openKey(key)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) { return nil, nil }
	if err != nil { return nil, err }
	defer syscall.RegCloseKey(h)
	var names []string
	for index := uint32(0); ; index++ {
		buf := make([]uint16, 512)
		n := uint32(len(buf))
		err := syscall.RegEnumKeyEx(h, index, &buf[0], &n, nil, nil, nil, nil)
		if errors.Is(err, syscall.Errno(259)) { return names, nil } // ERROR_NO_MORE_ITEMS
		if err != nil { return nil, err }
		names = append(names, syscall.UTF16ToString(buf[:n]))
	}
}

// IsAdministrator checks actual TokenElevation rather than group membership.
func IsAdministrator() (bool, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil { return false, err }
	defer token.Close()
	var elevated uint32
	var returned uint32
	err = syscall.GetTokenInformation(token, syscall.TokenElevation,
		(*byte)(unsafe.Pointer(&elevated)), 4, &returned)
	return err == nil && elevated != 0, err
}
