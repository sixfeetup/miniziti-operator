#!/usr/bin/env bash

# Task discipline checks for /speckit.implement workflow.
#
# Enforces two guardrails:
# 1) If code changes exist, tasks.md must also be updated (checkbox progress tracked).
# 2) Before moving to the next task (--ready-next), working tree must be clean.

set -euo pipefail

READY_NEXT=false
JSON_MODE=false

for arg in "$@"; do
    case "$arg" in
        --ready-next)
            READY_NEXT=true
            ;;
        --json)
            JSON_MODE=true
            ;;
        --help|-h)
            cat <<'USAGE'
Usage: check-task-discipline.sh [--ready-next] [--json]

Options:
  --ready-next   Strict mode: require clean git working tree before next task.
  --json         Print machine-readable result.
  --help, -h     Show help.

Exit codes:
  0 = pass
  1 = fail
USAGE
            exit 0
            ;;
        *)
            echo "ERROR: Unknown option '$arg'" >&2
            exit 1
            ;;
    esac
done

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

eval "$(get_feature_paths)"

if [[ ! -f "$TASKS" ]]; then
    echo "ERROR: tasks.md not found: $TASKS" >&2
    exit 1
fi

if [[ "$HAS_GIT" != "true" ]]; then
    echo "WARN: Git repository not detected; skipping discipline checks" >&2
    exit 0
fi

tasks_rel="$TASKS"
if [[ "$tasks_rel" == "$REPO_ROOT"/* ]]; then
    tasks_rel="${tasks_rel#$REPO_ROOT/}"
fi

unstaged_files="$(git diff --name-only)"
staged_files="$(git diff --cached --name-only)"
changed_files="$(printf '%s\n%s\n' "$unstaged_files" "$staged_files" | awk 'NF' | sort -u)"

has_tasks_change=false
if [[ -n "$changed_files" ]] && echo "$changed_files" | grep -Fxq "$tasks_rel"; then
    has_tasks_change=true
fi

non_tasks_changes=""
if [[ -n "$changed_files" ]]; then
    non_tasks_changes="$(echo "$changed_files" | grep -Fvx "$tasks_rel" || true)"
fi

# Count open/completed tasks from current tasks.md.
incomplete_count="$(grep -Ec '^- \[ \]' "$TASKS" || true)"
completed_count="$(grep -Ec '^- \[[xX]\]' "$TASKS" || true)"

newly_completed_count=0
if git cat-file -e "HEAD:$tasks_rel" 2>/dev/null; then
    prev_completed_count="$(git show "HEAD:$tasks_rel" | grep -Ec '^- \[[xX]\]' || true)"
    if [[ "$completed_count" -gt "$prev_completed_count" ]]; then
        newly_completed_count=$((completed_count - prev_completed_count))
    fi
fi

errors=()

if [[ -n "$non_tasks_changes" && "$has_tasks_change" != "true" ]]; then
    errors+=("Code changes detected without tasks.md progress update. Mark completed tasks as [X].")
fi

if [[ "$READY_NEXT" == "true" ]]; then
    if [[ -n "$(git status --porcelain)" ]]; then
        errors+=("Working tree is not clean. Commit structural/behavioral changes before moving to next task.")
    fi
fi

if [[ "$JSON_MODE" == "true" ]]; then
    status="pass"
    if [[ ${#errors[@]} -gt 0 ]]; then
        status="fail"
    fi

    # JSON output intentionally minimal to keep script dependency-free.
    printf '{"status":"%s","feature_dir":"%s","tasks":"%s","completed":%d,"incomplete":%d,"newly_completed":%d,"tasks_changed":%s,"non_tasks_changes":%s}\n' \
        "$status" \
        "$FEATURE_DIR" \
        "$TASKS" \
        "$completed_count" \
        "$incomplete_count" \
        "$newly_completed_count" \
        "$has_tasks_change" \
        "$( [[ -n "$non_tasks_changes" ]] && echo true || echo false )"

    if [[ ${#errors[@]} -gt 0 ]]; then
        printf 'ERRORS:\n' >&2
        for err in "${errors[@]}"; do
            printf -- '- %s\n' "$err" >&2
        done
        exit 1
    fi

    exit 0
fi

if [[ ${#errors[@]} -gt 0 ]]; then
    echo "Task discipline check: FAIL" >&2
    echo "feature: $FEATURE_DIR" >&2
    echo "tasks:   $TASKS" >&2
    echo "completed=$completed_count incomplete=$incomplete_count newly_completed=$newly_completed_count" >&2
    for err in "${errors[@]}"; do
        echo "- $err" >&2
    done
    exit 1
fi

echo "Task discipline check: PASS"
echo "feature: $FEATURE_DIR"
echo "tasks:   $TASKS"
echo "completed=$completed_count incomplete=$incomplete_count newly_completed=$newly_completed_count"
