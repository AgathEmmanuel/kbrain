# Contributing to kbrain

Thank you for your interest in contributing to kbrain. This document provides guidelines and information for contributors.

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.23+
- Docker or Podman
- kubectl
- A Kubernetes cluster (kind or minikube for local development)
- kubebuilder 4.4.0+ (for scaffolding changes)

### Development Setup

```bash
# Clone the repository
git clone https://github.com/agath/kbrain.git
cd kbrain

# Install dependencies
go mod download

# Generate code (deepcopy, CRDs, RBAC)
make generate
make manifests

# Build the operator
make build

# Run tests
make test

# Run the operator locally against your current kubeconfig
make run
```

### Running Against a Local Cluster

```bash
# Start a kind cluster
kind create cluster --name kbrain-dev

# Install CRDs
make install

# Run the operator locally
make run

# In another terminal, apply a sample task
kubectl apply -f config/samples/agents_v1alpha1_agenttask.yaml
```

## How to Contribute

### Reporting Bugs

Open an issue using the **Bug Report** template. Include:

- kbrain version (or commit SHA)
- Kubernetes version (`kubectl version`)
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs (`kubectl logs` from the operator and agent pods)

### Suggesting Features

Open an issue using the **Feature Request** template. Describe:

- The problem you're trying to solve
- Your proposed solution
- Alternatives you've considered

### Submitting Pull Requests

1. Fork the repository and create a branch from `main`.
2. If you've added code, add tests. We aim for meaningful coverage on controller logic.
3. Ensure `make test` passes.
4. Ensure `make lint` passes (if golangci-lint is installed).
5. Update documentation if your change affects user-facing behavior.
6. Write a clear PR description explaining **what** and **why**.

#### PR Checklist

- [ ] Code compiles (`make build`)
- [ ] Tests pass (`make test`)
- [ ] CRD manifests regenerated if types changed (`make manifests`)
- [ ] DeepCopy regenerated if types changed (`make generate`)
- [ ] Sample CRs updated if CRD fields changed
- [ ] ARCHITECTURE.md updated if system design changed

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add bitbucket PR creation support
fix: handle timeout parsing for durations > 24h
docs: update ARCHITECTURE.md with pool scheduling details
refactor: extract secret resolution into dedicated package
test: add integration tests for ollama pod builder
```

## Project Structure

```
api/v1alpha1/            CRD type definitions (AgentTask, AgentPool)
internal/controller/     Reconciliation controllers (state machines)
internal/podbuilder/     Pod spec construction (cloud mode, ollama mode)
internal/gitops/         PR/MR creation (GitHub, GitLab APIs)
docker/                  Dockerfiles for operator and agent images
docker/entrypoints/      Shell entrypoints for each agent type
config/crd/              Generated CRD manifests (do not edit by hand)
config/rbac/             Generated RBAC manifests (do not edit by hand)
config/samples/          Example CRs for testing
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed system design.

## Development Guidelines

### Adding a New Agent Type

1. Add the agent name to the `AgentType` enum in `api/v1alpha1/agenttask_types.go`.
2. Add the image mapping in `internal/podbuilder/podbuilder.go` (`agentImage` function).
3. Create `docker/Dockerfile.agent-<name>` and `docker/entrypoints/<name>-entrypoint.sh`.
4. Run `make generate && make manifests` to regenerate.
5. Add a sample CR in `config/samples/`.

### Adding a New Git Platform

1. Add the platform to the `Platform` enum in `api/v1alpha1/agenttask_types.go`.
2. Implement the creation method in `internal/gitops/gitops.go`.
3. Add the case to `CreatePullRequest` switch.
4. Run `make generate && make manifests`.

### Testing

- **Unit tests** for pod builder and gitops use standard Go testing.
- **Integration tests** for controllers use [envtest](https://book.kubebuilder.io/reference/envtest) (real API server, no kubelet).
- **E2E tests** use a Kind cluster (`make test-e2e`).

## Release Process

Releases follow [Semantic Versioning](https://semver.org/). Tags trigger the CI to build and push container images.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
