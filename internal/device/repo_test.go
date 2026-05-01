package device

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type recordedSSH struct {
	target SSHTarget
	script string
}

func newSSHRecorder(out string, retErr error) (*[]recordedSSH, SSHRunner) {
	recs := &[]recordedSSH{}
	return recs, func(_ context.Context, target SSHTarget, script string, stdout, _ io.Writer) error {
		*recs = append(*recs, recordedSSH{target: target, script: script})
		fmt.Fprint(stdout, out)
		return retErr
	}
}

type recordedSCP struct {
	target SSHTarget
	src    string
	dst    string
}

func newSCPRecorder(retErr error) (*[]recordedSCP, SCPRunner) {
	recs := &[]recordedSCP{}
	return recs, func(_ context.Context, target SSHTarget, src, dst string, _, _ io.Writer) error {
		*recs = append(*recs, recordedSCP{target: target, src: src, dst: dst})
		return retErr
	}
}

func TestRepoAddWritesFile(t *testing.T) {
	sshRecs, ssh := newSSHRecorder("OK\n", nil)
	_, scp := newSCPRecorder(nil)

	ops := RepoOps{SSH: ssh, SCP: scp}
	target := SSHTarget{Host: "dev-pi.local", User: "root"}
	err := ops.Add(context.Background(), target, RepoAddInput{
		Name:    "yoe-dev",
		FeedURL: "http://laptop.local:8765/myproj",
		Out:     &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(*sshRecs) != 1 {
		t.Fatalf("expected 1 ssh call, got %d", len(*sshRecs))
	}
	got := (*sshRecs)[0].script
	want := []string{
		"mkdir -p /etc/apk/repositories.d",
		"http://laptop.local:8765/myproj",
		"/etc/apk/repositories.d/yoe-dev.list",
		"apk update",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("ssh script missing %q\n--- script ---\n%s", w, got)
		}
	}
}

func TestRepoAddPushesKey(t *testing.T) {
	_, ssh := newSSHRecorder("OK\n", nil)
	scpRecs, scp := newSCPRecorder(nil)
	ops := RepoOps{SSH: ssh, SCP: scp}
	target := SSHTarget{Host: "dev-pi.local"}
	err := ops.Add(context.Background(), target, RepoAddInput{
		Name:        "yoe-dev",
		FeedURL:     "http://laptop.local:8765/myproj",
		PushKeyFrom: "/keys/myproj.rsa.pub",
		PushKeyTo:   "/etc/apk/keys/myproj.rsa.pub",
		Out:         &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(*scpRecs) != 1 {
		t.Fatalf("expected 1 scp call, got %d", len(*scpRecs))
	}
	rec := (*scpRecs)[0]
	if rec.src != "/keys/myproj.rsa.pub" || rec.dst != "/etc/apk/keys/myproj.rsa.pub" {
		t.Errorf("scp src=%q dst=%q, want /keys/myproj.rsa.pub -> /etc/apk/keys/myproj.rsa.pub", rec.src, rec.dst)
	}
}

func TestRepoRemove(t *testing.T) {
	sshRecs, ssh := newSSHRecorder("", nil)
	ops := RepoOps{SSH: ssh}
	err := ops.Remove(context.Background(), SSHTarget{Host: "dev-pi"}, "yoe-dev", &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(*sshRecs) != 1 {
		t.Fatalf("expected 1 ssh call, got %d", len(*sshRecs))
	}
	if !strings.Contains((*sshRecs)[0].script, "rm -f /etc/apk/repositories.d/yoe-dev.list") {
		t.Errorf("script: %s", (*sshRecs)[0].script)
	}
}

func TestRepoListPropagatesError(t *testing.T) {
	_, ssh := newSSHRecorder("", errors.New("connection refused"))
	ops := RepoOps{SSH: ssh}
	err := ops.List(context.Background(), SSHTarget{Host: "dev-pi"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHTargetDestUserOverride(t *testing.T) {
	tt := SSHTarget{Host: "user@host.local", User: "root"}
	if got := tt.dest(); got != "root@host.local" {
		t.Errorf("dest = %q, want root@host.local", got)
	}
	tt = SSHTarget{Host: "host.local"}
	if got := tt.dest(); got != "host.local" {
		t.Errorf("dest = %q, want host.local", got)
	}
}

func TestParseSSHTarget(t *testing.T) {
	cases := []struct {
		spec, defaultUser string
		wantHost          string
		wantUser          string
		wantPort          int
		wantErr           bool
	}{
		{"localhost", "root", "localhost", "root", 0, false},
		{"localhost:2222", "root", "localhost", "root", 2222, false},
		{"pi@dev-pi.local", "root", "dev-pi.local", "pi", 0, false},
		{"pi@dev-pi.local:22", "root", "dev-pi.local", "pi", 22, false},
		{"[::1]:2222", "root", "::1", "root", 2222, false},
		{"localhost:abc", "root", "", "", 0, true},
	}
	for _, c := range cases {
		got, err := ParseSSHTarget(c.spec, c.defaultUser)
		if (err != nil) != c.wantErr {
			t.Errorf("Parse(%q): err=%v wantErr=%v", c.spec, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if got.Host != c.wantHost || got.User != c.wantUser || got.Port != c.wantPort {
			t.Errorf("Parse(%q) = %+v, want host=%q user=%q port=%d",
				c.spec, got, c.wantHost, c.wantUser, c.wantPort)
		}
	}
}
