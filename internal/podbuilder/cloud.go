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
	agentsv1alpha1 "github.com/agath/kbrain/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// CloudPodBuilder builds a Pod for cloud-based model providers (Anthropic, OpenAI, etc.).
type CloudPodBuilder struct{}

func (b *CloudPodBuilder) Build(task *agentsv1alpha1.AgentTask) (*corev1.Pod, error) {
	env := commonAgentEnv(task)
	env = append(env, b.apiKeyEnvVars(task)...)

	agentContainer := corev1.Container{
		Name:    "agent",
		Image:   agentImage(task.Spec.AgentType),
		Command: []string{"/entrypoint.sh"},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "workspace", MountPath: "/workspace"},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: basePodMeta(task),
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			AutomountServiceAccountToken: boolPtr(false),
			InitContainers: []corev1.Container{
				gitCloneInitContainer(task),
			},
			Containers: []corev1.Container{agentContainer},
			Volumes: []corev1.Volume{
				workspaceVolume(),
				gitCredentialsVolume(task),
			},
		},
	}

	return pod, nil
}

// apiKeyEnvVars injects the API key from the referenced secret.
func (b *CloudPodBuilder) apiKeyEnvVars(task *agentsv1alpha1.AgentTask) []corev1.EnvVar {
	if task.Spec.APIKeySecretRef == nil {
		return nil
	}

	secretName := task.Spec.APIKeySecretRef.Name
	secretKey := task.Spec.APIKeySecretRef.Key
	if secretKey == "" {
		secretKey = "api-key"
	}

	// Map the secret to the appropriate env var based on agent type
	envVarName := "ANTHROPIC_API_KEY"
	switch task.Spec.AgentType {
	case "codex":
		envVarName = "OPENAI_API_KEY"
	case "aider":
		// aider supports multiple providers; use ANTHROPIC by default
		envVarName = "ANTHROPIC_API_KEY"
	}

	return []corev1.EnvVar{
		{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  secretKey,
				},
			},
		},
	}
}
