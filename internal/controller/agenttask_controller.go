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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	agentsv1alpha1 "github.com/agath/kbrain/api/v1alpha1"
	"github.com/agath/kbrain/internal/gitops"
	"github.com/agath/kbrain/internal/podbuilder"
)

const (
	taskFinalizer = "agents.kbrain.io/finalizer"
)

// AgentTaskReconciler reconciles a AgentTask object
type AgentTaskReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	GitClient *gitops.Client
}

// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agenttasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agenttasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agenttasks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *AgentTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var task agentsv1alpha1.AgentTask
	if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !task.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &task)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&task, taskFinalizer) {
		controllerutil.AddFinalizer(&task, taskFinalizer)
		if err := r.Update(ctx, &task); err != nil {
			return ctrl.Result{}, err
		}
	}

	// State machine
	switch task.Status.Phase {
	case "", agentsv1alpha1.AgentTaskPhasePending:
		return r.handlePending(ctx, &task)
	case agentsv1alpha1.AgentTaskPhaseInitializing:
		return r.handleInitializing(ctx, &task)
	case agentsv1alpha1.AgentTaskPhaseRunning:
		return r.handleRunning(ctx, &task)
	case agentsv1alpha1.AgentTaskPhaseCreatingMR:
		return r.handleCreatingMR(ctx, &task)
	case agentsv1alpha1.AgentTaskPhaseSucceeded, agentsv1alpha1.AgentTaskPhaseFailed:
		return ctrl.Result{}, nil
	default:
		log.Info("unknown phase", "phase", task.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *AgentTaskReconciler) handlePending(ctx context.Context, task *agentsv1alpha1.AgentTask) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Validate required fields
	if task.Spec.Description == "" {
		return r.setFailed(ctx, task, "description is required")
	}
	if task.Spec.Git.RepoURL == "" {
		return r.setFailed(ctx, task, "git.repoURL is required")
	}
	if task.Spec.Git.BaseBranch == "" {
		return r.setFailed(ctx, task, "git.baseBranch is required")
	}

	// If part of a pool, check concurrency slot
	if task.Spec.PoolRef != "" {
		granted := task.Annotations["kbrain.io/pool-slot-granted"]
		if granted != "true" {
			log.Info("waiting for pool slot", "pool", task.Spec.PoolRef)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
	}

	// Validate secrets exist if referenced
	if task.Spec.ModelProvider == "cloud" && task.Spec.APIKeySecretRef != nil {
		secret := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      task.Spec.APIKeySecretRef.Name,
			Namespace: task.Namespace,
		}, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return r.setFailed(ctx, task, fmt.Sprintf("API key secret %q not found", task.Spec.APIKeySecretRef.Name))
			}
			return ctrl.Result{}, err
		}
	}

	// Transition to Initializing
	return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseInitializing, "Validated, creating agent pod")
}

func (r *AgentTaskReconciler) handleInitializing(ctx context.Context, task *agentsv1alpha1.AgentTask) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if pod already exists
	existingPod := &corev1.Pod{}
	podName := fmt.Sprintf("agent-%s", task.Name)
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: task.Namespace}, existingPod)
	if err == nil {
		// Pod exists, move to Running
		task.Status.PodName = podName
		return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseRunning, "Agent pod is running")
	}
	if !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Build the pod
	builder := podbuilder.NewPodBuilder(task)
	pod, err := builder.Build(task)
	if err != nil {
		return r.setFailed(ctx, task, fmt.Sprintf("failed to build pod spec: %v", err))
	}

	// Set owner reference so the pod is cleaned up with the task
	if err := controllerutil.SetControllerReference(task, pod, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("setting owner reference: %w", err)
	}

	log.Info("creating agent pod", "pod", pod.Name)
	if err := r.Create(ctx, pod); err != nil {
		if errors.IsAlreadyExists(err) {
			// Race condition, pod was created between our check and create
			task.Status.PodName = podName
			return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseRunning, "Agent pod is running")
		}
		return ctrl.Result{}, fmt.Errorf("creating pod: %w", err)
	}

	now := metav1.Now()
	task.Status.PodName = pod.Name
	task.Status.StartTime = &now
	task.Status.BranchName = podbuilder.WorkBranchFor(task)
	return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseRunning, "Agent pod created")
}

func (r *AgentTaskReconciler) handleRunning(ctx context.Context, task *agentsv1alpha1.AgentTask) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if task.Status.PodName == "" {
		return r.setFailed(ctx, task, "pod name not set in status")
	}

	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: task.Status.PodName, Namespace: task.Namespace}, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.setFailed(ctx, task, "agent pod was deleted")
		}
		return ctrl.Result{}, err
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		log.Info("agent pod succeeded, creating MR")
		return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseCreatingMR, "Agent completed, creating merge request")

	case corev1.PodFailed:
		exitCode := getExitCode(pod)
		task.Status.ExitCode = exitCode
		msg := "agent pod failed"
		if exitCode != nil {
			msg = fmt.Sprintf("agent pod failed with exit code %d", *exitCode)
		}
		return r.setFailed(ctx, task, msg)

	default:
		// Check timeout
		if task.Status.StartTime != nil {
			timeout, err := time.ParseDuration(task.Spec.Timeout)
			if err != nil {
				timeout = 30 * time.Minute
			}
			if time.Since(task.Status.StartTime.Time) > timeout {
				// Delete the pod and fail
				_ = r.Delete(ctx, pod)
				return r.setFailed(ctx, task, fmt.Sprintf("task timed out after %s", timeout))
			}
		}
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}
}

func (r *AgentTaskReconciler) handleCreatingMR(ctx context.Context, task *agentsv1alpha1.AgentTask) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Resolve git token from secret
	gitToken, err := r.resolveSecretValue(ctx, task.Namespace, task.Spec.GitCredentialsSecretRef)
	if err != nil {
		return r.setFailed(ctx, task, fmt.Sprintf("failed to resolve git credentials: %v", err))
	}

	if gitToken == "" {
		return r.setFailed(ctx, task, "git credentials secret not configured")
	}

	branchName := task.Status.BranchName
	if branchName == "" {
		branchName = podbuilder.WorkBranchFor(task)
	}

	targetBranch := task.Spec.Git.TargetBranch
	if targetBranch == "" {
		targetBranch = task.Spec.Git.BaseBranch
	}

	platform := task.Spec.Git.Platform
	if platform == "" {
		platform = "github"
	}

	opts := gitops.PROptions{
		Platform:     platform,
		RepoURL:      task.Spec.Git.RepoURL,
		SourceBranch: branchName,
		TargetBranch: targetBranch,
		Title:        fmt.Sprintf("[kbrain] %s", truncate(task.Spec.Description, 60)),
		Description:  fmt.Sprintf("## kbrain Agent Task\n\n**Task:** %s\n\n**Agent:** %s\n**Model:** %s\n\n---\n*Created by kbrain operator*", task.Spec.Description, task.Spec.AgentType, task.Spec.ModelName),
		Token:        gitToken,
		AutoMerge:    task.Spec.ApprovalMode == "auto-merge",
	}

	gitClient := r.GitClient
	if gitClient == nil {
		gitClient = gitops.NewClient()
	}

	result, err := gitClient.CreatePullRequest(ctx, opts)
	if err != nil {
		log.Error(err, "failed to create merge request")
		return r.setFailed(ctx, task, fmt.Sprintf("failed to create MR: %v", err))
	}

	log.Info("merge request created", "url", result.URL)
	task.Status.MergeRequestURL = result.URL
	now := metav1.Now()
	task.Status.EndTime = &now
	return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseSucceeded, fmt.Sprintf("MR created: %s", result.URL))
}

func (r *AgentTaskReconciler) handleDeletion(ctx context.Context, task *agentsv1alpha1.AgentTask) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Clean up the agent pod if it still exists
	if task.Status.PodName != "" {
		pod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{Name: task.Status.PodName, Namespace: task.Namespace}, pod)
		if err == nil {
			log.Info("deleting agent pod", "pod", task.Status.PodName)
			_ = r.Delete(ctx, pod)
		}
	}

	controllerutil.RemoveFinalizer(task, taskFinalizer)
	return ctrl.Result{}, r.Update(ctx, task)
}

// setPhase updates the task status phase and message.
func (r *AgentTaskReconciler) setPhase(ctx context.Context, task *agentsv1alpha1.AgentTask, phase agentsv1alpha1.AgentTaskPhase, message string) (ctrl.Result, error) {
	task.Status.Phase = phase
	task.Status.Message = message
	if err := r.Status().Update(ctx, task); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}
	// Requeue immediately for non-terminal phases to continue the state machine
	if phase != agentsv1alpha1.AgentTaskPhaseSucceeded && phase != agentsv1alpha1.AgentTaskPhaseFailed {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// setFailed transitions to Failed phase.
func (r *AgentTaskReconciler) setFailed(ctx context.Context, task *agentsv1alpha1.AgentTask, message string) (ctrl.Result, error) {
	now := metav1.Now()
	if task.Status.EndTime == nil {
		task.Status.EndTime = &now
	}
	return r.setPhase(ctx, task, agentsv1alpha1.AgentTaskPhaseFailed, message)
}

// resolveSecretValue reads a value from a Kubernetes secret.
func (r *AgentTaskReconciler) resolveSecretValue(ctx context.Context, namespace string, ref *agentsv1alpha1.SecretKeyRef) (string, error) {
	if ref == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, secret); err != nil {
		return "", err
	}

	key := ref.Key
	if key == "" {
		// Use the first key in the secret
		for k := range secret.Data {
			key = k
			break
		}
	}

	val, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", key, ref.Name)
	}

	return string(val), nil
}

// getExitCode extracts the exit code from a pod's container statuses.
func getExitCode(pod *corev1.Pod) *int32 {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == "agent" && cs.State.Terminated != nil {
			code := cs.State.Terminated.ExitCode
			return &code
		}
	}
	return nil
}

func truncate(s string, maxLen int) string {
	// Truncate to first line, then max length
	if idx := len(s); idx > 0 {
		for i, c := range s {
			if c == '\n' {
				s = s[:i]
				break
			}
		}
	}
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentsv1alpha1.AgentTask{}).
		Owns(&corev1.Pod{}).
		Named("agenttask").
		Complete(r)
}
