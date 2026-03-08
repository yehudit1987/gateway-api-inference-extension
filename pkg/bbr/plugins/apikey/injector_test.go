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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearerInjector(t *testing.T) {
	injector := &BearerInjector{}

	headers := injector.Inject("sk-test-key")

	assert.Equal(t, "Bearer sk-test-key", headers["Authorization"])
	assert.Len(t, headers, 1)
}

func TestRawHeaderInjector(t *testing.T) {
	tests := []struct {
		name         string
		headerName   string
		extraHeaders map[string]string
		apiKey       string
		wantHeaders  map[string]string
	}{
		{
			name:       "Anthropic x-api-key only",
			headerName: "x-api-key",
			apiKey:     "ant-key-123",
			wantHeaders: map[string]string{
				"x-api-key": "ant-key-123",
			},
		},
		{
			name:       "Azure api-key only",
			headerName: "api-key",
			apiKey:     "azure-key-456",
			wantHeaders: map[string]string{
				"api-key": "azure-key-456",
			},
		},
		{
			name:         "with extra headers",
			headerName:   "x-api-key",
			extraHeaders: map[string]string{"anthropic-version": "2023-06-01"},
			apiKey:       "ant-key-789",
			wantHeaders: map[string]string{
				"x-api-key":         "ant-key-789",
				"anthropic-version": "2023-06-01",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := &RawHeaderInjector{
				HeaderName:   tt.headerName,
				ExtraHeaders: tt.extraHeaders,
			}

			got := injector.Inject(tt.apiKey)

			assert.Equal(t, tt.wantHeaders, got)
		})
	}
}

func TestDefaultInjectors(t *testing.T) {
	injectors := DefaultInjectors()

	bearerProviders := []string{
		ProviderOpenAI, ProviderBedrock, ProviderCohere,
		ProviderMistral, ProviderDeepSeek,
	}
	for _, p := range bearerProviders {
		assert.Contains(t, injectors, p)
		assert.IsType(t, &BearerInjector{}, injectors[p], "provider %s should use BearerInjector", p)
	}

	assert.Contains(t, injectors, ProviderAnthropic)
	assert.IsType(t, &RawHeaderInjector{}, injectors[ProviderAnthropic])

	assert.Contains(t, injectors, ProviderAzure)
	assert.IsType(t, &RawHeaderInjector{}, injectors[ProviderAzure])
}
