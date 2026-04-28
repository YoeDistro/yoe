//go:build !linux

package device

import (
	"fmt"
	"runtime"
)

// Write is a stub on non-Linux platforms. Flash already early-returns
// before reaching this, but the package must still compile.
func Write(imagePath, devicePath string, progress func(written, total int64)) error {
	return fmt.Errorf("flash write not supported on %s", runtime.GOOS)
}
