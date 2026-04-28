package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// ErrPermission is returned when the device cannot be opened due to
// permissions. The caller (Flash) decides whether to prompt for a chown.
var ErrPermission = errors.New("permission denied")

// ErrBusy is returned when O_EXCL refuses the open because partitions are
// currently mounted.
var ErrBusy = errors.New("device busy")

// Write copies imagePath to devicePath, calling progress periodically with
// (bytes written so far, total image bytes). Opens with O_EXCL so the
// kernel rejects writing to a disk with mounted partitions. Calls Sync
// before close.
func Write(imagePath, devicePath string, progress func(written, total int64)) error {
	src, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat image: %w", err)
	}
	total := info.Size()

	dst, err := os.OpenFile(devicePath, os.O_WRONLY|syscall.O_EXCL, 0)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return ErrPermission
		}
		if errors.Is(err, syscall.EBUSY) {
			return ErrBusy
		}
		return fmt.Errorf("open device: %w", err)
	}
	defer dst.Close()

	if err := copyWithProgress(dst, src, total, progress); err != nil {
		return err
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}

// copyWithProgress copies src to dst using a 4 MiB buffer. progress is
// called at most every 250ms or every 16 MiB, whichever comes first.
// A final call is always made when the copy completes.
func copyWithProgress(dst io.Writer, src io.Reader, total int64, progress func(written, total int64)) error {
	const (
		bufSize     = 4 * 1024 * 1024
		throttleByt = 16 * 1024 * 1024
		throttleDur = 250 * time.Millisecond
	)
	buf := make([]byte, bufSize)
	var written int64
	lastProgressBytes := int64(0)
	lastProgressTime := time.Now()

	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return fmt.Errorf("write: %w", werr)
			}
			written += int64(n)
			now := time.Now()
			if progress != nil &&
				(written-lastProgressBytes >= throttleByt ||
					now.Sub(lastProgressTime) >= throttleDur) {
				progress(written, total)
				lastProgressBytes = written
				lastProgressTime = now
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("read: %w", rerr)
		}
	}
	if progress != nil {
		progress(written, total)
	}
	return nil
}
