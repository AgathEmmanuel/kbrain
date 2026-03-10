# kbrain

AI Agent Orchestration on Kubernetes.

A Kubernetes operator that spins up AI coding agents (Claude Code, Aider, Codex) as pods, gives them a feature description, and they clone the repo, create a worktree branch, implement the feature, push, and create a merge request.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                  │
│                                                      │
│  ┌──────────────┐     ┌──────────────────────────┐  │
│  │   kbrain      │     │  AgentTask Pod            │  │
│  │   Operator    │────▶│  ┌────────┐ ┌─────────┐  │  │
│  │              │     │  │ Agent  │ │ Ollama  │  │  │
│  └──────┬───────┘     │  │Container│ │(sidecar)│  │  │
│         │             │  └────────┘ └─────────┘  │  │
│         │             └──────────────────────────┘  │
│         │                                            │
│  ┌──────▼───────┐     ┌──────────────────────────┐  │
│  │  AgentPool   │     │  AgentTask Pod            │  │
│  │  (concurrency│────▶│  ┌────────┐              │  │
│  │   gating)    │     │  │ Agent  │  (cloud API) │  │
│  └──────────────┘     │  └────────┘              │  │
│                        └──────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

**State machine:**

```
Pending → Initializing → Running → CreatingMR → Succeeded
                                              → Failed
```

## CRDs

### AgentTask

Represents a single coding task assigned to an AI agent.

```yaml
apiVersion: agents.kbrain.io/v1alpha1
kind: AgentTask
metadata:
  name: fix-login-bug
spec:
  description: "Fix the login endpoint 500 error on emails with plus signs"
  modelProvider: cloud          # "cloud" or "ollama"
  modelName: claude-sonnet-4-20250514
  agentType: claude-code        # "claude-code", "aider", or "codex"
  git:
    repoURL: https://github.com/acme/backend.git
    baseBranch: main
    workBranch: fix/login-bug
    platform: github            # "github" or "gitlab"
  approvalMode: manual          # "manual" or "auto-merge"
  apiKeySecretRef:
    name: anthropic-api-key
    key: api-key
  gitCredentialsSecretRef:
    name: git-credentials
    key: token
  timeout: "45m"
```

### AgentPool

Manages concurrency for a group of AgentTasks working on the same project.

```yaml
apiVersion: agents.kbrain.io/v1alpha1
kind: AgentPool
metadata:
  name: backend-pool
spec:
  maxConcurrency: 5
  defaultModelProvider: cloud
  defaultModelName: claude-sonnet-4-20250514
  defaultAgentType: claude-code
  git:
    repoURL: https://github.com/acme/backend.git
    baseBranch: main
    platform: github
  selector:
    matchLabels:
      kbrain.io/pool: backend-pool
  queueStrategy: fifo
```

## Model Providers

### Cloud (`modelProvider: cloud`)

Uses cloud APIs (Anthropic, OpenAI). The agent pod runs with the API key injected from a Kubernetes Secret. No sidecar needed.

### Ollama (`modelProvider: ollama`)

Runs a local model via an Ollama sidecar container in the same pod. Supports GPU passthrough via `nvidia.com/gpu` resource requests.

```yaml
spec:
  modelProvider: ollama
  modelName: deepseek-coder-v2:latest
  ollamaConfig:
    image: ollama/ollama:latest
    useGPU: true
```

## Quick Start

### Prerequisites

- Kubernetes cluster (kind, minikube, or production)
- `kubectl` configured
- Secrets created for API keys and git credentials

### Install CRDs and deploy the operator

```bash
make install        # install CRDs
make deploy IMG=ghcr.io/agath/kbrain-operator:latest
```

### Create secrets

```bash
kubectl create namespace kbrain-system

kubectl create secret generic anthropic-api-key \
  --namespace kbrain-system \
  --from-literal=api-key=sk-ant-...

kubectl create secret generic git-credentials \
  --namespace kbrain-system \
  --from-literal=token=ghp_...
```

### Submit a task

```bash
kubectl apply -f config/samples/agents_v1alpha1_agenttask.yaml
```

### Watch progress

```bash
kubectl get agenttasks -w
# NAME            PHASE        AGENT        MODEL                      AGE
# fix-login-bug   Running      claude-code  claude-sonnet-4-20250514   2m

kubectl describe agenttask fix-login-bug
```

## Development

```bash
make generate    # generate deepcopy
make manifests   # generate CRDs + RBAC
make build       # compile the operator
make test        # run tests
make run         # run operator locally against current kubeconfig
```

### Build agent images

```bash
docker build -f docker/Dockerfile.agent-claude -t ghcr.io/agath/kbrain-agent-claude:latest .
docker build -f docker/Dockerfile.agent-aider -t ghcr.io/agath/kbrain-agent-aider:latest .
docker build -f docker/Dockerfile.agent-codex -t ghcr.io/agath/kbrain-agent-codex:latest .
docker build -f docker/Dockerfile.operator -t ghcr.io/agath/kbrain-operator:latest .
```

## Project Structure

```
api/v1alpha1/              CRD type definitions (AgentTask, AgentPool)
internal/controller/       Reconciliation controllers
internal/podbuilder/       Pod spec construction (cloud vs ollama modes)
internal/gitops/           PR/MR creation (GitHub, GitLab)
docker/                    Dockerfiles for operator and agent images
docker/entrypoints/        Entrypoint scripts for each agent type
config/crd/                Generated CRD manifests
config/rbac/               Generated RBAC rules
config/samples/            Example CRs
```

## License

Apache License 2.0
