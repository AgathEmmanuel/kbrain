#!/bin/bash
set -e

cd "${WORKSPACE:-/workspace/repo}"

git config user.email "kbrain-agent@kbrain.io"
git config user.name "kbrain-agent"

# If using ollama, wait for it and set the model URL
if [ -n "$OLLAMA_HOST" ]; then
    echo "Waiting for ollama at $OLLAMA_HOST..."
    until curl -sf "${OLLAMA_HOST}/api/tags" > /dev/null 2>&1; do
        sleep 2
    done
    echo "Ollama is ready."
    AIDER_MODEL="ollama/${MODEL_NAME}"
else
    AIDER_MODEL="$MODEL_NAME"
fi

echo "Running aider agent..."
aider --yes-always \
    --no-auto-commits \
    --model "$AIDER_MODEL" \
    --message "$TASK_DESCRIPTION"

git add -A
if git diff --cached --quiet; then
    echo "No changes to commit."
    exit 0
fi

git commit -m "feat: $(echo "$TASK_DESCRIPTION" | head -c 72)"
git push origin "$GIT_BRANCH"

echo "Changes pushed to branch $GIT_BRANCH"
