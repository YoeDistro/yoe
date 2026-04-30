package device

import (
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

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
