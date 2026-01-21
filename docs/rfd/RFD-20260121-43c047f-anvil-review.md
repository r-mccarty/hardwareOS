# RFD: Anvil Review 43c047f

**Commit:** 43c047f2133b82f8165c7f365f4dbe6da5a19fe9
**Date:** 2026-01-21
**Author:** r-mccarty

## Summary

This commit fixes the anvil review workflow to pass the decoded prompt through to Codex by escaping the `PROMPT` variable in the remote script heredoc, preventing local expansion on the GitHub runner. The change ensures the remote script uses the prompt decoded from `PROMPT_B64` before piping it into `codex exec`.

**Files Changed:**
- `.github/workflows/anvil-review.yml` (+1 line, -1 line)

## Findings

### None

No bugs, regressions, or test gaps were identified in the diff.

## Recommended Actions

1. Confirm the workflow run passes a non-empty prompt to Codex by inspecting the job logs for the step that runs `codex exec`.
2. Consider adding a guard that errors when the decoded `PROMPT` is empty to make failures more explicit.

## References

- Diff: `git show 7f216490dce5d07ce4ef938223020f8e9165f066..43c047f2133b82f8165c7f365f4dbe6da5a19fe9`
- Workflow line: `.github/workflows/anvil-review.yml:101`
- Workflow run: https://github.com/r-mccarty/hardwareOS/actions/runs/21229466963
