//go:build !windows

package platform

import "errors"

func WindowsBuild()(int,error){return 0,errors.New("Windows only")}
func AcquireInstallerLock()(func(),error){return nil,errors.New("Windows only")}
