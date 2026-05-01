package device

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// ParseSSHTarget parses a `[user@]host[:port]` spec, falling back to
// defaultUser when no `user@` prefix is given. IPv6 literals with ports
// must use the standard `[::1]:2222` form. Returned Port is 0 when none
// was specified — callers should treat that as "use ssh's default".
func ParseSSHTarget(spec, defaultUser string) (SSHTarget, error) {
	user := defaultUser
	hostPort := spec
	if u, hp, ok := strings.Cut(spec, "@"); ok {
		user = u
		hostPort = hp
	}
	host := hostPort
	port := 0
	// SplitHostPort handles "host:port" and "[ipv6]:port" but errors on
	// bare hostnames — distinguish the two via the error.
	if h, p, err := net.SplitHostPort(hostPort); err == nil {
		host = h
		n, err := strconv.Atoi(p)
		if err != nil {
			return SSHTarget{}, fmt.Errorf("ssh target %q: invalid port %q", spec, p)
		}
		port = n
	}
	return SSHTarget{Host: host, User: user, Port: port}, nil
}

// SSHTarget identifies a remote device for ssh/scp shellouts.
type SSHTarget struct {
	Host string // hostname, IP, or user@host
	User string // overrides any user@ prefix on Host; empty = none
	Port int    // 0 = default (22)
}

// sshArgs returns the leading flags for ssh invocations.
func (t SSHTarget) sshArgs() []string {
	var args []string
	if t.Port != 0 {
		args = append(args, "-p", strconv.Itoa(t.Port))
	}
	args = append(args, "-o", "BatchMode=no")
	return args
}

// scpArgs returns the leading flags for scp invocations.
func (t SSHTarget) scpArgs() []string {
	var args []string
	if t.Port != 0 {
		args = append(args, "-P", strconv.Itoa(t.Port))
	}
	return args
}

// dest returns the user@host string (preferring the explicit User field).
func (t SSHTarget) dest() string {
	if t.User == "" {
		return t.Host
	}
	host := t.Host
	if i := strings.Index(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	return t.User + "@" + host
}

// SSHRunner shells out to `ssh` for remote command execution. The factory
// is exposed so tests can substitute a stub.
type SSHRunner func(ctx context.Context, target SSHTarget, remoteScript string, stdout, stderr io.Writer) error

// SCPRunner shells out to `scp` for file transfer.
type SCPRunner func(ctx context.Context, target SSHTarget, src, dst string, stdout, stderr io.Writer) error

// DefaultSSH runs ssh from $PATH.
func DefaultSSH(ctx context.Context, target SSHTarget, remoteScript string, stdout, stderr io.Writer) error {
	args := target.sshArgs()
	args = append(args, target.dest(), remoteScript)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// DefaultSCP runs scp from $PATH.
func DefaultSCP(ctx context.Context, target SSHTarget, src, dst string, stdout, stderr io.Writer) error {
	args := target.scpArgs()
	args = append(args, src, target.dest()+":"+dst)
	cmd := exec.CommandContext(ctx, "scp", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
