#!/bin/bash
set -e

cd "${WORKSPACE:-/workspace/repo}"

git config user.email "kbrain-agent@kbrain.io"
git config user.name "kbrain-agent"

echo "Running codex agent..."
codex --approval-mode full-auto \
    --model "$MODEL_NAME" \
    "$TASK_DESCRIPTION"

git add -A
if git diff --cached --quiet; then
    echo "No changes to commit."
    exit 0
fi

git commit -m "feat: $(echo "$TASK_DESCRIPTION" | head -c 72)"
git push origin "$GIT_BRANCH"

echo "Changes pushed to branch $GIT_BRANCH"
