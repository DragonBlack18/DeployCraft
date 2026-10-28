//go:build !windows

package platform

import (
	"errors"
	"os"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

var errUnsupported = errors.New("Windows registry unavailable on this platform")

type registryAdapter struct{}
type pathsAdapter struct{}

func NewRegistry() domain.Registry { return registryAdapter{} }
func NewPaths() domain.Paths       { return pathsAdapter{} }

func (registryAdapter) ReadString(key, value string) (string, bool, error) {
	return "", false, errUnsupported
}
func (registryAdapter) ListSubKeys(key string) ([]string, error) {
	return nil, errUnsupported
}
func (pathsAdapter) ProgramData() string { return "" }
func (pathsAdapter) Temp() string        { return os.TempDir() }
func (pathsAdapter) Expand(s string) string {
	return os.Expand(s, func(k string) string { return os.Getenv(strings.TrimSpace(k)) })
}
func IsAdministrator() (bool, error) { return false, errUnsupported }
