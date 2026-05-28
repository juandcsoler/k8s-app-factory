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
	"encoding/json"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	autoscalingv2ac "k8s.io/client-go/applyconfigurations/autoscaling/v2"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1ac "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	platformv1alpha1 "github.com/juandcsoler/k8s-app-factory/api/v1alpha1"
)

func (c *coreAppCtx) buildLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       c.app.Name,
		"app.kubernetes.io/managed-by": "coreapp-operator",
	}
}

func (c *coreAppCtx) ownerRef() *metav1ac.OwnerReferenceApplyConfiguration {
	return metav1ac.OwnerReference().
		WithAPIVersion(platformv1alpha1.GroupVersion.String()).
		WithKind("CoreApp").
		WithName(c.app.Name).
		WithUID(c.app.UID).
		WithBlockOwnerDeletion(true).
		WithController(true)
}

func (c *coreAppCtx) buildDeployment() *appsv1ac.DeploymentApplyConfiguration {
	labels := c.buildLabels()
	var podSecCtx *corev1ac.PodSecurityContextApplyConfiguration
	var contSecCtx *corev1ac.SecurityContextApplyConfiguration

	if c.app.Spec.SecureByDefault == nil || *c.app.Spec.SecureByDefault {
		podSecCtx = corev1ac.PodSecurityContext().
			WithRunAsNonRoot(true).
			WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault))

		contSecCtx = corev1ac.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithReadOnlyRootFilesystem(true).
			WithCapabilities(corev1ac.Capabilities().WithDrop(corev1.Capability("ALL")))
	}

	var volumes []*corev1ac.VolumeApplyConfiguration
	var mounts []*corev1ac.VolumeMountApplyConfiguration

	// if c.app.Spec.Volumes != nil {
	// 	for _, cm := range c.app.Spec.Volumes.ConfigMaps {
	// 		volName := "cm-" + cm.SourceName
	// 		volumes = append(volumes, corev1ac.Volume().WithName(volName).
	// 			WithConfigMap(corev1ac.ConfigMapVolumeSource().WithName(cm.SourceName)))
	// 		mounts = append(mounts, corev1ac.VolumeMount().WithName(volName).WithMountPath(cm.MountPath))
	// 	}
	// 	for _, sec := range c.app.Spec.Volumes.Secrets {
	// 		volName := "sec-" + sec.SourceName
	// 		volumes = append(volumes, corev1ac.Volume().WithName(volName).
	// 			WithSecret(corev1ac.SecretVolumeSource().WithSecretName(sec.SourceName)))
	// 		mounts = append(mounts, corev1ac.VolumeMount().WithName(volName).WithMountPath(sec.MountPath).WithReadOnly(true))
	// 	}
	// }

	// envVars := convertEnvVars(c.app.Spec.Env)
	// envFroms := convertEnvFromSources(c.app.Spec.EnvFrom)
	// resources := convertResourceRequirements(c.app.Spec.Resources)

	container := corev1ac.Container().
		WithName("app").
		WithImage(c.app.Spec.Image).
		WithImagePullPolicy(corev1.PullIfNotPresent).
		WithPorts(corev1ac.ContainerPort().WithContainerPort(c.app.Spec.Port).WithName("http")).
		// WithEnv(envVars...).
		// WithEnvFrom(envFroms...).
		// WithResources(resources).
		WithVolumeMounts(mounts...).
		WithSecurityContext(contSecCtx)

	return appsv1ac.Deployment(c.app.Name, c.app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(c.ownerRef()).
		WithSpec(appsv1ac.DeploymentSpec().
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(labels).
				WithSpec(corev1ac.PodSpec().
					WithSecurityContext(podSecCtx).
					WithVolumes(volumes...).
					WithContainers(container))))
}

func (c *coreAppCtx) buildService() *corev1ac.ServiceApplyConfiguration {
	labels := c.buildLabels()
	return corev1ac.Service(c.app.Name, c.app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(c.ownerRef()).
		WithSpec(corev1ac.ServiceSpec().
			WithSelector(labels).
			WithType(corev1.ServiceTypeClusterIP).
			WithPorts(corev1ac.ServicePort().
				WithName("http").
				WithPort(c.app.Spec.Port).
				WithTargetPort(intstr.FromInt32(c.app.Spec.Port)).
				WithProtocol(corev1.ProtocolTCP)))
}

func (c *coreAppCtx) buildHPA() *autoscalingv2ac.HorizontalPodAutoscalerApplyConfiguration {
	labels := c.buildLabels()
	minReplicas := int32(1)
	if c.app.Spec.Autoscaling.MinReplicas != nil {
		minReplicas = *c.app.Spec.Autoscaling.MinReplicas
	}
	targetCPU := int32(80)
	if c.app.Spec.Autoscaling.TargetCPUUtilizationPercentage != nil {
		targetCPU = *c.app.Spec.Autoscaling.TargetCPUUtilizationPercentage
	}

	return autoscalingv2ac.HorizontalPodAutoscaler(c.app.Name, c.app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(c.ownerRef()).
		WithSpec(autoscalingv2ac.HorizontalPodAutoscalerSpec().
			WithMinReplicas(minReplicas).
			WithMaxReplicas(c.app.Spec.Autoscaling.MaxReplicas).
			WithScaleTargetRef(autoscalingv2ac.CrossVersionObjectReference().
				WithAPIVersion("apps/v1").
				WithKind("Deployment").
				WithName(c.app.Name)).
			WithMetrics(autoscalingv2ac.MetricSpec().
				WithType(autoscalingv2.ResourceMetricSourceType).
				WithResource(autoscalingv2ac.ResourceMetricSource().
					WithName(corev1.ResourceCPU).
					WithTarget(autoscalingv2ac.MetricTarget().
						WithType(autoscalingv2.UtilizationMetricType).
						WithAverageUtilization(targetCPU)))))
}

func (c *coreAppCtx) buildPDB() *policyv1ac.PodDisruptionBudgetApplyConfiguration {
	labels := c.buildLabels()
	pdbSpec := policyv1ac.PodDisruptionBudgetSpec().
		WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels))

	if c.app.Spec.PDB.MinAvailable != nil {
		pdbSpec.WithMinAvailable(*c.app.Spec.PDB.MinAvailable)
	}
	if c.app.Spec.PDB.MaxUnavailable != nil {
		pdbSpec.WithMaxUnavailable(*c.app.Spec.PDB.MaxUnavailable)
	}

	return policyv1ac.PodDisruptionBudget(c.app.Name, c.app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(c.ownerRef()).
		WithSpec(pdbSpec)
}

func (c *coreAppCtx) buildHTTPRoute() *gatewayv1ac.HTTPRouteApplyConfiguration {
	labels := c.buildLabels()
	pathMatchPrefix := gatewayv1.PathMatchPathPrefix
	port := gatewayv1.PortNumber(c.app.Spec.Port)

	return gatewayv1ac.HTTPRoute(c.app.Name, c.app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(c.ownerRef()).
		WithSpec(gatewayv1ac.HTTPRouteSpec().
			WithParentRefs(gatewayv1ac.ParentReference().WithName("platform-gateway")).
			WithHostnames(gatewayv1.Hostname(c.app.Spec.Route.Host)).
			WithRules(gatewayv1ac.HTTPRouteRule().
				WithMatches(gatewayv1ac.HTTPRouteMatch().
					WithPath(gatewayv1ac.HTTPPathMatch().
						WithType(pathMatchPrefix).
						WithValue(c.app.Spec.Route.Path))).
				WithBackendRefs(gatewayv1ac.HTTPBackendRef().
					WithName(gatewayv1.ObjectName(c.app.Name)).
					WithPort(port))))
}

// Helpers de conversión JSON
func convertEnvVars(envs []corev1.EnvVar) []*corev1ac.EnvVarApplyConfiguration {
	var res []*corev1ac.EnvVarApplyConfiguration
	b, _ := json.Marshal(envs)
	_ = json.Unmarshal(b, &res)
	return res
}

func convertEnvFromSources(envs []corev1.EnvFromSource) []*corev1ac.EnvFromSourceApplyConfiguration {
	var res []*corev1ac.EnvFromSourceApplyConfiguration
	b, _ := json.Marshal(envs)
	_ = json.Unmarshal(b, &res)
	return res
}

func convertResourceRequirements(reqs corev1.ResourceRequirements) *corev1ac.ResourceRequirementsApplyConfiguration {
	res := corev1ac.ResourceRequirements()
	if reqs.Requests != nil {
		res.WithRequests(reqs.Requests)
	}
	if reqs.Limits != nil {
		res.WithLimits(reqs.Limits)
	}
	return res
}
