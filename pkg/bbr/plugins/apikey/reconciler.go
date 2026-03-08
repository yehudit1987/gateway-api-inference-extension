/*
Copyright 2026 The Kubernetes Authors.

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

package apikey

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SecretReconciler watches Secrets labeled with ManagedLabel and updates
// the SecretStore with model-name → API-key mappings.
type SecretReconciler struct {
	client.Reader
	Store *SecretStore
}

// Reconcile handles create/update/delete events for managed Secrets.
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Secret", "name", req.Name, "namespace", req.Namespace)

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)

	if errors.IsNotFound(err) {
		logger.Info("Secret deleted", "name", req.Name)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get Secret: %w", err)
	}

	// Handle deletion (DeletionTimestamp is set).
	if !secret.DeletionTimestamp.IsZero() {
		modelName := secret.Annotations[ModelNameAnnotation]
		if modelName != "" {
			r.Store.DeleteModelKey(modelName)
			logger.Info("Removed API key for model", "model", modelName)
		}
		return ctrl.Result{}, nil
	}

	modelName := secret.Annotations[ModelNameAnnotation]
	if modelName == "" {
		logger.Info("Secret missing model-name annotation, skipping", "name", req.Name)
		return ctrl.Result{}, nil
	}

	apiKeyBytes, ok := secret.Data[SecretDataKey]
	if !ok || len(apiKeyBytes) == 0 {
		logger.Info("Secret missing api-key data field, skipping", "name", req.Name)
		return ctrl.Result{}, nil
	}

	provider := secret.Annotations[ProviderAnnotation]
	if provider == "" {
		provider = DefaultProvider
	}

	host := secret.Annotations[HostAnnotation]

	r.Store.SetModelKey(modelName, ModelKeyInfo{
		APIKey:   string(apiKeyBytes),
		Provider: provider,
		Host:     host,
	})
	logger.Info("Updated API key for model", "model", modelName, "provider", provider, "host", host)

	return ctrl.Result{}, nil
}

