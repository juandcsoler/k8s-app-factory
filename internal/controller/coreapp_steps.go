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
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/juandcsoler/k8s-app-factory/api/v1alpha1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (c *coreAppCtx) ensureDeployment() (StepResult, error) {
	deployConfig := c.buildDeployment()
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
	// 1. Definimos la condición deseada
	newReadyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "All resources successfully provisioned.",
		ObservedGeneration: c.app.Generation,
	}

	// 2. Comprobamos si el Status actual ya refleja esto
	// Usamos IsStatusConditionTrue para verificar si ya estaba Ready
	alreadyReady := apimeta.IsStatusConditionTrue(c.app.Status.Conditions, "Ready")
	alreadyObserved := c.app.Status.ObservedGeneration == c.app.Generation

	// 3. Si ya estábamos en este estado, no hacemos nada (rompemos el bucle)
	if alreadyReady && alreadyObserved {
		return StepResult{}, nil
	}

	// 4. Si hay cambios, actualizamos el Status
	apimeta.SetStatusCondition(&c.app.Status.Conditions, newReadyCondition)
	c.app.Status.ObservedGeneration = c.app.Generation
	c.app.Status.CreatedResources = c.createdResources

	if err := c.client.Status().Update(c.ctx, c.app); err != nil {
		c.logger.Error(err, "failed to update Status")
		return StepResult{}, err
	}

	// 5. Solo emitimos eventos si el estado cambia (por ejemplo, de False a True)
	c.recorder.Eventf(c.app, nil, corev1.EventTypeNormal, "Reconciled", "ApplySuccess", "All resources successfully provisioned.")
	c.logger.Info("CoreApp successfully reconciled and status updated")

	return StepResult{}, nil
}

// Helpers operativos
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
	_ = c.client.Status().Update(c.ctx, c.app)

	c.recorder.Eventf(c.app, nil, corev1.EventTypeWarning, reason, "ReconcileError", err.Error())
	return StepResult{}, err
}
