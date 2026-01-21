# RFD: Hammer Review 703643e

**Commit:** 703643e037d556ff3b3ee28a36d5b9ef1761b80e
**Date:** 2026-01-21
**Author:** Hammer Review Bot

## Summary

This commit significantly restructures the "Run hammer review" step in `.github/workflows/hammer-review.yml`. The previous implementation (commit 70799be) had a broken Python heredoc structure and invoked `claude` directly on the runner. This commit replaces that with a proper remote execution model using `sprite exec`, where the workflow:

1. Fetches the prompt via `curl` and base64-encodes it
2. Authenticates with the Sprite API
3. Executes a bash script remotely on a Sprite VM that clones the repo, checks out the commit, and runs `claude`

**Files Changed:**
- `.github/workflows/hammer-review.yml` (+24 lines, -2 lines)

## Findings

### Medium

#### 1. Heredoc Indentation Issue Remains

**Severity:** Medium
**Location:** `.github/workflows/hammer-review.yml:86-104`

The bash heredoc (`REMOTE_SCRIPT`) has indented content:

```yaml
REMOTE_SCRIPT=$(cat <<EOF
          set -euo pipefail
          cd /home/sprite/workspace
          ...
          EOF
          )
```

**Problem:** With `<<EOF` (not `<<-EOF`), the closing delimiter must appear at column 1 without leading spaces. The indented `EOF` (preceded by spaces) will not be recognized as the heredoc terminator. This will cause a parse error.

**Evidence:** Line 103 shows `EOF` with leading spaces, and line 104 shows `)` also indented. The shell expects `EOF` to be the first characters on a line.

**Impact:** The "Run hammer review" step will fail with a heredoc parse error. However, this requires verification against actual CI logs.

**Mitigation:** Either:
- Remove leading spaces from all heredoc content and the `EOF` delimiter
- Use `<<-EOF` with tabs (not spaces) for indentation

---

#### 2. Missing Remote Script Invocation Context

**Severity:** Medium
**Location:** `.github/workflows/hammer-review.yml:100-102`

The variable assignment and expansion inside the heredoc may have quoting issues:

```bash
PROMPT_B64="${PROMPT_B64}"
PROMPT="\$(printf '%s' \"\$PROMPT_B64\" | base64 -d -i)"
```

**Analysis:** The `${PROMPT_B64}` on line 100 is expanded at heredoc construction time (since it's inside `<<EOF`, not `<<'EOF'`). This is the intended behavior. However, if `PROMPT_B64` contains special characters (newlines, quotes, backticks), they could break the script.

The `tr -d '\n'` on line 80 strips newlines from the base64 output, which mitigates one source of issues, but base64 output could still contain `+`, `/`, and `=` characters. These should be safe in a double-quoted string context.

**Risk:** Low-to-medium depending on the base64 content. The current implementation should work for standard base64 output.

---

### Low

#### 3. AGENT_HARNESS_TOKEN May Be Empty

**Severity:** Low
**Location:** `.github/workflows/hammer-review.yml:65, 79`

Line 65 sets `AGENT_HARNESS_TOKEN`:
```yaml
echo "AGENT_HARNESS_TOKEN=${GITHUB_TOKEN}" >> "$GITHUB_ENV"
```

Line 79 uses it:
```bash
AUTH_HEADER="Authorization: token ${AGENT_HARNESS_TOKEN}"
```

**Analysis:** The `GITHUB_TOKEN` is a built-in secret in GitHub Actions and should always be available. However, if the previous step fails before writing to `GITHUB_ENV`, `AGENT_HARNESS_TOKEN` would be unset. The `curl -fsSL` will then send `Authorization: token ` (empty token), which would fail with a 401 if the endpoint requires authentication.

**Mitigation:** Add validation: `if [ -z "$AGENT_HARNESS_TOKEN" ]; then echo "Missing AGENT_HARNESS_TOKEN"; exit 1; fi`

---

#### 4. Repository Clone URL Uses HTTPS Without Credentials

**Severity:** Low
**Location:** `.github/workflows/hammer-review.yml:90`

```bash
git clone https://github.com/${REPO_FULL}.git "${REPO_NAME}"
```

**Analysis:** This works for public repositories. If the repository is private, this clone will fail because no credentials are passed to the Sprite VM. The `GITHUB_TOKEN` is not forwarded to the remote script.

**Impact:** For this specific repository (hardwareOS), if it's public, no issue. If private, the clone will fail.

---

#### 5. No Tests for Workflow Syntax

**Severity:** Low
**Location:** N/A

**Analysis:** There are no automated tests or linting for the GitHub Actions workflow syntax. A tool like `actionlint` would catch heredoc issues and other YAML/workflow syntax problems before merge.

---

## Recommended Actions

1. **Immediate:** Verify the workflow execution by checking the CI logs for the current run (https://github.com/r-mccarty/hardwareOS/actions/runs/21211436357). If it fails with a heredoc parse error, fix the indentation:
   - Remove leading spaces from lines 87-103
   - Ensure `EOF` appears at column 1 on its own line
   - Alternatively, use `<<-EOF` with tab characters

2. **Validation:** Add a check for `AGENT_HARNESS_TOKEN` before using it in the curl command.

3. **Prevention:** Consider adding `actionlint` to CI to validate workflow syntax:
   ```yaml
   - name: Lint workflows
     run: |
       brew install actionlint
       actionlint
   ```

4. **Documentation:** Add a comment explaining the remote execution model for future maintainers.

## References

- Diff: `git diff 70799be..703643e`
- Workflow run: https://github.com/r-mccarty/hardwareOS/actions/runs/21211436357
- Bash heredoc documentation: https://www.gnu.org/software/bash/manual/html_node/Redirections.html#Here-Documents
