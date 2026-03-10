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

var _ = Describe("AgentPool Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-agentpool"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		agentpool := &agentsv1alpha1.AgentPool{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind AgentPool")
			err := k8sClient.Get(ctx, typeNamespacedName, agentpool)
			if err != nil && errors.IsNotFound(err) {
				resource := &agentsv1alpha1.AgentPool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: agentsv1alpha1.AgentPoolSpec{
						MaxConcurrency:       3,
						DefaultModelProvider: "cloud",
						DefaultModelName:     "claude-sonnet-4-20250514",
						DefaultAgentType:     "claude-code",
						Git: agentsv1alpha1.GitSpec{
							RepoURL:    "https://github.com/test/repo.git",
							BaseBranch: "main",
							Platform:   "github",
						},
						QueueStrategy: "fifo",
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kbrain.io/pool": resourceName,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &agentsv1alpha1.AgentPool{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the specific resource instance AgentPool")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AgentPoolReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Pool status should be updated
			Expect(k8sClient.Get(ctx, typeNamespacedName, agentpool)).To(Succeed())
			Expect(agentpool.Status.ActiveAgents).To(Equal(int32(0)))
		})
	})
})
