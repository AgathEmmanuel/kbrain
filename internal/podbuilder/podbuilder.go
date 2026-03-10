/*
Copyright 2026 kbrain authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podbuilder

import (
	"fmt"

	agentsv1alpha1 "github.com/agath/kbrain/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodBuilder constructs a Pod spec for running an agent task.
type PodBuilder interface {
	Build(task *agentsv1alpha1.AgentTask) (*corev1.Pod, error)
}

// NewPodBuilder returns the appropriate builder based on model provider.
func NewPodBuilder(task *agentsv1alpha1.AgentTask) PodBuilder {
	switch task.Spec.ModelProvider {
	case "ollama":
		return &OllamaPodBuilder{}
	case "cloud":
		return &CloudPodBuilder{}
	default:
		return &CloudPodBuilder{}
	}
}

// agentImage returns the container image for the given agent type.
func agentImage(agentType string) string {
	switch agentType {
	case "claude-code":
		return "ghcr.io/agath/kbrain-agent-claude:latest"
	case "aider":
		return "ghcr.io/agath/kbrain-agent-aider:latest"
	case "codex":
		return "ghcr.io/agath/kbrain-agent-codex:latest"
	default:
		return "ghcr.io/agath/kbrain-agent-claude:latest"
	}
}

// WorkBranchFor returns the work branch name, generating one if not specified.
func WorkBranchFor(task *agentsv1alpha1.AgentTask) string {
	return workBranch(task)
}

// workBranch returns the work branch name, generating one if not specified.
func workBranch(task *agentsv1alpha1.AgentTask) string {
	if task.Spec.Git.WorkBranch != "" {
		return task.Spec.Git.WorkBranch
	}
	return fmt.Sprintf("kbrain/%s", task.Name)
}

// targetBranch returns the target branch, defaulting to base branch.
func targetBranch(task *agentsv1alpha1.AgentTask) string {
	if task.Spec.Git.TargetBranch != "" {
		return task.Spec.Git.TargetBranch
	}
	return task.Spec.Git.BaseBranch
}

// basePodMeta returns the common Pod metadata.
func basePodMeta(task *agentsv1alpha1.AgentTask) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      fmt.Sprintf("agent-%s", task.Name),
		Namespace: task.Namespace,
		Labels: map[string]string{
			"app.kubernetes.io/name":       "kbrain-agent",
			"app.kubernetes.io/instance":   task.Name,
			"app.kubernetes.io/managed-by": "kbrain",
			"kbrain.io/task":               task.Name,
			"kbrain.io/agent-type":         task.Spec.AgentType,
		},
	}
}

// gitCloneInitContainer returns the init container that clones the repo and creates the work branch.
func gitCloneInitContainer(task *agentsv1alpha1.AgentTask) corev1.Container {
	branch := workBranch(task)
	return corev1.Container{
		Name:    "git-clone",
		Image:   "alpine/git:latest",
		Command: []string{"/bin/sh", "-c"},
		Args: []string{fmt.Sprintf(`set -e
git clone --branch %s --single-branch %s /workspace/repo
cd /workspace/repo
git checkout -b %s
git config user.email "kbrain-agent@kbrain.io"
git config user.name "kbrain-agent"
`, task.Spec.Git.BaseBranch, task.Spec.Git.RepoURL, branch)},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
			{Name: "git-credentials", MountPath: "/etc/git-credentials", ReadOnly: true},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "GIT_TERMINAL_PROMPT",
				Value: "0",
			},
		},
	}
}

// commonAgentEnv returns environment variables shared by all agent containers.
func commonAgentEnv(task *agentsv1alpha1.AgentTask) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "TASK_DESCRIPTION", Value: task.Spec.Description},
		{Name: "MODEL_NAME", Value: task.Spec.ModelName},
		{Name: "GIT_BRANCH", Value: workBranch(task)},
		{Name: "TARGET_BRANCH", Value: targetBranch(task)},
		{Name: "GIT_REPO_URL", Value: task.Spec.Git.RepoURL},
		{Name: "AGENT_TYPE", Value: task.Spec.AgentType},
		{Name: "WORKSPACE", Value: "/workspace/repo"},
	}
}

// workspaceVolume returns the shared workspace emptyDir volume.
func workspaceVolume() corev1.Volume {
	return corev1.Volume{
		Name: "workspace",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// gitCredentialsVolume returns the volume sourced from a git credentials secret.
func gitCredentialsVolume(task *agentsv1alpha1.AgentTask) corev1.Volume {
	secretName := "git-credentials"
	if task.Spec.GitCredentialsSecretRef != nil {
		secretName = task.Spec.GitCredentialsSecretRef.Name
	}
	return corev1.Volume{
		Name: "git-credentials",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Optional:   boolPtr(true),
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}
