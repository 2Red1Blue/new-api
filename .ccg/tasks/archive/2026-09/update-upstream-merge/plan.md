# Plan

1. Record the dirty worktree and refresh `upstream/main` with per-command proxy bypass.
2. Stash all pre-existing tracked and untracked work with a named, recoverable stash.
3. Merge `upstream/main` into `main`; split conflicts by independent backend, core-relay, and frontend ownership.
4. Run formatting, diff checks, focused and full test validation.
5. Commit the merge, restore the pre-existing stash, verify it builds, and archive the task record.
