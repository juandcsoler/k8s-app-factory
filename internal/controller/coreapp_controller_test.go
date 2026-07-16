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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	platformv1alpha1 "github.com/juandcsoler/platform-k8s-core-operator/api/v1alpha1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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
			// 1. Provide valid data to the Spec so Reconcile does not fail
			resource := &platformv1alpha1.CoreApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.CoreAppSpec{
					Image: "nginx:latest",
					Port:  8080,
					// ADD THIS SO IT PASSES VALIDATION
					Autoscaling: platformv1alpha1.Autoscaling{ // Adjust the Struct name if yours is different
						MaxReplicas: 1,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
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

		It("should create the Deployment and mark Ready=True", func() {
			// GOLDEN RULE FOR TUTORIAL: Use Eventually for async operations

			By("Checking that the operator created the Deployment with the correct image and resilience improvements")
			var dep appsv1.Deployment
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:latest"))

				// Validate resilience features
				g.Expect(dep.Spec.Template.Spec.Containers[0].Lifecycle).NotTo(BeNil())
				g.Expect(dep.Spec.Template.Spec.Containers[0].Lifecycle.PreStop).NotTo(BeNil())
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints).NotTo(BeEmpty())
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints[0].TopologyKey).To(Equal("kubernetes.io/hostname"))
			}, "10s", "250ms").Should(Succeed())

			By("Checking that the operator created the default NetworkPolicy")
			var netpol networkingv1.NetworkPolicy
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &netpol)).To(Succeed())
				g.Expect(netpol.Spec.PolicyTypes).To(ContainElement(networkingv1.PolicyTypeIngress))
				g.Expect(netpol.Spec.PolicyTypes).To(ContainElement(networkingv1.PolicyTypeEgress))
			}, "10s", "250ms").Should(Succeed())

			By("Checking that the operator created the ServiceAccount and ConfigMap")
			var sa corev1.ServiceAccount
			var cm corev1.ConfigMap
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-sa", Namespace: "default"}, &sa)).To(Succeed())
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-metadata", Namespace: "default"}, &cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKeyWithValue("APP_NAME", resourceName))
				g.Expect(cm.Data).To(HaveKeyWithValue("APP_VERSION", "nginx:latest"))
			}, "10s", "250ms").Should(Succeed())

			By("Simulating that the Deployment is Ready")
			dep.Status.Replicas = 1
			dep.Status.ReadyReplicas = 1
			dep.Status.AvailableReplicas = 1
			Expect(k8sClient.Status().Update(ctx, &dep)).To(Succeed())

			By("Checking that the operator updated the Status to Ready=True")
			Eventually(func(g Gomega) {
				var got platformv1alpha1.CoreApp
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &got)).To(Succeed())

				cond := apimeta.FindStatusCondition(got.Status.Conditions, "Ready")
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}, "10s", "250ms").Should(Succeed())
		})
	})

	Context("When reconciling a fully-configured CoreApp", func() {
		const resourceName = "test-full"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a CoreApp with PDB, Route, Probes, Env, Volumes and custom SA")
			minAvail := intstr.FromInt32(1)
			resource := &platformv1alpha1.CoreApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.CoreAppSpec{
					Image: "myapp:v1.0",
					Port:  3000,
					Autoscaling: platformv1alpha1.Autoscaling{
						MaxReplicas: 3,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
					ServiceAccountName: "my-custom-sa",
					PDB: &platformv1alpha1.PDBConfig{
						MinAvailable: &minAvail,
					},
					Route: &platformv1alpha1.AppRoute{
						Host: "myapp.example.com",
						Path: "/api",
					},
					Probes: &platformv1alpha1.ProbeConfig{
						Liveness: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt32(3000),
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       15,
						},
						Readiness: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt32(3000),
								},
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
					},
					Env: []corev1.EnvVar{
						{Name: "LOG_LEVEL", Value: "debug"},
					},
					Volumes: &platformv1alpha1.Volumes{
						ConfigMaps: []platformv1alpha1.ConfigMount{
							{SourceName: "app-config", MountPath: "/etc/config"},
						},
						Secrets: []platformv1alpha1.ConfigMount{
							{SourceName: "app-secret", MountPath: "/etc/secrets"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.CoreApp{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create PDB, use custom SA and configure HTTP probes", func() {
			By("Checking that the Deployment uses the custom ServiceAccount and HTTP probes")
			var dep appsv1.Deployment
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dep)).To(Succeed())
				g.Expect(dep.Spec.Template.Spec.ServiceAccountName).To(Equal("my-custom-sa"))

				container := dep.Spec.Template.Spec.Containers[0]
				g.Expect(container.Image).To(Equal("myapp:v1.0"))

				// Validate custom probes
				g.Expect(container.LivenessProbe).NotTo(BeNil())
				g.Expect(container.LivenessProbe.HTTPGet).NotTo(BeNil())
				g.Expect(container.LivenessProbe.HTTPGet.Path).To(Equal("/healthz"))
				g.Expect(container.ReadinessProbe).NotTo(BeNil())
				g.Expect(container.ReadinessProbe.HTTPGet.Path).To(Equal("/readyz"))

				// Validate volumes
				g.Expect(dep.Spec.Template.Spec.Volumes).NotTo(BeEmpty())

				// Validate env var
				foundEnv := false
				for _, e := range container.Env {
					if e.Name == "LOG_LEVEL" && e.Value == "debug" {
						foundEnv = true
					}
				}
				g.Expect(foundEnv).To(BeTrue(), "Expected LOG_LEVEL=debug env var")
			}, "10s", "250ms").Should(Succeed())

			By("Checking that the PDB was created")
			var pdb policyv1.PodDisruptionBudget
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &pdb)).To(Succeed())
				g.Expect(pdb.Spec.MinAvailable).NotTo(BeNil())
				g.Expect(pdb.Spec.MinAvailable.IntVal).To(Equal(int32(1)))
			}, "10s", "250ms").Should(Succeed())

			By("Checking that the HTTPRoute was created")
			var route gatewayv1.HTTPRoute
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &route)).To(Succeed())
				g.Expect(route.Spec.Hostnames).To(ContainElement(gatewayv1.Hostname("myapp.example.com")))
			}, "10s", "250ms").Should(Succeed())

			By("Checking that the custom SA was created with that name")
			var sa corev1.ServiceAccount
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "my-custom-sa", Namespace: "default"}, &sa)).To(Succeed())
			}, "10s", "250ms").Should(Succeed())
		})
	})

	Context("When reconciling with SecureByDefault disabled", func() {
		const resourceName = "test-insecure"
		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a CoreApp with SecureByDefault=false")
			secureByDefault := false
			resource := &platformv1alpha1.CoreApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: platformv1alpha1.CoreAppSpec{
					Image: "nginx:latest",
					Port:  80,
					Autoscaling: platformv1alpha1.Autoscaling{
						MaxReplicas: 1,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
					SecureByDefault: &secureByDefault,
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &platformv1alpha1.CoreApp{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create the Deployment WITHOUT SecurityContext and WITHOUT TopologySpread", func() {
			var dep appsv1.Deployment
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, typeNamespacedName, &dep)).To(Succeed())

				// RunAsNonRoot should NOT be set (we didn't inject it)
				if dep.Spec.Template.Spec.SecurityContext != nil {
					g.Expect(dep.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeNil())
				}

				// Container SecurityContext should not have restrictive caps
				container := dep.Spec.Template.Spec.Containers[0]
				if container.SecurityContext != nil {
					g.Expect(container.SecurityContext.AllowPrivilegeEscalation).To(BeNil())
				}

				// Without restrictive TopologySpreadConstraints
				g.Expect(dep.Spec.Template.Spec.TopologySpreadConstraints).To(BeEmpty())
			}, "10s", "250ms").Should(Succeed())

			By("Checking that NO NetworkPolicy was created")
			var netpol networkingv1.NetworkPolicy
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, &netpol)
			}, "3s", "250ms").ShouldNot(Succeed())
		})
	})
})
