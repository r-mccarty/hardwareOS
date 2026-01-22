# RFD: Forge Synthesis 43c047f

**Commit:** 43c047f
**Date:** 2026-01-22
**Sources:** hammer, anvil

## Executive Summary

This commit fixes a shell variable quoting bug in the anvil-review workflow where `$PROMPT` was being expanded prematurely on the CI runner instead of inside the remote script heredoc. Overall risk is low—no actionable findings exceed the 0.6 threshold.

## Actionable Findings (Score >= 0.6)

None. All findings scored below the 0.6 actionable threshold.

## Deferred Findings (Score < 0.6)

- **[LOW] Heredoc Indentation Issue Persists** (score: 0.41, hammer only)
  - Location: `.github/workflows/anvil-review.yml:88-106`
  - Summary: Potential indentation concerns in the heredoc block remain.

- **[INFO] Fix is Correct and Necessary** (score: 0.36, hammer only)
  - Location: `.github/workflows/anvil-review.yml:104`
  - Summary: Before the fix, `"$PROMPT"` would expand at heredoc construction time on the CI runner when `PROMPT` was not yet set (it's only defined inside the remote script). The escaped `\$PROMPT` correctly defers expansion to runtime on the remote host.

- **[INFO] Consistency with PROMPT_B64 Line** (score: 0.36, hammer only)
  - Location: `.github/workflows/anvil-review.yml:102`
  - Summary: The `PROMPT_B64="${PROMPT_B64}"` pattern on line 102 passes the base64-encoded prompt into the remote script, which is then decoded and used as `$PROMPT`.

## Cross-Repo Pattern Analysis

No recurring patterns detected across repositories.

## Recommended Actions

1. **Immediate:** None required—no critical or high severity findings.
2. **Near-term:** None required—no medium findings with recurrence.
3. **Backlog:** Consider reviewing heredoc indentation practices in workflow files for consistency (LOW priority).
