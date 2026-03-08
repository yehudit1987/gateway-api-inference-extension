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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretStore(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, s *SecretStore)
	}{
		{
			name: "set and get returns stored info",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI})

				info, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
				assert.Equal(t, "sk-key-1", info.APIKey)
				assert.Equal(t, ProviderOpenAI, info.Provider)
			},
		},
		{
			name: "set and get with host returns stored host",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI, Host: "api.openai.com"})

				info, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
				assert.Equal(t, "api.openai.com", info.Host)
			},
		},
		{
			name: "get nonexistent model returns not found",
			run: func(t *testing.T, s *SecretStore) {
				_, found := s.GetModelKey("nonexistent")
				assert.False(t, found)
			},
		},
		{
			name: "set overwrites existing entry",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "old-key", Provider: ProviderOpenAI})
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "new-key", Provider: ProviderAzure})

				info, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
				assert.Equal(t, "new-key", info.APIKey)
				assert.Equal(t, ProviderAzure, info.Provider)
			},
		},
		{
			name: "delete removes entry",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI})
				s.DeleteModelKey("gpt-4")

				_, found := s.GetModelKey("gpt-4")
				assert.False(t, found)
			},
		},
		{
			name: "delete nonexistent key is a no-op",
			run: func(t *testing.T, s *SecretStore) {
				s.DeleteModelKey("nonexistent") // should not panic
			},
		},
		{
			name: "multiple models are independent",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "key-gpt4", Provider: ProviderOpenAI})
				s.SetModelKey("claude", ModelKeyInfo{APIKey: "key-claude", Provider: ProviderAnthropic})

				i1, f1 := s.GetModelKey("gpt-4")
				i2, f2 := s.GetModelKey("claude")
				assert.True(t, f1)
				assert.True(t, f2)
				assert.Equal(t, "key-gpt4", i1.APIKey)
				assert.Equal(t, "key-claude", i2.APIKey)

				s.DeleteModelKey("gpt-4")
				_, f1 = s.GetModelKey("gpt-4")
				_, f2 = s.GetModelKey("claude")
				assert.False(t, f1)
				assert.True(t, f2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSecretStore()
			tt.run(t, s)
		})
	}
}

func TestSecretStoreConcurrentAccess(t *testing.T) {
	s := NewSecretStore()
	var wg sync.WaitGroup
	goroutines := 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			model := fmt.Sprintf("model-%d", n)
			s.SetModelKey(model, ModelKeyInfo{APIKey: "key", Provider: ProviderOpenAI})
			s.GetModelKey(model)
			s.DeleteModelKey(model)
		}(i)
	}
	wg.Wait()
}
