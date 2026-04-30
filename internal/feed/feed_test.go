package feed

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerServesRepoFiles(t *testing.T) {
	repoDir := t.TempDir()
	archDir := filepath.Join(repoDir, "myproj", "x86_64")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archDir, "APKINDEX.tar.gz"), []byte("fake-index"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := StartHTTP(repoDir, "127.0.0.1:0", io.Discard)
	if err != nil {
		t.Fatalf("StartHTTP: %v", err)
	}
	defer srv.Stop()

	url := "http://" + srv.Addr() + "/myproj/x86_64/APKINDEX.tar.gz"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "fake-index") {
		t.Fatalf("body = %q, want to contain fake-index", body)
	}
}

func TestServerStop(t *testing.T) {
	repoDir := t.TempDir()
	srv, err := StartHTTP(repoDir, "127.0.0.1:0", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestMDNSAdvertiseAndBrowse(t *testing.T) {
	if testing.Short() {
		t.Skip("mDNS test requires loopback multicast")
	}

	adv, err := AdvertiseMDNS(MDNSConfig{
		Instance: "yoe-test-feed",
		Project:  "test-project",
		Path:     "/test-project",
		Archs:    []string{"x86_64"},
		Port:     8765,
	})
	if err != nil {
		t.Fatalf("AdvertiseMDNS: %v", err)
	}
	defer adv.Stop()

	time.Sleep(200 * time.Millisecond)

	results, err := BrowseMDNS(2 * time.Second)
	if err != nil {
		t.Fatalf("BrowseMDNS: %v", err)
	}

	var found *MDNSResult
	for i, r := range results {
		if r.Instance == "yoe-test-feed" {
			found = &results[i]
			break
		}
	}
	if found == nil {
		t.Skipf("did not discover yoe-test-feed (multicast may be disabled); got %d results", len(results))
	}
	if found.Project != "test-project" {
		t.Errorf("Project = %q, want test-project", found.Project)
	}
	if found.Path != "/test-project" {
		t.Errorf("Path = %q, want /test-project", found.Path)
	}
	if found.Port != 8765 {
		t.Errorf("Port = %d, want 8765", found.Port)
	}
}

func TestServerWithMDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires loopback multicast")
	}
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "p", "x86_64"), 0o755); err != nil {
		t.Fatal(err)
	}

	srv, err := Start(Config{
		RepoDir:  repoDir,
		BindAddr: "127.0.0.1:0",
		Project:  "p",
		Archs:    []string{"x86_64"},
		Instance: "yoe-p-test",
		LogW:     io.Discard,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	if !strings.HasPrefix(srv.URL(), "http://") {
		t.Errorf("URL = %q, expected http:// prefix", srv.URL())
	}
}

func TestServerNoMDNS(t *testing.T) {
	repoDir := t.TempDir()
	srv, err := Start(Config{
		RepoDir:  repoDir,
		BindAddr: "127.0.0.1:0",
		Project:  "p",
		Archs:    []string{"x86_64"},
		Instance: "",
		NoMDNS:   true,
		LogW:     io.Discard,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	if srv.URL() == "" {
		t.Error("URL is empty")
	}
}
