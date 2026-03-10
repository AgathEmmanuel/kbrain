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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentTaskSpec defines the desired state of AgentTask.
type AgentTaskSpec struct {
	// Description is the task/feature description or problem statement for the agent.
	Description string `json:"description"`

	// ModelProvider selects local open-source (ollama) or cloud API-based models.
	// +kubebuilder:validation:Enum=ollama;cloud
	ModelProvider string `json:"modelProvider"`

	// ModelName is the model to use (e.g. "deepseek-coder:33b", "claude-sonnet-4-20250514").
	ModelName string `json:"modelName"`

	// AgentType selects which coding agent to run.
	// +kubebuilder:validation:Enum=claude-code;aider;codex
	AgentType string `json:"agentType"`

	// Git holds repository and branch configuration.
	Git GitSpec `json:"git"`

	// ApprovalMode controls whether the MR is auto-merged or requires manual approval.
	// +kubebuilder:validation:Enum=auto-merge;manual
	// +kubebuilder:default=manual
	// +optional
	ApprovalMode string `json:"approvalMode,omitempty"`

	// APIKeySecretRef references a Secret containing the model provider API key.
	// +optional
	APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`

	// GitCredentialsSecretRef references a Secret containing git credentials (token, username).
	// +optional
	GitCredentialsSecretRef *SecretKeyRef `json:"gitCredentialsSecretRef,omitempty"`

	// Resources defines resource requirements for the agent pod.
	// +optional
	Resources *ResourceSpec `json:"resources,omitempty"`

	// OllamaConfig holds ollama-specific settings (sidecar image, GPU, etc.).
	// +optional
	OllamaConfig *OllamaConfig `json:"ollamaConfig,omitempty"`

	// Timeout for the agent task (e.g. "30m", "1h"). Defaults to "30m".
	// +kubebuilder:default="30m"
	// +optional
	Timeout string `json:"timeout,omitempty"`

	// PoolRef is an optional reference to an AgentPool this task belongs to.
	// +optional
	PoolRef string `json:"poolRef,omitempty"`
}

// GitSpec defines git repository and branch configuration.
type GitSpec struct {
	// RepoURL is the git repository URL (HTTPS or SSH).
	RepoURL string `json:"repoURL"`

	// BaseBranch is the source branch to base work on.
	BaseBranch string `json:"baseBranch"`

	// TargetBranch is the branch the MR/PR targets. Defaults to BaseBranch.
	// +optional
	TargetBranch string `json:"targetBranch,omitempty"`

	// WorkBranch is the branch name the agent will create. Auto-generated if empty.
	// +optional
	WorkBranch string `json:"workBranch,omitempty"`

	// Platform is the git hosting platform for MR/PR creation.
	// +kubebuilder:validation:Enum=github;gitlab;bitbucket
	// +kubebuilder:default=github
	// +optional
	Platform string `json:"platform,omitempty"`
}

// SecretKeyRef references a key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Name of the Secret.
	Name string `json:"name"`
	// Key within the Secret data. Defaults to the first key if empty.
	// +optional
	Key string `json:"key,omitempty"`
}

// ResourceSpec defines compute resource requirements.
type ResourceSpec struct {
	// CPURequest for the agent container (e.g. "500m").
	// +optional
	CPURequest string `json:"cpuRequest,omitempty"`
	// MemoryRequest for the agent container (e.g. "512Mi").
	// +optional
	MemoryRequest string `json:"memoryRequest,omitempty"`
	// GPULimit for the ollama sidecar (e.g. "1").
	// +optional
	GPULimit string `json:"gpuLimit,omitempty"`
}

// OllamaConfig holds configuration specific to the ollama sidecar.
type OllamaConfig struct {
	// Image for the ollama sidecar container.
	// +kubebuilder:default="ollama/ollama:latest"
	// +optional
	Image string `json:"image,omitempty"`
	// UseGPU enables GPU resource requests for ollama.
	// +optional
	UseGPU bool `json:"useGPU,omitempty"`
	// PullTimeout is how long to wait for model download (e.g. "10m").
	// +optional
	PullTimeout string `json:"pullTimeout,omitempty"`
}

// AgentTaskPhase represents the current lifecycle phase of an AgentTask.
type AgentTaskPhase string

const (
	AgentTaskPhasePending       AgentTaskPhase = "Pending"
	AgentTaskPhaseInitializing  AgentTaskPhase = "Initializing"
	AgentTaskPhaseRunning       AgentTaskPhase = "Running"
	AgentTaskPhasePushingBranch AgentTaskPhase = "PushingBranch"
	AgentTaskPhaseCreatingMR    AgentTaskPhase = "CreatingMR"
	AgentTaskPhaseSucceeded     AgentTaskPhase = "Succeeded"
	AgentTaskPhaseFailed        AgentTaskPhase = "Failed"
)

// AgentTaskStatus defines the observed state of AgentTask.
type AgentTaskStatus struct {
	// Phase is the current lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Initializing;Running;PushingBranch;CreatingMR;Succeeded;Failed
	// +optional
	Phase AgentTaskPhase `json:"phase,omitempty"`

	// PodName is the name of the Pod running the agent.
	// +optional
	PodName string `json:"podName,omitempty"`

	// BranchName is the git branch that was created.
	// +optional
	BranchName string `json:"branchName,omitempty"`

	// MergeRequestURL is the URL of the created MR/PR.
	// +optional
	MergeRequestURL string `json:"mergeRequestURL,omitempty"`

	// Message is a human-readable status message.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime is when the task started running.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// EndTime is when the task completed.
	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// ExitCode of the agent process.
	// +optional
	ExitCode *int32 `json:"exitCode,omitempty"`

	// OutputSummary is a summary of the agent's output.
	// +optional
	OutputSummary string `json:"outputSummary,omitempty"`

	// Conditions provide detailed status information.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Agent",type=string,JSONPath=`.spec.agentType`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AgentTask is the Schema for the agenttasks API.
type AgentTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentTaskSpec   `json:"spec,omitempty"`
	Status AgentTaskStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentTaskList contains a list of AgentTask.
type AgentTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentTask{}, &AgentTaskList{})
}
