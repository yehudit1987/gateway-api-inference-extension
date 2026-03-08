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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

const (
	// PluginType is the registered name for this plugin in the BBR registry.
	PluginType = "api-key-injection"

	// DefaultModelHeader is the header that holds the selected model name,
	// typically set by a preceding plugin (e.g. vSR or body-field-to-header).
	DefaultModelHeader = "X-Gateway-Model-Name"
)

// compile-time interface check
var _ framework.RequestProcessor = &APIKeyInjectionPlugin{}

// Config defines the JSON parameters accepted from the --plugin CLI flag.
type Config struct {
	// ModelHeader is the header containing the selected model name.
	ModelHeader string `json:"model_header"`
}

// Factory creates a new APIKeyInjectionPlugin from CLI parameters and
// registers its Secret reconciler via the Handle.
// It matches the framework.FactoryFunc signature.
func Factory(name string, rawParams json.RawMessage, handle framework.Handle) (framework.BBRPlugin, error) {
	var cfg Config
	if len(rawParams) > 0 {
		if err := json.Unmarshal(rawParams, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s plugin parameters: %w", PluginType, err)
		}
	}

	modelHeader := DefaultModelHeader
	if cfg.ModelHeader != "" {
		modelHeader = cfg.ModelHeader
	}

	store := NewSecretStore()

	reconciler := &SecretReconciler{
		Reader: handle.ClientReader(),
		Store:  store,
	}
	bld := handle.ReconcilerBuilder()
	if err := bld.For(&corev1.Secret{}).Complete(reconciler); err != nil {
		return nil, fmt.Errorf("failed to register Secret reconciler for plugin %q: %w", PluginType, err)
	}

	return &APIKeyInjectionPlugin{
		typedName: plugin.TypedName{
			Type: PluginType,
			Name: name,
		},
		modelHeader: modelHeader,
		injectors:   DefaultInjectors(),
		store:       store,
	}, nil
}

// APIKeyInjectionPlugin injects an API key from a Kubernetes Secret
// into the request headers based on the target model name and provider.
// The provider (openai, anthropic, azure, etc.) determines which header
// name and value format are used.
type APIKeyInjectionPlugin struct {
	typedName   plugin.TypedName
	modelHeader string
	injectors   map[string]ApiKeyInjector
	store       *SecretStore
}

// TypedName returns the type and name tuple of this plugin instance.
func (p *APIKeyInjectionPlugin) TypedName() plugin.TypedName {
	return p.typedName
}

// ProcessRequest looks up the API key and provider for the selected model,
// then delegates to the matching ApiKeyInjector to determine the correct
// header name and value. The result is applied via request.SetHeader()
// so that the framework tracks the mutation.
// If no model header is present, no key is found, or the provider is
// unknown, the request passes through unmodified.
func (p *APIKeyInjectionPlugin) ProcessRequest(ctx context.Context, _ *framework.CycleState, request *framework.InferenceRequest) error {
	logger := log.FromContext(ctx)

	if request == nil || request.Headers == nil {
		return fmt.Errorf("request or headers is nil")
	}
	if p.store == nil {
		return fmt.Errorf("secret store is not initialized")
	}

	modelName, ok := request.Headers[p.modelHeader]
	if !ok || modelName == "" {
		logger.V(2).Info("No model header found, skipping API key injection",
			"header", p.modelHeader)
		return nil
	}

	info, found := p.store.GetModelKey(modelName)
	if !found {
		logger.V(2).Info("No API key found for model, skipping injection",
			"model", modelName)
		return nil
	}

	injector, ok := p.injectors[info.Provider]
	if !ok {
		injector = p.injectors[DefaultProvider]
	}
	if injector == nil {
		logger.V(2).Info("No injector found for provider, skipping injection",
			"provider", info.Provider)
		return nil
	}

	for headerName, headerValue := range injector.Inject(info.APIKey) {
		request.SetHeader(headerName, headerValue)
	}

	if info.Host != "" {
		request.SetHeader("Host", info.Host)
	}

	logger.Info("API key injected", "model", modelName, "provider", info.Provider, "host", info.Host)
	return nil
}
