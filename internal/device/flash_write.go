//go:build linux

package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// blockSize is the device-side I/O alignment O_DIRECT requires. 512
	// bytes is the universal floor for block devices on Linux.
	blockSize = 512

	// bufSize is the per-write transfer size. 4 MiB matches what
	// etcher-sdk uses; large enough for good throughput on USB and SD,
	// small enough that holding it doesn't matter.
	bufSize = 4 * 1024 * 1024

	progressByteThreshold = 16 * 1024 * 1024
	progressTimeThreshold = 250 * time.Millisecond
)

// Write copies imagePath to devicePath, calling progress periodically with
// (bytes written so far, total image bytes).
//
// The device is opened with:
//   - O_EXCL: kernel rejects writing to a disk with mounted partitions.
//   - O_DIRECT: writes bypass the page cache, so progress reflects
//     actual device throughput. Without this the kernel buffers up to
//     hundreds of MiB in RAM and the apparent "100%" can land long
//     before any of it has reached the device, hiding the real wait
//     inside Sync(). With O_DIRECT, write(2) blocks at device speed and
//     the progress bar tracks reality.
//
// Sync() is still called before close so the SCSI SYNCHRONIZE CACHE /
// ATA FLUSH CACHE command goes down to the device's internal cache.
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

	dst, err := os.OpenFile(devicePath, os.O_WRONLY|syscall.O_EXCL|syscall.O_DIRECT, 0)
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

	// Page-aligned via mmap. Go's allocator usually page-aligns
	// multi-MB allocations, but doesn't promise it; mmap does, and
	// O_DIRECT requires it.
	buf, err := unix.Mmap(-1, 0, bufSize,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		return fmt.Errorf("alloc aligned buffer: %w", err)
	}
	defer unix.Munmap(buf)

	if err := copyAlignedWithProgress(dst, src, buf, total, progress); err != nil {
		return err
	}

	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}

// copyAlignedWithProgress copies src to dst using buf, writing in chunks
// that are always a multiple of blockSize so an O_DIRECT fd accepts
// them. The trailing short read is zero-padded up to the next blockSize
// boundary; progress reports the real source bytes (not the padded
// amount). Padding writes zeros to sectors past the image — harmless
// on a block device, since those sectors are unused after partitioning.
//
// buf must be at least blockSize bytes and a multiple of blockSize. For
// O_DIRECT fds it must also be page-aligned in memory; callers obtain
// alignment via unix.Mmap.
func copyAlignedWithProgress(dst io.Writer, src io.Reader, buf []byte, total int64, progress func(written, total int64)) error {
	var written int64
	lastBytes := int64(0)
	lastTime := time.Now()

	for {
		n, rerr := io.ReadFull(src, buf)
		if n > 0 {
			padded := alignUp(n, blockSize)
			for i := n; i < padded; i++ {
				buf[i] = 0
			}
			if _, werr := dst.Write(buf[:padded]); werr != nil {
				return fmt.Errorf("write: %w", werr)
			}
			written += int64(n)

			now := time.Now()
			if progress != nil &&
				(written-lastBytes >= progressByteThreshold ||
					now.Sub(lastTime) >= progressTimeThreshold) {
				progress(written, total)
				lastBytes = written
				lastTime = now
			}
		}
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
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

func alignUp(n, align int) int {
	return ((n + align - 1) / align) * align
}
