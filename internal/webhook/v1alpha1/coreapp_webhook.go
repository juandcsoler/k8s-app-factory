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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	platformv1alpha1 "github.com/juandcsoler/platform-k8s-core-operator/api/v1alpha1"
)

// nolint:unused
// log is for logging in this package.
var coreapplog = logf.Log.WithName("coreapp-resource")

// SetupCoreAppWebhookWithManager registers the webhook for CoreApp in the manager.
func SetupCoreAppWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &platformv1alpha1.CoreApp{}).
		WithValidator(&CoreAppCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-platform-juandc-dev-v1alpha1-coreapp,mutating=false,failurePolicy=fail,sideEffects=None,groups=platform.juandc.dev,resources=coreapps,verbs=create;update,versions=v1alpha1,name=vcoreapp-v1alpha1.kb.io,admissionReviewVersions=v1

// CoreAppCustomValidator struct is responsible for validating the CoreApp resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type CoreAppCustomValidator struct{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type CoreApp.
func (v *CoreAppCustomValidator) ValidateCreate(_ context.Context, obj *platformv1alpha1.CoreApp) (admission.Warnings, error) {
	coreapplog.Info("Validation for CoreApp upon creation", "name", obj.GetName())
	return nil, v.validate(obj)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type CoreApp.
func (v *CoreAppCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *platformv1alpha1.CoreApp) (admission.Warnings, error) {
	coreapplog.Info("Validation for CoreApp upon update", "name", newObj.GetName())
	return nil, v.validate(newObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type CoreApp.
func (v *CoreAppCustomValidator) ValidateDelete(_ context.Context, obj *platformv1alpha1.CoreApp) (admission.Warnings, error) {
	coreapplog.Info("Validation for CoreApp upon deletion", "name", obj.GetName())
	return nil, nil
}

func (v *CoreAppCustomValidator) validate(app *platformv1alpha1.CoreApp) error {
	// Validate Autoscaling
	if app.Spec.Autoscaling.MinReplicas != nil {
		if *app.Spec.Autoscaling.MinReplicas > app.Spec.Autoscaling.MaxReplicas {
			return fmt.Errorf("minReplicas (%d) cannot exceed maxReplicas (%d)", *app.Spec.Autoscaling.MinReplicas, app.Spec.Autoscaling.MaxReplicas)
		}
	} else {
		// MinReplicas defaults to 1
		if 1 > app.Spec.Autoscaling.MaxReplicas {
			return fmt.Errorf("default minReplicas (1) cannot exceed maxReplicas (%d)", app.Spec.Autoscaling.MaxReplicas)
		}
	}

	// Validate Route
	if app.Spec.Route != nil {
		if app.Spec.Route.Host == "" {
			return fmt.Errorf("route.host cannot be empty when route is specified")
		}
	}

	// Validate Resources
	if app.Spec.Resources.Requests == nil ||
		app.Spec.Resources.Requests.Cpu().IsZero() ||
		app.Spec.Resources.Requests.Memory().IsZero() {
		return fmt.Errorf("resources.requests.cpu and resources.requests.memory are required for the HorizontalPodAutoscaler to function correctly")
	}

	return nil
}
