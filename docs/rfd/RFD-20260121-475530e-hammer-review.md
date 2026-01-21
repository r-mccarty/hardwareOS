# RFD: Hammer Review 475530e

**Commit:** 475530e4c405088e89a9d84777685e11bc34db66
**Date:** 2026-01-21
**Author:** Hammer Review Bot

## Summary

This commit modifies `.github/workflows/hammer-review.yml` to align heredoc indentation within the Python and bash scripts. The change adds leading spaces to the content inside two heredoc blocks (Python `<<'PY'` and bash `<<EOF`).

**Files Changed:**
- `.github/workflows/hammer-review.yml` (lines 78-135)

## Findings

### Critical

#### 1. Python Heredoc Indentation Breaks Script Execution

**Severity:** Critical
**Location:** `.github/workflows/hammer-review.yml:78-111`

The Python heredoc now has indented content:

```yaml
PROMPT_B64="$(python - <<'PY'
          import base64
          import os
          ...
          PY
          )"
```

**Problem:** In bash heredocs using `<<'DELIMITER'` (without the `-` variant), the content and closing delimiter must start at column 1. When you indent the content and delimiter with spaces:

1. The heredoc delimiter `PY` is never recognized because bash looks for `PY` at the start of a line, not `          PY`
2. This causes a parse error: "here-document delimited by end-of-file (wanted `PY')"
3. The workflow will fail immediately when attempting to run the "Run hammer review" step

**Impact:** The hammer-review workflow is completely broken and will fail on every trigger.

**Fix:** Either:
- Revert to the original unindented format (recommended)
- Use `<<-'PY'` with tabs (not spaces) for indentation, which strips leading tabs

---

#### 2. Bash Heredoc Indentation Breaks Script Execution

**Severity:** Critical
**Location:** `.github/workflows/hammer-review.yml:117-135`

The bash heredoc (`<<EOF`) has the same issue:

```yaml
REMOTE_SCRIPT=$(cat <<EOF
          set -euo pipefail
          cd /home/sprite/workspace
          ...
          EOF
          )
```

**Problem:** Same as above - the `EOF` delimiter will not be recognized when indented with spaces.

**Impact:** Even if the Python heredoc issue were fixed, this would still cause the workflow to fail.

**Fix:** Same as above - revert to unindented format or use `<<-EOF` with tabs.

---

### Low

#### 3. Inconsistent Indentation Style

**Severity:** Low
**Location:** `.github/workflows/hammer-review.yml`

The change attempts to align heredoc content with surrounding YAML indentation for visual consistency, but this approach doesn't work with bash heredoc syntax. This suggests a misunderstanding of heredoc behavior.

**Note:** This is a stylistic observation; the critical issue is the broken functionality above.

---

## Recommended Actions

1. **Immediate:** Revert this commit or push a fix that removes the leading spaces from both heredoc blocks:
   - Python heredoc (lines 79-110): Remove 10-space indentation from all lines including the `PY` delimiter
   - Bash heredoc (lines 118-134): Remove 10-space indentation from all lines including the `EOF` delimiter

2. **Verification:** After fixing, manually trigger the workflow via `workflow_dispatch` to verify it runs successfully.

3. **Prevention:** Consider adding a CI linting step (e.g., `shellcheck`) to catch heredoc syntax issues before merge.

## References

- Diff: `git diff bace908..475530e`
- Bash heredoc documentation: https://www.gnu.org/software/bash/manual/html_node/Redirections.html#Here-Documents
