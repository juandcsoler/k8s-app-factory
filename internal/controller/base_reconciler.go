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
// 1. GENERIC TYPES (The Framework Contract)
// ============================================================================

// StepResult defines the outcome of a single reconciliation step.
type StepResult struct {
	Stop         bool
	RequeueAfter time.Duration
}

// ReconciliationContext is the base interface that groups the state of an execution.
type ReconciliationContext interface {
	GetObject() client.Object
}

// StepFunction is the signature that any pipeline step must implement.
type StepFunction func() (StepResult, error)

// ReconciliationStrategy is the contract that each specific operator must fulfill.
// It defines how to inject the concrete logic into this generic engine.
type ReconciliationStrategy interface {
	BuildContext(ctx context.Context, obj client.Object, c client.Client, rec events.EventRecorder) (ReconciliationContext, error)
	GetPipeline(ctx ReconciliationContext) []StepFunction
}

// ============================================================================
// 2. THE MAIN ENGINE (Template Method)
// ============================================================================

// GenericReconciler holds the generic Kubernetes dependencies
// and the operator-specific logic provider.
type GenericReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
	Strategy ReconciliationStrategy
}

// ReconcileBase is the invariant skeleton of the reconciliation algorithm.
func (r *GenericReconciler) ReconcileBase(ctx context.Context, req ctrl.Request, emptyObj client.Object) (ctrl.Result, error) {
	start := time.Now()
	resultStatus := "success"

	defer func() {
		metrics.ReconcileDuration.WithLabelValues(req.Name, req.Namespace).Observe(time.Since(start).Seconds())
		metrics.ReconcileTotal.WithLabelValues(req.Name, req.Namespace, resultStatus).Inc()
	}()

	// 1. Fetch: Load the primary object from the cluster
	if err := r.Get(ctx, req.NamespacedName, emptyObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			resultStatus = "not_found"
			return ctrl.Result{}, nil
		}
		resultStatus = "error"
		return ctrl.Result{}, err
	}

	// 2. Setup: Build the execution context
	reconcileCtx, err := r.Strategy.BuildContext(ctx, emptyObj, r.Client, r.Recorder)
	if err != nil {
		resultStatus = "error"
		return ctrl.Result{}, err
	}

	// 3. Strategy: Get the pipeline of steps from the specific operator
	steps := r.Strategy.GetPipeline(reconcileCtx)

	// 4. Execution: Iterate over the pipeline
	for _, step := range steps {
		result, err := step()

		if err != nil {
			resultStatus = "error"
			return ctrl.Result{}, err // Failure: K8s will apply backoff
		}

		if result.Stop {
			if result.RequeueAfter > 0 {
				resultStatus = "requeued"
			}
			return ctrl.Result{RequeueAfter: result.RequeueAfter}, nil // Controlled stop
		}
	}

	// 5. Successful completion
	return ctrl.Result{}, nil
}
