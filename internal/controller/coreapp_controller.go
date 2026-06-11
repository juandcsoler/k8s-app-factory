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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	platformv1alpha1 "github.com/juandcsoler/platform-k8s-core-operator/api/v1alpha1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// ============================================================================
// 1. THE CONTEXT
// ============================================================================
type coreAppCtx struct {
	ctx              context.Context
	app              *platformv1alpha1.CoreApp
	client           client.Client
	logger           logr.Logger
	recorder         events.EventRecorder
	createdResources []platformv1alpha1.CreatedResource
}

func (c *coreAppCtx) GetObject() client.Object {
	return c.app
}

// ============================================================================
// 2. THE STRATEGY
// ============================================================================
type CoreAppStrategy struct{}

func (s CoreAppStrategy) BuildContext(ctx context.Context, obj client.Object, c client.Client, rec events.EventRecorder) (ReconciliationContext, error) {
	app := obj.(*platformv1alpha1.CoreApp)
	return &coreAppCtx{
		ctx:              ctx,
		app:              app,
		client:           c,
		logger:           log.FromContext(ctx),
		recorder:         rec,
		createdResources: make([]platformv1alpha1.CreatedResource, 0, 5),
	}, nil
}

func (s CoreAppStrategy) GetPipeline(rc ReconciliationContext) []StepFunction {
	c := rc.(*coreAppCtx)
	return []StepFunction{
		c.ensureServiceAccount,
		c.ensureConfigMap,
		c.ensureDeployment,
		c.ensureService,
		c.ensureHPA,
		c.ensurePDB,
		c.ensureNetworkPolicy,
		c.ensureHTTPRoute,
		c.updateStatusOnSuccess,
	}
}

// ============================================================================
// 3. THE CONTROLLER
// ============================================================================
type CoreAppReconciler struct {
	GenericReconciler
}

// +kubebuilder:rbac:groups=platform.juandc.dev,resources=coreapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.juandc.dev,resources=coreapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (r *CoreAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.ReconcileBase(ctx, req, &platformv1alpha1.CoreApp{})
}

func (r *CoreAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// GenerationChangedPredicate filters status events in child resources,
	// avoiding unnecessary reconciliations (e.g., HPA updating metrics,
	// Deployment transitioning pods). Only changes in .spec (which increment
	// metadata.generation) will trigger a re-queue of the parent CoreApp.
	genChanged := builder.WithPredicates(predicate.GenerationChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.CoreApp{}).
		Owns(&appsv1.Deployment{}, genChanged).
		Owns(&corev1.Service{}, genChanged).
		Owns(&corev1.ServiceAccount{}, genChanged).
		Owns(&corev1.ConfigMap{}, genChanged).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}, genChanged).
		Owns(&policyv1.PodDisruptionBudget{}, genChanged).
		Owns(&networkingv1.NetworkPolicy{}, genChanged).
		Owns(&gatewayv1.HTTPRoute{}, genChanged).
		Complete(r)
}
