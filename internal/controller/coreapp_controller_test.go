/*
Copyright 2026 Juandi.

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

	appsv1 "k8s.io/api/apps/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	platformv1alpha1 "github.com/juandcsoler/k8s-app-factory/api/v1alpha1"
)

var _ = Describe("CoreApp Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind CoreApp")
			// 1. Damos datos válidos al Spec para que el Reconcile no falle
			resource := &platformv1alpha1.CoreApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.CoreAppSpec{
					Image: "nginx:latest",
					Port:  8080,
					// AÑADIMOS ESTO PARA QUE PASE LA VALIDACIÓN
					Autoscaling: platformv1alpha1.Autoscaling{ // Ajusta el nombre del Struct si el tuyo es distinto
						MaxReplicas: 1,
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance CoreApp")
			resource := &platformv1alpha1.CoreApp{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("debería crear el Deployment y marcar Ready=True", func() {
			// REGLA DE ORO DEL TUTORIAL: Usar Eventually para asincronía

			By("Comprobando que el operador ha creado el Deployment con la imagen correcta")
			Eventually(func(g Gomega) {
				var dep appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:latest"))
			}, "10s", "250ms").Should(Succeed())

			By("Comprobando que el operador ha actualizado el Status a Ready=True")
			Eventually(func(g Gomega) {
				var got platformv1alpha1.CoreApp
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &got)).To(Succeed())

				cond := apimeta.FindStatusCondition(got.Status.Conditions, "Ready")
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}, "10s", "250ms").Should(Succeed())
		})
	})
})
