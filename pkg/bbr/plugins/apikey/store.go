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

import "sync"

const (
	// ModelNameAnnotation identifies which model an API key Secret is associated with.
	ModelNameAnnotation = "inference.networking.k8s.io/model-name"

	// ProviderAnnotation identifies the provider type for API key injection.
	// Supported values: "openai", "anthropic", "azure".
	// Defaults to "openai" if not set.
	ProviderAnnotation = "inference.networking.k8s.io/provider"

	// HostAnnotation specifies the target Host header for the external endpoint.
	// When set, the plugin injects a Host header so that the request is routed
	// to the correct virtual host (e.g. "httpbin.org", "api.openai.com").
	HostAnnotation = "inference.networking.k8s.io/host"

	// SecretDataKey is the key within Secret.Data that holds the API key value.
	SecretDataKey = "api-key"
)

// ModelKeyInfo holds the API key, provider type, and optional target host for a model.
type ModelKeyInfo struct {
	APIKey   string
	Provider string
	Host     string
}

// SecretStore is a thread-safe in-memory store that maps model names
// to their API key info (key + provider).
// The SecretReconciler writes to it; the APIKeyInjectionPlugin reads from it.
type SecretStore struct {
	mu   sync.RWMutex
	keys map[string]ModelKeyInfo
}

// NewSecretStore creates an empty SecretStore.
func NewSecretStore() *SecretStore {
	return &SecretStore{keys: make(map[string]ModelKeyInfo)}
}

// SetModelKey stores or updates the API key info for a given model.
func (s *SecretStore) SetModelKey(modelName string, info ModelKeyInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[modelName] = info
}

// DeleteModelKey removes the API key info associated with a model.
func (s *SecretStore) DeleteModelKey(modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, modelName)
}

// GetModelKey returns the API key info for a model and whether it was found.
func (s *SecretStore) GetModelKey(modelName string) (ModelKeyInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.keys[modelName]
	return info, ok
}
