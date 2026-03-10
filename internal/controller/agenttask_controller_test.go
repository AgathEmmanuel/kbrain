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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentsv1alpha1 "github.com/agath/kbrain/api/v1alpha1"
)

var _ = Describe("AgentTask Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-agenttask"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		agenttask := &agentsv1alpha1.AgentTask{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AgentTask")
			err := k8sClient.Get(ctx, typeNamespacedName, agenttask)
			if err != nil && errors.IsNotFound(err) {
				resource := &agentsv1alpha1.AgentTask{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: agentsv1alpha1.AgentTaskSpec{
						Description:   "Fix a test bug",
						ModelProvider: "cloud",
						ModelName:     "claude-sonnet-4-20250514",
						AgentType:     "claude-code",
						Git: agentsv1alpha1.GitSpec{
							RepoURL:    "https://github.com/test/repo.git",
							BaseBranch: "main",
							Platform:   "github",
						},
						ApprovalMode: "manual",
						Timeout:      "30m",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &agentsv1alpha1.AgentTask{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				// Remove finalizer before deleting
				resource.Finalizers = nil
				_ = k8sClient.Update(ctx, resource)
				By("Cleanup the specific resource instance AgentTask")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AgentTaskReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// After reconciliation, the task should have a finalizer and be in Pending phase
			Expect(k8sClient.Get(ctx, typeNamespacedName, agenttask)).To(Succeed())
			Expect(agenttask.Finalizers).To(ContainElement(taskFinalizer))
		})

		It("should progress through the state machine after multiple reconciles", func() {
			controllerReconciler := &AgentTaskReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// Reconcile several times to let the state machine progress
			for i := 0; i < 5; i++ {
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())
			}

			// Task should have progressed past Pending and have a pod name set
			Expect(k8sClient.Get(ctx, typeNamespacedName, agenttask)).To(Succeed())
			Expect(agenttask.Status.Phase).To(BeElementOf(
				agentsv1alpha1.AgentTaskPhaseInitializing,
				agentsv1alpha1.AgentTaskPhaseRunning,
			))
			Expect(agenttask.Status.PodName).To(Equal("agent-test-agenttask"))
		})
	})
})
