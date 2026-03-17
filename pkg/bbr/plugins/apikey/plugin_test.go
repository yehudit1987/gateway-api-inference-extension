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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/framework/interface/plugin"
)

// newTestPlugin creates an APIKeyInjectionPlugin for unit tests, bypassing the
// Handle-based Factory (which requires a real manager).
func newTestPlugin(cfg Config, store *SecretStore) *APIKeyInjectionPlugin {
	modelHeader := DefaultModelHeader
	if cfg.ModelHeader != "" {
		modelHeader = cfg.ModelHeader
	}
	return &APIKeyInjectionPlugin{
		typedName:   plugin.TypedName{Type: PluginType, Name: PluginType},
		modelHeader: modelHeader,
		injectors:   DefaultInjectors(),
		store:       store,
	}
}

// newTestRequest builds an InferenceRequest pre-populated with the given headers and body.
func newTestRequest(headers map[string]string, body map[string]any) *framework.InferenceRequest {
	req := framework.NewInferenceRequest()
	for k, v := range headers {
		req.Headers[k] = v
	}
	if body != nil {
		req.Body = body
	}
	return req
}

func TestProcessRequest(t *testing.T) {
	tests := []struct {
		name            string
		cfg             Config
		storeEntries    map[string]ModelKeyInfo
		headers         map[string]string
		body            map[string]any
		wantHeaders     map[string]string
		wantErr         bool
		wantErrContains string
	}{
		{
			name: "OpenAI provider — injects Authorization: Bearer",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-test-key", Provider: ProviderOpenAI},
			},
			headers: map[string]string{"X-Gateway-Model-Name": "gpt-4"},
			body:    map[string]any{"model": "gpt-4"},
			wantHeaders: map[string]string{
				"X-Gateway-Model-Name": "gpt-4",
				"Authorization":        "Bearer sk-test-key",
			},
		},
		{
			name: "Anthropic provider — injects x-api-key with raw value",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"claude-3": {APIKey: "ant-key-123", Provider: ProviderAnthropic},
			},
			headers: map[string]string{"X-Gateway-Model-Name": "claude-3"},
			body:    map[string]any{},
			wantHeaders: map[string]string{
				"X-Gateway-Model-Name": "claude-3",
				"x-api-key":           "ant-key-123",
			},
		},
		{
			name: "Azure provider — injects api-key with raw value",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"gpt-4-azure": {APIKey: "azure-key-456", Provider: ProviderAzure},
			},
			headers: map[string]string{"X-Gateway-Model-Name": "gpt-4-azure"},
			body:    map[string]any{},
			wantHeaders: map[string]string{
				"X-Gateway-Model-Name": "gpt-4-azure",
				"api-key":             "azure-key-456",
			},
		},
		{
			name: "unknown provider — falls back to OpenAI Bearer",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"some-model": {APIKey: "key-789", Provider: "unknown-provider"},
			},
			headers: map[string]string{"X-Gateway-Model-Name": "some-model"},
			body:    map[string]any{},
			wantHeaders: map[string]string{
				"X-Gateway-Model-Name": "some-model",
				"Authorization":        "Bearer key-789",
			},
		},
		{
			name: "custom model header",
			cfg:  Config{ModelHeader: "X-Selected-Model"},
			storeEntries: map[string]ModelKeyInfo{
				"claude": {APIKey: "ant-key", Provider: ProviderAnthropic},
			},
			headers: map[string]string{"X-Selected-Model": "claude"},
			body:    map[string]any{},
			wantHeaders: map[string]string{
				"X-Selected-Model": "claude",
				"x-api-key":       "ant-key",
			},
		},
		{
			name: "no model header present — passes through",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-key", Provider: ProviderOpenAI},
			},
			headers:     map[string]string{},
			body:        map[string]any{},
			wantHeaders: map[string]string{},
		},
		{
			name: "model header empty string — passes through",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-key", Provider: ProviderOpenAI},
			},
			headers:     map[string]string{"X-Gateway-Model-Name": ""},
			body:        map[string]any{},
			wantHeaders: map[string]string{"X-Gateway-Model-Name": ""},
		},
		{
			name:         "no API key found for model — passes through",
			cfg:          Config{},
			storeEntries: map[string]ModelKeyInfo{},
			headers:      map[string]string{"X-Gateway-Model-Name": "unknown-model"},
			body:         map[string]any{},
			wantHeaders:  map[string]string{"X-Gateway-Model-Name": "unknown-model"},
		},
		{
			name: "nil body — still works (body is not used)",
			cfg:  Config{},
			storeEntries: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-key", Provider: ProviderOpenAI},
			},
			headers: map[string]string{"X-Gateway-Model-Name": "gpt-4"},
			body:    nil,
			wantHeaders: map[string]string{
				"X-Gateway-Model-Name": "gpt-4",
				"Authorization":        "Bearer sk-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSecretStore()
			for model, info := range tt.storeEntries {
				store.SetModelKey(model, info, "")
			}

			p := newTestPlugin(tt.cfg, store)
			req := newTestRequest(tt.headers, tt.body)

			err := p.ProcessRequest(context.Background(), nil, req)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantHeaders, req.Headers)
		})
	}
}

func TestProcessRequestNilRequest(t *testing.T) {
	store := NewSecretStore()
	p := newTestPlugin(Config{}, store)

	err := p.ProcessRequest(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request or headers is nil")
}

func TestProcessRequestNilStore(t *testing.T) {
	p := newTestPlugin(Config{}, nil)
	req := newTestRequest(map[string]string{}, map[string]any{})

	err := p.ProcessRequest(context.Background(), nil, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret store is not initialized")
}

func TestProcessRequestMutationTracking(t *testing.T) {
	store := NewSecretStore()
	store.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key", Provider: ProviderOpenAI}, "")

	p := newTestPlugin(Config{}, store)
	req := newTestRequest(map[string]string{"X-Gateway-Model-Name": "gpt-4"}, map[string]any{})

	err := p.ProcessRequest(context.Background(), nil, req)
	require.NoError(t, err)

	mutated := req.MutatedHeaders()
	assert.Equal(t, "Bearer sk-key", mutated["Authorization"],
		"SetHeader should register the injected header in MutatedHeaders()")
}

func TestProcessRequestHostInjection(t *testing.T) {
	store := NewSecretStore()
	store.SetModelKey("httpbin-model", ModelKeyInfo{
		APIKey: "sk-dummy", Provider: ProviderOpenAI, Host: "httpbin.org",
	}, "")

	p := newTestPlugin(Config{}, store)
	req := newTestRequest(map[string]string{"X-Gateway-Model-Name": "httpbin-model"}, map[string]any{})

	err := p.ProcessRequest(context.Background(), nil, req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer sk-dummy", req.Headers["Authorization"])
	assert.Equal(t, "httpbin.org", req.Headers["Host"])

	mutated := req.MutatedHeaders()
	assert.Equal(t, "httpbin.org", mutated["Host"],
		"Host header should be tracked as a mutation")
}

func TestProcessRequestNoHostWhenEmpty(t *testing.T) {
	store := NewSecretStore()
	store.SetModelKey("gpt-4", ModelKeyInfo{
		APIKey: "sk-key", Provider: ProviderOpenAI, Host: "",
	}, "")

	p := newTestPlugin(Config{}, store)
	req := newTestRequest(map[string]string{"X-Gateway-Model-Name": "gpt-4"}, map[string]any{})

	err := p.ProcessRequest(context.Background(), nil, req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer sk-key", req.Headers["Authorization"])
	_, hasHost := req.Headers["Host"]
	assert.False(t, hasHost, "Host header should not be set when ModelKeyInfo.Host is empty")
}

func TestTypedName(t *testing.T) {
	p := newTestPlugin(Config{}, nil)
	assert.Equal(t, PluginType, p.TypedName().Type)
	assert.Equal(t, PluginType, p.TypedName().Name)
}
