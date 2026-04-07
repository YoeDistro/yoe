package build

import (
	"fmt"
	"path/filepath"

	"github.com/YoeDistro/yoe-ng/internal/artifact"
	"github.com/YoeDistro/yoe-ng/internal/repo"
	yoestar "github.com/YoeDistro/yoe-ng/internal/starlark"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// BuildContext carries unit and project metadata needed by packaging builtins.
// Stored as a thread-local on build threads.
type BuildContext struct {
	Unit       *yoestar.Unit
	Project    *yoestar.Project
	ProjectDir string
	BuildDir   string
	DestDir    string
	ScopeDir   string
}

const buildCtxKey = "yoe.buildctx"

// fnAPKCreate implements the apk_create() Starlark builtin.
// Creates an .apk from the unit's destdir and returns a struct with the path.
//
//	apk_create() -> struct(path)
func fnAPKCreate(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	bctx := thread.Local(buildCtxKey).(*BuildContext)

	apkPath, err := artifact.CreateAPK(
		bctx.Unit,
		bctx.DestDir,
		filepath.Join(bctx.BuildDir, "pkg"),
		bctx.ScopeDir,
	)
	if err != nil {
		return nil, fmt.Errorf("apk_create: %w", err)
	}

	return starlarkstruct.FromStringDict(starlark.String("apk"), starlark.StringDict{
		"path": starlark.String(apkPath),
	}), nil
}

// fnAPKPublish implements the apk_publish() Starlark builtin.
// Copies an .apk to the local repo and regenerates APKINDEX.
//
//	apk_publish(path) -> None
func fnAPKPublish(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var apkPath starlark.String
	if err := starlark.UnpackPositionalArgs("apk_publish", args, nil, 1, &apkPath); err != nil {
		return nil, err
	}

	bctx := thread.Local(buildCtxKey).(*BuildContext)
	repoDir := repo.RepoDir(bctx.Project, bctx.ProjectDir)

	if err := repo.Publish(string(apkPath), repoDir); err != nil {
		return nil, fmt.Errorf("apk_publish: %w", err)
	}

	return starlark.None, nil
}

// fnSysrootStage implements the sysroot_stage() Starlark builtin.
// Stages the unit's destdir for downstream units' per-unit sysroots.
//
//	sysroot_stage() -> None
func fnSysrootStage(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	bctx := thread.Local(buildCtxKey).(*BuildContext)

	if err := StageSysroot(bctx.DestDir, bctx.BuildDir); err != nil {
		return nil, fmt.Errorf("sysroot_stage: %w", err)
	}

	return starlark.None, nil
}
