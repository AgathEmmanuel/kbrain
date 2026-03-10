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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// OllamaPodBuilder builds a Pod with an ollama sidecar for local model serving.
type OllamaPodBuilder struct{}

func (b *OllamaPodBuilder) Build(task *agentsv1alpha1.AgentTask) (*corev1.Pod, error) {
	ollamaImage := "ollama/ollama:latest"
	if task.Spec.OllamaConfig != nil && task.Spec.OllamaConfig.Image != "" {
		ollamaImage = task.Spec.OllamaConfig.Image
	}

	env := commonAgentEnv(task)
	env = append(env, corev1.EnvVar{
		Name:  "OLLAMA_HOST",
		Value: "http://localhost:11434",
	})

	// Ollama sidecar
	ollamaSidecar := corev1.Container{
		Name:  "ollama",
		Image: ollamaImage,
		Ports: []corev1.ContainerPort{
			{ContainerPort: 11434, Name: "ollama", Protocol: corev1.ProtocolTCP},
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/api/tags",
					Port: intstr.FromInt32(11434),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			FailureThreshold:    60, // up to 5 minutes for startup
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/api/tags",
					Port: intstr.FromInt32(11434),
				},
			},
			PeriodSeconds: 10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "ollama-cache", MountPath: "/root/.ollama"},
		},
	}

	// Add GPU resources if requested
	if task.Spec.OllamaConfig != nil && task.Spec.OllamaConfig.UseGPU {
		gpuLimit := "1"
		if task.Spec.Resources != nil && task.Spec.Resources.GPULimit != "" {
			gpuLimit = task.Spec.Resources.GPULimit
		}
		ollamaSidecar.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse(gpuLimit),
			},
		}
	}

	// Model pull init container — pulls the model before the agent starts
	modelPullInit := corev1.Container{
		Name:    "model-pull",
		Image:   "curlimages/curl:latest",
		Command: []string{"/bin/sh", "-c"},
		Args: []string{fmt.Sprintf(`echo "Waiting for ollama to be ready..."
until curl -sf http://localhost:11434/api/tags > /dev/null 2>&1; do
  sleep 2
done
echo "Pulling model %s..."
curl -sf -X POST http://localhost:11434/api/pull -d '{"name": "%s"}' | head -c 0
echo "Model pull complete."
`, task.Spec.ModelName, task.Spec.ModelName)},
	}

	// Agent container
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
				modelPullInit,
			},
			Containers: []corev1.Container{
				ollamaSidecar,
				agentContainer,
			},
			Volumes: []corev1.Volume{
				workspaceVolume(),
				gitCredentialsVolume(task),
				{
					Name: "ollama-cache",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	return pod, nil
}
