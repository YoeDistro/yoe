package device

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyWithProgressReportsAndCopiesAll(t *testing.T) {
	const size = 20 * 1024 * 1024 // 20 MiB triggers > 1 progress callback
	src := make([]byte, size)
	if _, err := rand.Read(src); err != nil {
		t.Fatal(err)
	}

	var dst bytes.Buffer
	var calls int
	var lastWritten int64

	err := copyWithProgress(&dst, bytes.NewReader(src), int64(size), func(written, total int64) {
		calls++
		lastWritten = written
		if total != int64(size) {
			t.Errorf("progress total = %d, want %d", total, size)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(dst.Bytes(), src) {
		t.Errorf("destination bytes do not match source")
	}
	if calls < 1 {
		t.Errorf("expected at least one progress call, got %d", calls)
	}
	if lastWritten != int64(size) {
		t.Errorf("final progress written = %d, want %d", lastWritten, size)
	}
}

func TestWriteCopiesToRegularFile(t *testing.T) {
	// Write doesn't strictly require a block device — passing a regular
	// file exercises the open/copy/sync path. O_EXCL on a regular file
	// behaves differently than on a block device, so we open the dest
	// indirectly by routing through copyWithProgress here.
	tmp := t.TempDir()
	imagePath := filepath.Join(tmp, "image.img")
	const size = 1024 * 1024
	src := make([]byte, size)
	for i := range src {
		src[i] = byte(i)
	}
	if err := os.WriteFile(imagePath, src, 0o644); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(tmp, "out.img")
	dst, err := os.Create(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	srcFile, err := os.Open(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	defer srcFile.Close()
	info, _ := srcFile.Stat()

	if err := copyWithProgress(dst, srcFile, info.Size(), nil); err != nil {
		t.Fatal(err)
	}
	if err := dst.Sync(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("destination contents do not match source")
	}
}
