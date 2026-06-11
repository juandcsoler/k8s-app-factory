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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juandcsoler/platform-k8s-core-operator/internal/metrics"

	platformv1alpha1 "github.com/juandcsoler/platform-k8s-core-operator/api/v1alpha1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (c *coreAppCtx) ensureServiceAccount() (StepResult, error) {
	saConfig := c.buildServiceAccount()
	if err := c.applyResource(saConfig); err != nil {
		c.logger.Error(err, "failed to apply ServiceAccount")
		return c.updateStatusWithError("ServiceAccountFailed", err)
	}
	c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "", Kind: "ServiceAccount", Name: *saConfig.Name})
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureConfigMap() (StepResult, error) {
	cmConfig := c.buildMetadataConfigMap()
	if err := c.applyResource(cmConfig); err != nil {
		c.logger.Error(err, "failed to apply ConfigMap")
		return c.updateStatusWithError("ConfigMapFailed", err)
	}
	c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "", Kind: "ConfigMap", Name: *cmConfig.Name})
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureDeployment() (StepResult, error) {
	deployConfig, err := c.buildDeployment()
	if err != nil {
		c.logger.Error(err, "failed to build Deployment configuration")
		return c.updateStatusWithError("DeploymentConfigFailed", err)
	}

	if err := c.applyResource(deployConfig); err != nil {
		c.logger.Error(err, "failed to apply Deployment")
		return c.updateStatusWithError("DeploymentFailed", err)
	}
	c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "apps", Kind: "Deployment", Name: c.app.Name})
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureService() (StepResult, error) {
	svcConfig := c.buildService()
	if err := c.applyResource(svcConfig); err != nil {
		c.logger.Error(err, "failed to apply Service")
		return c.updateStatusWithError("ServiceFailed", err)
	}
	c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "", Kind: "Service", Name: c.app.Name})
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureHPA() (StepResult, error) {
	hpaConfig := c.buildHPA()
	if err := c.applyResource(hpaConfig); err != nil {
		c.logger.Error(err, "failed to apply HPA")
		return c.updateStatusWithError("HPAFailed", err)
	}
	c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "autoscaling", Kind: "HorizontalPodAutoscaler", Name: c.app.Name})
	return StepResult{}, nil
}

func (c *coreAppCtx) ensurePDB() (StepResult, error) {
	if c.app.Spec.PDB != nil {
		pdbConfig := c.buildPDB()
		if err := c.applyResource(pdbConfig); err != nil {
			c.logger.Error(err, "failed to apply PDB")
			return c.updateStatusWithError("PDBFailed", err)
		}
		c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "policy", Kind: "PodDisruptionBudget", Name: c.app.Name})
	} else {
		pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: c.app.Name, Namespace: c.app.Namespace}}
		_ = c.client.Delete(c.ctx, pdb)
	}
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureNetworkPolicy() (StepResult, error) {
	npConfig := c.buildNetworkPolicy()
	if npConfig != nil {
		if err := c.applyResource(npConfig); err != nil {
			c.logger.Error(err, "failed to apply NetworkPolicy")
			return c.updateStatusWithError("NetworkPolicyFailed", err)
		}
		c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "networking.k8s.io", Kind: "NetworkPolicy", Name: c.app.Name})
	} else {
		np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: c.app.Name, Namespace: c.app.Namespace}}
		_ = c.client.Delete(c.ctx, np)
	}
	return StepResult{}, nil
}

func (c *coreAppCtx) ensureHTTPRoute() (StepResult, error) {
	if c.app.Spec.Route != nil {
		routeConfig := c.buildHTTPRoute()
		if err := c.applyResource(routeConfig); err != nil {
			c.logger.Error(err, "failed to apply HTTPRoute")
			return c.updateStatusWithError("RouteFailed", err)
		}
		c.createdResources = append(c.createdResources, platformv1alpha1.CreatedResource{Group: "gateway.networking.k8s.io", Kind: "HTTPRoute", Name: c.app.Name})
	} else {
		route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: c.app.Name, Namespace: c.app.Namespace}}
		_ = c.client.Delete(c.ctx, route)
	}
	return StepResult{}, nil
}

func (c *coreAppCtx) updateStatusOnSuccess() (StepResult, error) {
	// 1. Fetch Deployment to check readiness
	var dep appsv1.Deployment
	isReady := false
	if err := c.client.Get(c.ctx, client.ObjectKey{Name: c.app.Name, Namespace: c.app.Namespace}, &dep); err != nil {
		if client.IgnoreNotFound(err) != nil {
			c.logger.Error(err, "failed to get Deployment for readiness check")
			return c.updateStatusWithError("ReadinessCheckFailed", err)
		}
		// If not found, it's not ready yet
	} else {
		isReady = dep.Status.AvailableReplicas > 0
	}

	// Update the Prometheus metric
	if isReady {
		metrics.AppReady.WithLabelValues(c.app.Name, c.app.Namespace).Set(1)
	} else {
		metrics.AppReady.WithLabelValues(c.app.Name, c.app.Namespace).Set(0)
	}

	var newReadyCondition metav1.Condition
	if isReady {
		newReadyCondition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Reconciled",
			Message:            "All resources successfully provisioned and pods are available.",
			ObservedGeneration: c.app.Generation,
		}
	} else {
		newReadyCondition = metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "Provisioning",
			Message:            "Resources provisioned, waiting for pods to become available.",
			ObservedGeneration: c.app.Generation,
		}
	}

	// 2. Check if the current Status already reflects this
	wasReady := apimeta.IsStatusConditionTrue(c.app.Status.Conditions, "Ready")
	alreadyReflectsState := wasReady == isReady
	alreadyObserved := c.app.Status.ObservedGeneration == c.app.Generation

	// 3. If we are already in this state, do not trigger an update in the API Server
	if alreadyReflectsState && alreadyObserved {
		if !isReady {
			// If not ready, requeue to keep checking
			return StepResult{Stop: true, RequeueAfter: 5 * time.Second}, nil
		}
		return StepResult{}, nil
	}

	// 4. If there are changes, update the Status
	apimeta.SetStatusCondition(&c.app.Status.Conditions, newReadyCondition)
	c.app.Status.ObservedGeneration = c.app.Generation
	c.app.Status.CreatedResources = c.createdResources

	if err := c.client.Status().Update(c.ctx, c.app); err != nil {
		c.logger.Error(err, "failed to update Status")
		return StepResult{}, err
	}

	// 5. Emit events based on readiness state
	if isReady {
		c.recorder.Eventf(c.app, nil, corev1.EventTypeNormal, "Reconciled", "ApplySuccess", "All resources successfully provisioned.")
		c.logger.Info("CoreApp successfully reconciled and status updated")
	} else {
		c.logger.Info("CoreApp provisioned, waiting for readiness...")
		return StepResult{Stop: true, RequeueAfter: 5 * time.Second}, nil
	}

	return StepResult{}, nil
}

// Operational helpers
func (c *coreAppCtx) applyResource(acObject runtime.ApplyConfiguration) error {
	return c.client.Apply(c.ctx, acObject, client.ForceOwnership, client.FieldOwner("coreapp-operator"))
}

func (c *coreAppCtx) updateStatusWithError(reason string, err error) (StepResult, error) {
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: c.app.Generation,
	}
	apimeta.SetStatusCondition(&c.app.Status.Conditions, condition)

	c.app.Status.ObservedGeneration = c.app.Generation
	c.app.Status.CreatedResources = c.createdResources

	if updateErr := c.client.Status().Update(c.ctx, c.app); updateErr != nil {
		c.logger.Error(updateErr, "failed to update Status with error condition")
		// Return the original error so the reconcile fails for the root cause
	}

	c.recorder.Eventf(c.app, nil, corev1.EventTypeWarning, reason, "ReconcileError", err.Error())
	return StepResult{}, err
}
