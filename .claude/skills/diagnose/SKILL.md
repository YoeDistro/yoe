---
name: diagnose
description: >
  This skill should be used when the user asks to "diagnose a build failure",
  "debug a recipe", "fix a build", "why did the build fail", "/diagnose",
  or mentions a recipe that failed to build. Iteratively analyzes build logs,
  identifies root causes, applies fixes, and rebuilds until the recipe succeeds.
---

# Diagnose Build Failures

Analyze and fix recipe build failures through an iterative read-fix-rebuild
loop. This skill reads the build log, identifies the root cause, applies a fix
to the recipe or source, and rebuilds until the recipe succeeds.

## When to Use

- A recipe fails to build (`yoe build <recipe>` exits with error)
- The user asks to diagnose or debug a build failure
- The user says `/diagnose <recipe>` or `/diagnose` (most recent failure)

## Diagnosis Workflow

### Step 1: Identify the Failing Recipe

If the user specifies a recipe name, use that. If not, find the most recent
failure by checking build output or looking for recipes with no cache marker:

```
ls build/*/build.log -lt | head -5
```

### Step 2: Read the Build Log

The build log lives at `build/<recipe>/build.log`. Read the **end** of the log
first — the error is almost always in the last 100 lines:

```
Read build/<recipe>/build.log (last 100 lines)
```

If the error references earlier output (e.g., a missing header first used
hundreds of lines up), read more context as needed.

### Step 3: Read the Recipe

Load the recipe's `.star` file to understand what's being built, its
dependencies, build class, configure args, and any custom build steps:

```
Find and read layers/**/recipes/**/<recipe>.star
```

### Step 4: Identify the Root Cause

Common failure categories in order of likelihood:

1. **Missing dependency** — compiler error for a missing header or library.
   Check if the required package is in the recipe's `deps` list. Check if the
   dep is built and installed to `build/sysroot/`.
2. **Configure flag issue** — `./configure` or `cmake` can't find a feature or
   path. Check `configure_args` in the recipe and verify paths reference
   `/build/sysroot`.
3. **Source/patch conflict** — patch doesn't apply, or source version changed.
   Check `build/<recipe>/src/` for `.rej` files or git errors in the log.
4. **Toolchain mismatch** — wrong compiler flags, missing tools. Check the
   build environment and Dockerfile.
5. **Parallel build race** — intermittent failure in `make -j`. Look for
   "No rule to make target" or missing generated files. Retry with
   `make -j1` as a diagnostic step.

### Step 5: Apply the Fix

Based on the root cause, apply the appropriate fix:

- **Missing dep**: Add to the recipe's `deps` list in the `.star` file
- **Configure flag**: Adjust `configure_args` in the recipe
- **Patch conflict**: Update or remove the conflicting patch
- **Source issue**: Check if the source needs updating or the extraction failed

Always explain what was found and what the fix is before applying it.

### Step 6: Rebuild with --force

After applying the fix, rebuild the specific recipe:

```bash
yoe build --force <recipe>
```

Use `--force` (not `--clean`) to skip the cache but preserve the source tree.
Use `--clean` only if the source tree itself is corrupted or needs a fresh
extract.

### Step 7: Check the Result

Read the build output. If the build succeeds, report the fix. If it fails
again, go back to Step 2 with the new log — the next error may be different
(e.g., fixing a missing header reveals a missing library).

## Iteration Rules

- **Maximum 5 iterations** before stopping to reassess with the user. If a
  recipe fails 5 times with different errors, there may be a deeper issue
  (wrong source version, fundamentally incompatible configuration).
- **Never apply the same fix twice.** If an attempted fix didn't resolve the
  error, revert it and try a different approach.
- **Read the actual error, not just the exit code.** Build systems often print
  the real error hundreds of lines before the final "make: *** Error 1".
- **Check dependencies first.** Most build failures in this system are missing
  deps — a package needs a header or library that hasn't been built or isn't
  in the sysroot.

## Key Paths

| Path | Contents |
|------|----------|
| `build/<recipe>/build.log` | Full build output |
| `build/<recipe>/src/` | Extracted source tree |
| `build/<recipe>/destdir/` | Install staging directory |
| `build/sysroot/` | Shared sysroot (deps' headers/libs) |
| `layers/**/recipes/**/<recipe>.star` | Recipe definition |

## What NOT to Do

- Do not modify files in `build/sysroot/` directly — it's populated
  automatically from built packages.
- Do not modify source files in `build/<recipe>/src/` as a permanent fix —
  changes there are lost on rebuild. Instead, create a patch in the recipe.
- Do not skip the build log. Always read it before proposing a fix.
- Do not take shortcuts to make the build pass (e.g., disabling features,
  removing configure checks) without explaining the trade-off and getting
  user approval.
