package device

import (
	"context"
	"fmt"
	"io"
)

// DeployInput parameterizes Deploy.
//
// Deploy itself is the post-build orchestration step: ssh to the target
// and run apk add. Building <unit> is the caller's job — the CLI runs
// `yoe build` ahead of this and starts/stops the feed.
type DeployInput struct {
	Target  SSHTarget
	Unit    string
	FeedURL string // already resolved (mDNS reuse or ephemeral)
	Out     io.Writer
	SSH     SSHRunner // defaults to DefaultSSH if nil
}

// Deploy writes /etc/apk/repositories.d/yoe-dev.list with FeedURL and
// runs `apk add --upgrade <Unit>` on the target. The repo file is left
// in place — that's the persistent feed config the spec requires.
func Deploy(ctx context.Context, in DeployInput) error {
	if in.Unit == "" {
		return fmt.Errorf("unit is empty")
	}
	if in.FeedURL == "" {
		return fmt.Errorf("feed URL is empty")
	}
	ssh := in.SSH
	if ssh == nil {
		ssh = DefaultSSH
	}
	if in.Out == nil {
		in.Out = io.Discard
	}

	script := fmt.Sprintf(`set -e
mkdir -p /etc/apk/repositories.d
printf '%%s\n' '%s' > /etc/apk/repositories.d/yoe-dev.list
apk update
apk add --upgrade %s
`, in.FeedURL, in.Unit)

	return ssh(ctx, in.Target, script, in.Out, in.Out)
}
