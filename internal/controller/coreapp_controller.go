/*
Copyright 2026.

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
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	// Apply Configurations (Builders)
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	autoscalingv2ac "k8s.io/client-go/applyconfigurations/autoscaling/v2"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"
	gatewayv1ac "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	// OJO: Update this to your real module path
	platformv1alpha1 "github.com/juandcsoler/k8s-app-factory/api/v1alpha1"
)

type CoreAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.juandc.dev,resources=coreapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.juandc.dev,resources=coreapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

func (r *CoreAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var app platformv1alpha1.CoreApp
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Local slice to inventory successfully created resources during this run
	var createdResources []platformv1alpha1.CreatedResource

	// 1. Ensure Deployment
	deployConfig := r.buildDeployment(&app)
	if err := r.applyResource(ctx, deployConfig); err != nil {
		logger.Error(err, "failed to apply Deployment")
		return r.updateStatusWithError(ctx, &app, "DeploymentFailed", err)
	}
	createdResources = append(createdResources, platformv1alpha1.CreatedResource{
		Group: "apps",
		Kind:  "Deployment",
		Name:  app.Name,
	})

	// 2. Ensure Service
	svcConfig := r.buildService(&app)
	if err := r.applyResource(ctx, svcConfig); err != nil {
		logger.Error(err, "failed to apply Service")
		return r.updateStatusWithError(ctx, &app, "ServiceFailed", err)
	}
	createdResources = append(createdResources, platformv1alpha1.CreatedResource{
		Group: "", // Core group is empty
		Kind:  "Service",
		Name:  app.Name,
	})

	// 3. Ensure HPA
	hpaConfig := r.buildHPA(&app)
	if err := r.applyResource(ctx, hpaConfig); err != nil {
		logger.Error(err, "failed to apply HPA")
		return r.updateStatusWithError(ctx, &app, "HPAFailed", err)
	}
	createdResources = append(createdResources, platformv1alpha1.CreatedResource{
		Group: "autoscaling",
		Kind:  "HorizontalPodAutoscaler",
		Name:  app.Name,
	})

	// 4. Ensure or Cleanup PDB
	if app.Spec.PDB != nil {
		pdbConfig := r.buildPDB(&app)
		if err := r.applyResource(ctx, pdbConfig); err != nil {
			logger.Error(err, "failed to apply PDB")
			return r.updateStatusWithError(ctx, &app, "PDBFailed", err)
		}
		createdResources = append(createdResources, platformv1alpha1.CreatedResource{
			Group: "policy",
			Kind:  "PodDisruptionBudget",
			Name:  app.Name,
		})
	} else {
		pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		_ = r.Delete(ctx, pdb)
	}

	// 5. Ensure or Cleanup HTTPRoute
	if app.Spec.Route != nil {
		routeConfig := r.buildHTTPRoute(&app)
		if err := r.applyResource(ctx, routeConfig); err != nil {
			logger.Error(err, "failed to apply HTTPRoute")
			return r.updateStatusWithError(ctx, &app, "RouteFailed", err)
		}
		createdResources = append(createdResources, platformv1alpha1.CreatedResource{
			Group: "gateway.networking.k8s.io",
			Kind:  "HTTPRoute",
			Name:  app.Name,
		})
	} else {
		route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		_ = r.Delete(ctx, route)
	}

	// 6. Update Status on Success
	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "All resources successfully provisioned.",
		ObservedGeneration: app.Generation,
	}
	apimeta.SetStatusCondition(&app.Status.Conditions, readyCondition)

	// Save metadata and resource inventory to the root status
	app.Status.ObservedGeneration = app.Generation
	app.Status.CreatedResources = createdResources

	if err := r.Status().Update(ctx, &app); err != nil {
		logger.Error(err, "failed to update Status")
		return ctrl.Result{}, err
	}

	logger.Info("CoreApp successfully reconciled")
	return ctrl.Result{}, nil
}

// ============================================================================
// RESOURCE BUILDERS
// ============================================================================

// buildLabels genera las etiquetas estándar recomendadas por Kubernetes
func (r *CoreAppReconciler) buildLabels(app *platformv1alpha1.CoreApp) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/managed-by": "coreapp-operator",
	}
}

func (r *CoreAppReconciler) ownerRef(app *platformv1alpha1.CoreApp) *metav1ac.OwnerReferenceApplyConfiguration {
	return metav1ac.OwnerReference().
		WithAPIVersion(platformv1alpha1.GroupVersion.String()).
		WithKind("CoreApp").
		WithName(app.Name).
		WithUID(app.UID).
		WithBlockOwnerDeletion(true).
		WithController(true)
}

func (r *CoreAppReconciler) buildDeployment(app *platformv1alpha1.CoreApp) *appsv1ac.DeploymentApplyConfiguration {
	labels := r.buildLabels(app)

	var podSecCtx *corev1ac.PodSecurityContextApplyConfiguration
	var contSecCtx *corev1ac.SecurityContextApplyConfiguration

	if app.Spec.SecureByDefault == nil || *app.Spec.SecureByDefault {
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

	if app.Spec.Volumes != nil {
		for _, cm := range app.Spec.Volumes.ConfigMaps {
			volName := "cm-" + cm.SourceName
			volumes = append(volumes, corev1ac.Volume().WithName(volName).
				WithConfigMap(corev1ac.ConfigMapVolumeSource().WithName(cm.SourceName)))
			mounts = append(mounts, corev1ac.VolumeMount().WithName(volName).WithMountPath(cm.MountPath))
		}
		for _, sec := range app.Spec.Volumes.Secrets {
			volName := "sec-" + sec.SourceName
			volumes = append(volumes, corev1ac.Volume().WithName(volName).
				WithSecret(corev1ac.SecretVolumeSource().WithSecretName(sec.SourceName)))
			mounts = append(mounts, corev1ac.VolumeMount().WithName(volName).WithMountPath(sec.MountPath).WithReadOnly(true))
		}
	}

	envVars := convertEnvVars(app.Spec.Env)
	envFroms := convertEnvFromSources(app.Spec.EnvFrom)
	resources := convertResourceRequirements(app.Spec.Resources)

	container := corev1ac.Container().
		WithName("app").
		WithImage(app.Spec.Image).
		WithImagePullPolicy(corev1.PullIfNotPresent).
		WithPorts(corev1ac.ContainerPort().WithContainerPort(app.Spec.Port).WithName("http")).
		WithEnv(envVars...).
		WithEnvFrom(envFroms...).
		WithResources(resources).
		WithVolumeMounts(mounts...).
		WithSecurityContext(contSecCtx)

	return appsv1ac.Deployment(app.Name, app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(r.ownerRef(app)).
		WithSpec(appsv1ac.DeploymentSpec().
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(labels).
				WithSpec(corev1ac.PodSpec().
					WithSecurityContext(podSecCtx).
					WithVolumes(volumes...).
					WithContainers(container))))
}

func (r *CoreAppReconciler) buildService(app *platformv1alpha1.CoreApp) *corev1ac.ServiceApplyConfiguration {
	labels := r.buildLabels(app)

	return corev1ac.Service(app.Name, app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(r.ownerRef(app)).
		WithSpec(corev1ac.ServiceSpec().
			WithSelector(labels).
			WithType(corev1.ServiceTypeClusterIP).
			WithPorts(corev1ac.ServicePort().
				WithName("http").
				WithPort(app.Spec.Port).
				WithTargetPort(intstr.FromInt32(app.Spec.Port)).
				WithProtocol(corev1.ProtocolTCP)))
}

func (r *CoreAppReconciler) buildHPA(app *platformv1alpha1.CoreApp) *autoscalingv2ac.HorizontalPodAutoscalerApplyConfiguration {
	labels := r.buildLabels(app)

	minReplicas := int32(1)
	if app.Spec.Autoscaling.MinReplicas != nil {
		minReplicas = *app.Spec.Autoscaling.MinReplicas
	}

	targetCPU := int32(80)
	if app.Spec.Autoscaling.TargetCPUUtilizationPercentage != nil {
		targetCPU = *app.Spec.Autoscaling.TargetCPUUtilizationPercentage
	}

	return autoscalingv2ac.HorizontalPodAutoscaler(app.Name, app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(r.ownerRef(app)).
		WithSpec(autoscalingv2ac.HorizontalPodAutoscalerSpec().
			WithMinReplicas(minReplicas).
			WithMaxReplicas(app.Spec.Autoscaling.MaxReplicas).
			WithScaleTargetRef(autoscalingv2ac.CrossVersionObjectReference().
				WithAPIVersion("apps/v1").
				WithKind("Deployment").
				WithName(app.Name)).
			WithMetrics(autoscalingv2ac.MetricSpec().
				WithType(autoscalingv2.ResourceMetricSourceType).
				WithResource(autoscalingv2ac.ResourceMetricSource().
					WithName(corev1.ResourceCPU).
					WithTarget(autoscalingv2ac.MetricTarget().
						WithType(autoscalingv2.UtilizationMetricType).
						WithAverageUtilization(targetCPU)))))
}

func (r *CoreAppReconciler) buildPDB(app *platformv1alpha1.CoreApp) *policyv1ac.PodDisruptionBudgetApplyConfiguration {
	labels := r.buildLabels(app)

	pdbSpec := policyv1ac.PodDisruptionBudgetSpec().
		WithSelector(metav1ac.LabelSelector().WithMatchLabels(labels))

	if app.Spec.PDB.MinAvailable != nil {
		pdbSpec.WithMinAvailable(*app.Spec.PDB.MinAvailable)
	}
	if app.Spec.PDB.MaxUnavailable != nil {
		pdbSpec.WithMaxUnavailable(*app.Spec.PDB.MaxUnavailable)
	}

	return policyv1ac.PodDisruptionBudget(app.Name, app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(r.ownerRef(app)).
		WithSpec(pdbSpec)
}

func (r *CoreAppReconciler) buildHTTPRoute(app *platformv1alpha1.CoreApp) *gatewayv1ac.HTTPRouteApplyConfiguration {
	labels := r.buildLabels(app)
	pathMatchPrefix := gatewayv1.PathMatchPathPrefix
	port := gatewayv1.PortNumber(app.Spec.Port)

	return gatewayv1ac.HTTPRoute(app.Name, app.Namespace).
		WithLabels(labels).
		WithOwnerReferences(r.ownerRef(app)).
		WithSpec(gatewayv1ac.HTTPRouteSpec().
			WithParentRefs(gatewayv1ac.ParentReference().WithName("platform-gateway")).
			WithHostnames(gatewayv1.Hostname(app.Spec.Route.Host)).
			WithRules(gatewayv1ac.HTTPRouteRule().
				WithMatches(gatewayv1ac.HTTPRouteMatch().
					WithPath(gatewayv1ac.HTTPPathMatch().
						WithType(pathMatchPrefix).
						WithValue(app.Spec.Route.Path))).
				WithBackendRefs(gatewayv1ac.HTTPBackendRef().
					WithName(gatewayv1.ObjectName(app.Name)).
					WithPort(port))))
}

// ============================================================================
// CORE HELPERS
// ============================================================================
func (r *CoreAppReconciler) applyResource(ctx context.Context, acObject runtime.ApplyConfiguration) error {
	return r.Client.Apply(ctx, acObject, client.ForceOwnership, client.FieldOwner("coreapp-operator"))
}

func (r *CoreAppReconciler) updateStatusWithError(ctx context.Context, app *platformv1alpha1.CoreApp, reason string, err error) (ctrl.Result, error) {
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: app.Generation, // Guardamos la generación incluso si hay error
	}
	apimeta.SetStatusCondition(&app.Status.Conditions, condition)

	// Actualizamos la raíz en caso de error para que herramientas GitOps lo detecten
	app.Status.ObservedGeneration = app.Generation
	_ = r.Status().Update(ctx, app)
	return ctrl.Result{}, err
}

// ============================================================================
// TYPE CONVERSION HELPERS
// ============================================================================

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
	var res corev1ac.ResourceRequirementsApplyConfiguration
	b, _ := json.Marshal(reqs)
	_ = json.Unmarshal(b, &res)
	return &res
}

// ============================================================================
// MANAGER SETUP
// ============================================================================

func (r *CoreAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.CoreApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Complete(r)
}
