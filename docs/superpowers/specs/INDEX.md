# Design Specs

Status of design specs and their corresponding implementation plans.

## Specs

| Spec                                   | Plan                                   | Status      |
| -------------------------------------- | -------------------------------------- | ----------- |
| Container as build worker (2026-03-27) | container-as-build-worker (2026-03-27) | Done        |
| Units-core module design (2026-03-26)  | yoe-ng implementation (2026-03-25)     | In progress |
| TUI redesign (2026-03-30)              | tui-redesign (2026-03-30)              | In progress |

## Plans

| Plan                                   | Status      | Notes                                                         |
| -------------------------------------- | ----------- | ------------------------------------------------------------- |
| yoe-ng implementation (2026-03-25)     | In progress | Core CLI, Starlark, build, image all working                  |
| QEMU x86 bootable image (2026-03-26)   | Done        | QEMU boot, flash, image assembly all working                  |
| Container as build worker (2026-03-27) | Done        | RunInContainer API, host CLI, container sandbox               |
| TUI redesign (2026-03-30)              | In progress | Unit list, build status, detail view working                  |
| Content-addressed cache                | Not started | Only basic hash markers exist                                 |
| Host image building with bwrap         | Not started | Image assembly still runs in container                        |
| Per-recipe containers                  | Not started | No per-unit container field yet                               |
| Per-unit sysroots                      | In progress | AssembleSysroot/StageSysroot implemented, untested end-to-end |
