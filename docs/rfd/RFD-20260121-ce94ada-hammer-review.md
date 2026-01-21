# RFD: Hammer Review ce94ada

**Commit:** ce94adad0750004d36555d9c40758dfbc031012a
**Date:** 2026-01-21
**Author:** r-mccarty
**Reviewer:** hammer (Claude)

## Summary

This commit adds a stale git lock file cleanup mechanism to the anvil-review workflow. The fix removes `.git/index.lock` if present before running `git fetch`, preventing failures when a previous git operation was interrupted or terminated unexpectedly.

### Change Details

```diff
+          if [ -f ".git/index.lock" ]; then
+            rm -f .git/index.lock
+          fi
```

The 3-line addition is inserted at `.github/workflows/anvil-review.yml:95-97` in the remote script that runs on the sprite VM, between the `cd` into the repo directory and the `git fetch origin` command.

## Findings

### 1. Low: Inconsistency Between anvil-review and hammer-review Workflows

**Severity:** Low
**Location:** `.github/workflows/hammer-review.yml` (missing the fix)

The fix was only applied to `anvil-review.yml` but not to `hammer-review.yml`. Both workflows share nearly identical remote script logic and both could encounter the same stale lock file issue.

**Evidence:**
- `anvil-review.yml:95-97`: Contains the lock cleanup
- `hammer-review.yml`: Does not contain the lock cleanup (lines around the `git fetch` have no such check)

**Impact:** The hammer-review workflow remains vulnerable to the same failure mode this commit was intended to fix.

**Recommendation:** Apply the same fix to `hammer-review.yml` for consistency.

### 2. Info: Unconditional Lock Removal Without Warning

**Severity:** Informational
**Location:** `.github/workflows/anvil-review.yml:95-97`

The lock file is removed silently without logging. While this is acceptable for CI automation, adding a log message would improve debugging visibility when investigating workflow issues.

**Recommendation (optional):** Consider adding a brief log:
```bash
if [ -f ".git/index.lock" ]; then
  echo "Removing stale git lock file"
  rm -f .git/index.lock
fi
```

### 3. Info: No Test Coverage for Workflow Logic

**Severity:** Informational

GitHub Actions workflows are inherently difficult to test. This is a known limitation and not a critical issue for a simple defensive fix like this.

**Impact:** None for this specific change.

## Risk Assessment

| Risk | Level | Mitigation |
|------|-------|------------|
| Race condition if git is running | Very Low | Workflow runs in isolated environment; concurrent git operations unlikely |
| Data loss from lock removal | None | Lock files are ephemeral; removing them only affects in-progress operations |
| Inconsistent behavior across workflows | Low | Apply same fix to hammer-review.yml |

## Recommended Actions

1. **Apply fix to hammer-review.yml** - Add the same 3-line lock cleanup to maintain consistency between workflows.

2. **No further action required** - The fix itself is sound and addresses a legitimate CI reliability issue.

## Verdict

**LGTM with minor follow-up.** The fix is correct and appropriate. The only issue is that it should also be applied to the parallel `hammer-review.yml` workflow for consistency.
