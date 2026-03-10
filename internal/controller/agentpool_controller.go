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
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agentsv1alpha1 "github.com/agath/kbrain/api/v1alpha1"
)

// AgentPoolReconciler reconciles a AgentPool object
type AgentPoolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agentpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agentpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agentpools/finalizers,verbs=update
// +kubebuilder:rbac:groups=agents.kbrain.io,resources=agenttasks,verbs=get;list;watch;update;patch

func (r *AgentPoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pool agentsv1alpha1.AgentPool
	if err := r.Get(ctx, req.NamespacedName, &pool); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// List all tasks that belong to this pool
	tasks, err := r.listPoolTasks(ctx, &pool)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Categorize tasks by phase
	var active, pending, completed, failed []agentsv1alpha1.AgentTask
	for _, task := range tasks {
		switch task.Status.Phase {
		case agentsv1alpha1.AgentTaskPhaseRunning, agentsv1alpha1.AgentTaskPhaseInitializing,
			agentsv1alpha1.AgentTaskPhasePushingBranch, agentsv1alpha1.AgentTaskPhaseCreatingMR:
			active = append(active, task)
		case "", agentsv1alpha1.AgentTaskPhasePending:
			pending = append(pending, task)
		case agentsv1alpha1.AgentTaskPhaseSucceeded:
			completed = append(completed, task)
		case agentsv1alpha1.AgentTaskPhaseFailed:
			failed = append(failed, task)
		}
	}

	// Grant slots to pending tasks if we have capacity
	slotsAvailable := int(pool.Spec.MaxConcurrency) - len(active)
	if slotsAvailable > 0 && len(pending) > 0 {
		// Sort pending tasks by creation time (FIFO)
		sort.Slice(pending, func(i, j int) bool {
			return pending[i].CreationTimestamp.Before(&pending[j].CreationTimestamp)
		})

		toGrant := slotsAvailable
		if toGrant > len(pending) {
			toGrant = len(pending)
		}

		for i := 0; i < toGrant; i++ {
			task := &pending[i]
			if task.Annotations == nil {
				task.Annotations = make(map[string]string)
			}
			task.Annotations["kbrain.io/pool-slot-granted"] = "true"
			if err := r.Update(ctx, task); err != nil {
				log.Error(err, "failed to grant pool slot", "task", task.Name)
				continue
			}
			log.Info("granted pool slot", "task", task.Name, "pool", pool.Name)
		}
	}

	// Update pool status
	activeNames := make([]string, 0, len(active))
	for _, t := range active {
		activeNames = append(activeNames, t.Name)
	}

	pool.Status.ActiveAgents = int32(len(active))
	pool.Status.QueuedTasks = int32(len(pending))
	pool.Status.CompletedTasks = int32(len(completed))
	pool.Status.FailedTasks = int32(len(failed))
	pool.Status.ActiveTaskNames = activeNames

	if err := r.Status().Update(ctx, &pool); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue periodically to check for new pending tasks
	if len(pending) > 0 {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

// listPoolTasks returns all AgentTasks belonging to this pool.
func (r *AgentPoolReconciler) listPoolTasks(ctx context.Context, pool *agentsv1alpha1.AgentPool) ([]agentsv1alpha1.AgentTask, error) {
	taskList := &agentsv1alpha1.AgentTaskList{}

	// Use label selector if defined on the pool
	var listOpts []client.ListOption
	listOpts = append(listOpts, client.InNamespace(pool.Namespace))

	if pool.Spec.Selector != nil {
		selector, err := labels.Parse(labels.Set(pool.Spec.Selector.MatchLabels).String())
		if err != nil {
			return nil, err
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	} else {
		// Fall back to matching by poolRef
		listOpts = append(listOpts, client.MatchingFields{"spec.poolRef": pool.Name})
	}

	if err := r.List(ctx, taskList, listOpts...); err != nil {
		return nil, err
	}

	// Filter by poolRef if using label selector (belt and suspenders)
	var result []agentsv1alpha1.AgentTask
	for _, task := range taskList.Items {
		if pool.Spec.Selector != nil || task.Spec.PoolRef == pool.Name {
			result = append(result, task)
		}
	}

	return result, nil
}

// mapTaskToPool maps an AgentTask event to the pool it belongs to.
func (r *AgentPoolReconciler) mapTaskToPool(ctx context.Context, obj client.Object) []reconcile.Request {
	task, ok := obj.(*agentsv1alpha1.AgentTask)
	if !ok {
		return nil
	}

	if task.Spec.PoolRef == "" {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name:      task.Spec.PoolRef,
				Namespace: task.Namespace,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentsv1alpha1.AgentPool{}).
		Watches(
			&agentsv1alpha1.AgentTask{},
			handler.EnqueueRequestsFromMapFunc(r.mapTaskToPool),
		).
		Named("agentpool").
		Complete(r)
}
