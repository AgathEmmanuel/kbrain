#!/bin/bash
set -e

cd "${WORKSPACE:-/workspace/repo}"

# Configure git
git config user.email "kbrain-agent@kbrain.io"
git config user.name "kbrain-agent"

# If using ollama, wait for it to be ready
if [ -n "$OLLAMA_HOST" ]; then
    echo "Waiting for ollama at $OLLAMA_HOST..."
    until curl -sf "${OLLAMA_HOST}/api/tags" > /dev/null 2>&1; do
        sleep 2
    done
    echo "Ollama is ready."
fi

# Run claude-code with the task description
echo "Running claude-code agent..."
claude -p "$TASK_DESCRIPTION" \
    --model "$MODEL_NAME" \
    --allowedTools "Bash,Read,Write,Edit,Glob,Grep"

# Stage, commit, and push changes
git add -A
if git diff --cached --quiet; then
    echo "No changes to commit."
    exit 0
fi

git commit -m "feat: $(echo "$TASK_DESCRIPTION" | head -c 72)"
git push origin "$GIT_BRANCH"

echo "Changes pushed to branch $GIT_BRANCH"
