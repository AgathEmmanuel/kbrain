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

// AgentPoolSpec defines the desired state of AgentPool.
type AgentPoolSpec struct {
	// MaxConcurrency is the maximum number of concurrent agent tasks.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	MaxConcurrency int32 `json:"maxConcurrency"`

	// DefaultModelProvider for tasks in this pool.
	// +kubebuilder:validation:Enum=ollama;cloud
	// +optional
	DefaultModelProvider string `json:"defaultModelProvider,omitempty"`

	// DefaultModelName for tasks in this pool.
	// +optional
	DefaultModelName string `json:"defaultModelName,omitempty"`

	// DefaultAgentType for tasks in this pool.
	// +kubebuilder:validation:Enum=claude-code;aider;codex
	// +optional
	DefaultAgentType string `json:"defaultAgentType,omitempty"`

	// Git holds the default repository configuration for this pool.
	Git GitSpec `json:"git"`

	// DefaultAPIKeySecretRef is the default API key secret for tasks in this pool.
	// +optional
	DefaultAPIKeySecretRef *SecretKeyRef `json:"defaultAPIKeySecretRef,omitempty"`

	// DefaultGitCredentialsSecretRef is the default git credentials secret for tasks in this pool.
	// +optional
	DefaultGitCredentialsSecretRef *SecretKeyRef `json:"defaultGitCredentialsSecretRef,omitempty"`

	// Selector matches AgentTasks belonging to this pool.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// QueueStrategy determines task ordering: fifo or priority.
	// +kubebuilder:validation:Enum=fifo;priority
	// +kubebuilder:default=fifo
	// +optional
	QueueStrategy string `json:"queueStrategy,omitempty"`
}

// AgentPoolStatus defines the observed state of AgentPool.
type AgentPoolStatus struct {
	// ActiveAgents is the number of currently running agents.
	ActiveAgents int32 `json:"activeAgents"`

	// QueuedTasks is the number of tasks waiting for a slot.
	QueuedTasks int32 `json:"queuedTasks"`

	// CompletedTasks is the total number of completed tasks.
	CompletedTasks int32 `json:"completedTasks"`

	// FailedTasks is the total number of failed tasks.
	FailedTasks int32 `json:"failedTasks"`

	// ActiveTaskNames lists the names of currently active tasks.
	// +optional
	ActiveTaskNames []string `json:"activeTaskNames,omitempty"`

	// Conditions provide detailed status information.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Active",type=integer,JSONPath=`.status.activeAgents`
// +kubebuilder:printcolumn:name="Queued",type=integer,JSONPath=`.status.queuedTasks`
// +kubebuilder:printcolumn:name="Completed",type=integer,JSONPath=`.status.completedTasks`
// +kubebuilder:printcolumn:name="Max",type=integer,JSONPath=`.spec.maxConcurrency`

// AgentPool is the Schema for the agentpools API.
type AgentPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentPoolSpec   `json:"spec,omitempty"`
	Status AgentPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentPoolList contains a list of AgentPool.
type AgentPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentPool{}, &AgentPoolList{})
}
