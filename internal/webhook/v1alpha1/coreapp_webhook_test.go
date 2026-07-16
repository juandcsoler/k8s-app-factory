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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	platformv1alpha1 "github.com/juandcsoler/platform-k8s-core-operator/api/v1alpha1"
)

var _ = Describe("CoreApp Webhook", func() {
	var (
		obj       *platformv1alpha1.CoreApp
		oldObj    *platformv1alpha1.CoreApp
		validator CoreAppCustomValidator
		ctx       context.Context
	)

	BeforeEach(func() {
		obj = &platformv1alpha1.CoreApp{}
		oldObj = &platformv1alpha1.CoreApp{}
		validator = CoreAppCustomValidator{}
		ctx = context.Background()
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	Context("When creating or updating CoreApp under Validating Webhook", func() {
		It("Should admit creation with valid data", func() {
			obj.Spec.Autoscaling.MaxReplicas = 2
			min := int32(1)
			obj.Spec.Autoscaling.MinReplicas = &min
			obj.Spec.Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			}
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Should deny creation if minReplicas > maxReplicas", func() {
			obj.Spec.Autoscaling.MaxReplicas = 1
			min := int32(2)
			obj.Spec.Autoscaling.MinReplicas = &min
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot exceed maxReplicas"))
			Expect(warnings).To(BeNil())
		})

		It("Should deny creation if Route is present but Host is empty", func() {
			obj.Spec.Autoscaling.MaxReplicas = 2
			obj.Spec.Route = &platformv1alpha1.AppRoute{Host: ""}
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("route.host cannot be empty"))
			Expect(warnings).To(BeNil())
		})

		It("Should deny creation if CPU or Memory requests are missing", func() {
			obj.Spec.Autoscaling.MaxReplicas = 2
			// Resources not set
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("resources.requests.cpu and resources.requests.memory are required"))
			Expect(warnings).To(BeNil())
		})
	})
})
