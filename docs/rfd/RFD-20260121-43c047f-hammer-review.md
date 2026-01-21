# RFD: Hammer Review 43c047f

**Commit:** 43c047f2133b82f8165c7f365f4dbe6da5a19fe9
**Date:** 2026-01-21
**Author:** Hammer Review Bot

## Summary

This commit fixes a shell variable quoting bug in `.github/workflows/anvil-review.yml`. The change corrects how the `$PROMPT` variable is passed to the `codex exec` command inside a remotely-executed bash heredoc.

**Files Changed:**
- `.github/workflows/anvil-review.yml` (+1 line, -1 line)

**Diff:**
```diff
-          printf '%s' "$PROMPT" | codex exec --dangerously-bypass-approvals-and-sandbox -
+          printf '%s' "\$PROMPT" | codex exec --dangerously-bypass-approvals-and-sandbox -
```

## Findings

### Critical

None identified.

### High

None identified.

### Medium

None identified.

### Low

#### 1. Fix is Correct and Necessary

**Severity:** Informational (Positive)
**Location:** `.github/workflows/anvil-review.yml:104`

**Analysis:** The change from `"$PROMPT"` to `"\$PROMPT"` is correct. This is a heredoc without single-quote delimiter (`<<EOF` not `<<'EOF'`), which means:

- **Before:** `"$PROMPT"` would be expanded at heredoc construction time on the CI runner. At that point, `PROMPT` is not set (it's only defined later inside the remote script on line 103), so it would expand to an empty string. The `codex exec` command would receive no input.

- **After:** `"\$PROMPT"` escapes the `$`, so the literal string `$PROMPT` is preserved in the heredoc content. When the script executes remotely on the Sprite VM, the variable expansion happens correctly after `PROMPT` has been assigned its decoded value.

**Evidence:** Lines 102-104 show the intent:
```bash
PROMPT_B64="${PROMPT_B64}"         # Line 102: Expanded at heredoc creation (correct)
PROMPT="\$(printf '%s' \"\$PROMPT_B64\" | base64 -d -i)"  # Line 103: Escaped for remote execution
printf '%s' "\$PROMPT" | codex exec ...  # Line 104: Now correctly escaped
```

**Impact:** This fix resolves what was likely a silent failure where `codex exec` received an empty prompt.

---

#### 2. Consistency with PROMPT_B64 Line

**Severity:** Informational
**Location:** `.github/workflows/anvil-review.yml:102`

**Observation:** Line 102 uses unescaped `${PROMPT_B64}`:
```bash
PROMPT_B64="${PROMPT_B64}"
```

This is intentional and correct: the outer `PROMPT_B64` is set on line 82 in the CI environment, and this line passes its value into the remote script. The asymmetry between lines 102 and 103-104 is by design:
- Line 102: Pass value from CI to remote (expand now)
- Lines 103-104: Execute commands remotely (escape for later execution)

---

#### 3. Heredoc Indentation Issue Persists

**Severity:** Low (Pre-existing)
**Location:** `.github/workflows/anvil-review.yml:88-106`

**Analysis:** The heredoc uses `<<EOF` but the closing `EOF` delimiter has leading spaces (line 105). Standard bash requires the delimiter at column 1 for `<<EOF`. However, this is a pre-existing issue from the parent commit and not introduced by this change. Additionally, if the workflow has been running successfully, there may be YAML processing or other factors that handle this correctly.

**Mitigation:** Consider using `<<-EOF` with tabs or removing indentation in a future cleanup commit.

---

## Recommended Actions

1. **None Required:** The fix is correct and addresses the bug where `$PROMPT` was being expanded prematurely (to empty) instead of being passed to the remote script for later expansion.

2. **Verification:** Check the workflow run at https://github.com/r-mccarty/hardwareOS/actions/runs/21229466959 to confirm the fix works as expected.

3. **Future Improvement:** Consider adding a brief inline comment explaining the escaping strategy for maintainability, given the nested execution context (CI runner -> heredoc -> Sprite VM -> codex).

## References

- Diff: `git diff 7f21649..43c047f`
- Parent commit: 7f21649 (feat: add anvil codex review)
- Workflow run: https://github.com/r-mccarty/hardwareOS/actions/runs/21229466959
