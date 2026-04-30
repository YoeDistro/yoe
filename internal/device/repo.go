package device

import (
	"context"
	"fmt"
	"io"
)

// RepoOps groups Add/Remove/List against a remote target. SSH and SCP are
// pluggable so tests can substitute stubs; production wiring uses
// DefaultSSH and DefaultSCP.
type RepoOps struct {
	SSH SSHRunner
	SCP SCPRunner
}

// RepoAddInput carries the parameters for Add.
type RepoAddInput struct {
	Name        string // basename for /etc/apk/repositories.d/<name>.list
	FeedURL     string
	PushKeyFrom string    // local path; empty = skip
	PushKeyTo   string    // remote path
	Out         io.Writer // streams ssh stdout/stderr
}

// Add writes the repo file on the target and runs apk update.
func (r RepoOps) Add(ctx context.Context, t SSHTarget, in RepoAddInput) error {
	if in.Name == "" {
		return fmt.Errorf("repo name is empty")
	}
	if in.FeedURL == "" {
		return fmt.Errorf("feed URL is empty")
	}
	if r.SSH == nil {
		return fmt.Errorf("SSH runner is nil")
	}
	if in.Out == nil {
		in.Out = io.Discard
	}

	if in.PushKeyFrom != "" {
		if r.SCP == nil {
			return fmt.Errorf("SCP runner is nil but key push requested")
		}
		if err := r.SCP(ctx, t, in.PushKeyFrom, in.PushKeyTo, in.Out, in.Out); err != nil {
			return fmt.Errorf("scp key %s -> %s: %w", in.PushKeyFrom, in.PushKeyTo, err)
		}
	}

	script := fmt.Sprintf(`set -e
mkdir -p /etc/apk/repositories.d
printf '%%s\n' '%s' > /etc/apk/repositories.d/%s.list
apk update
`, in.FeedURL, in.Name)

	return r.SSH(ctx, t, script, in.Out, in.Out)
}

// Remove deletes /etc/apk/repositories.d/<name>.list on the target.
func (r RepoOps) Remove(ctx context.Context, t SSHTarget, name string, out io.Writer) error {
	if name == "" {
		return fmt.Errorf("repo name is empty")
	}
	if r.SSH == nil {
		return fmt.Errorf("SSH runner is nil")
	}
	if out == nil {
		out = io.Discard
	}
	script := fmt.Sprintf("rm -f /etc/apk/repositories.d/%s.list\n", name)
	return r.SSH(ctx, t, script, out, out)
}

// List cats /etc/apk/repositories and the .list files in repositories.d/.
func (r RepoOps) List(ctx context.Context, t SSHTarget, stdout, stderr io.Writer) error {
	if r.SSH == nil {
		return fmt.Errorf("SSH runner is nil")
	}
	script := `set -e
for f in /etc/apk/repositories /etc/apk/repositories.d/*.list; do
    [ -e "$f" ] || continue
    while IFS= read -r line; do
        printf '%s: %s\n' "$f" "$line"
    done < "$f"
done
`
	return r.SSH(ctx, t, script, stdout, stderr)
}
