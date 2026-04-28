package device

import "errors"

// ErrPermission is returned when the device cannot be opened due to
// permissions. The caller (Flash) decides whether to prompt for a chown.
var ErrPermission = errors.New("permission denied")

// ErrBusy is returned when O_EXCL refuses the open because partitions are
// currently mounted.
var ErrBusy = errors.New("device busy")
