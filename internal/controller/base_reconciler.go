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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juandcsoler/k8s-app-factory/internal/metrics"
)

// ============================================================================
// 1. TIPOS GENÉRICOS (El contrato del Framework)
// ============================================================================

// StepResult define el resultado de un único paso de reconciliación.
type StepResult struct {
	Stop         bool
	RequeueAfter time.Duration
}

// ReconciliationContext es la interfaz base que agrupa el estado de una ejecución.
type ReconciliationContext interface {
	GetObject() client.Object
}

// StepFunction es la firma que debe tener cualquier paso del pipeline.
type StepFunction func() (StepResult, error)

// ReconciliationStrategy es el contrato que cada operador específico debe cumplir.
// Define cómo inyectar la lógica concreta dentro de este motor genérico.
type ReconciliationStrategy interface {
	BuildContext(ctx context.Context, obj client.Object, c client.Client, rec events.EventRecorder) (ReconciliationContext, error)
	GetPipeline(ctx ReconciliationContext) []StepFunction
}

// ============================================================================
// 2. EL MOTOR PRINCIPAL (Template Method)
// ============================================================================

// GenericReconciler contiene las dependencias genéricas de Kubernetes
// y el proveedor de lógica específico del operador.
type GenericReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Strategy ReconciliationStrategy
}

// ReconcileBase es el esqueleto invariable del algoritmo de reconciliación.
func (r *GenericReconciler) ReconcileBase(ctx context.Context, req ctrl.Request, emptyObj client.Object) (ctrl.Result, error) {
	start := time.Now()
	resultStatus := "success"

	defer func() {
		metrics.ReconcileDuration.WithLabelValues(req.Name, req.Namespace).Observe(time.Since(start).Seconds())
		metrics.ReconcileTotal.WithLabelValues(req.Name, req.Namespace, resultStatus).Inc()
	}()

	// 1. Fetch: Cargar el objeto primario desde el clúster
	if err := r.Get(ctx, req.NamespacedName, emptyObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			resultStatus = "not_found"
			return ctrl.Result{}, nil
		}
		resultStatus = "error"
		return ctrl.Result{}, err
	}

	// 2. State: Pedir a la estrategia que construya el contexto para este ciclo
	reconcileCtx, err := r.Strategy.BuildContext(ctx, emptyObj, r.Client, r.Recorder)
	if err != nil {
		resultStatus = "error"
		return ctrl.Result{}, err
	}

	// 3. Strategy: Obtener el pipeline de pasos del operador específico
	steps := r.Strategy.GetPipeline(reconcileCtx)

	// 4. Execution: Iterar sobre el pipeline
	for _, step := range steps {
		result, err := step()

		if err != nil {
			resultStatus = "error"
			return ctrl.Result{}, err // Fallo: K8s aplicará backoff
		}

		if result.Stop {
			if result.RequeueAfter > 0 {
				resultStatus = "requeued"
			}
			return ctrl.Result{RequeueAfter: result.RequeueAfter}, nil // Parada controlada
		}
	}

	// 5. Finalización exitosa
	return ctrl.Result{}, nil
}
