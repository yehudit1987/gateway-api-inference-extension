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
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI}, "default/openai-key")

				info, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
				assert.Equal(t, "sk-key-1", info.APIKey)
				assert.Equal(t, ProviderOpenAI, info.Provider)
			},
		},
		{
			name: "set and get with host returns stored host",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI, Host: "api.openai.com"}, "default/openai-key")

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
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "old-key", Provider: ProviderOpenAI}, "default/key")
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "new-key", Provider: ProviderAzure}, "default/key")

				info, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
				assert.Equal(t, "new-key", info.APIKey)
				assert.Equal(t, ProviderAzure, info.Provider)
			},
		},
		{
			name: "delete removes entry",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key-1", Provider: ProviderOpenAI}, "default/key")
				s.DeleteModelKey("gpt-4")

				_, found := s.GetModelKey("gpt-4")
				assert.False(t, found)
			},
		},
		{
			name: "delete nonexistent key is a no-op",
			run: func(t *testing.T, s *SecretStore) {
				s.DeleteModelKey("nonexistent")
			},
		},
		{
			name: "multiple models are independent",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "key-gpt4", Provider: ProviderOpenAI}, "default/key-gpt4")
				s.SetModelKey("claude", ModelKeyInfo{APIKey: "key-claude", Provider: ProviderAnthropic}, "default/key-claude")

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
		{
			name: "DeleteBySecret removes entry via reverse index",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key", Provider: ProviderOpenAI}, "default/openai-key")

				s.DeleteBySecret("default/openai-key")

				_, found := s.GetModelKey("gpt-4")
				assert.False(t, found)
			},
		},
		{
			name: "DeleteBySecret on unknown secret is a no-op",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key", Provider: ProviderOpenAI}, "default/openai-key")

				s.DeleteBySecret("default/unknown")

				_, found := s.GetModelKey("gpt-4")
				assert.True(t, found)
			},
		},
		{
			name: "DeleteBySecret does not affect other secrets",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "key-1", Provider: ProviderOpenAI}, "default/key-1")
				s.SetModelKey("claude", ModelKeyInfo{APIKey: "key-2", Provider: ProviderAnthropic}, "default/key-2")

				s.DeleteBySecret("default/key-1")

				_, f1 := s.GetModelKey("gpt-4")
				_, f2 := s.GetModelKey("claude")
				assert.False(t, f1)
				assert.True(t, f2)
			},
		},
		{
			name: "model-name annotation change — old mapping cleaned on re-set",
			run: func(t *testing.T, s *SecretStore) {
				s.SetModelKey("gpt-4", ModelKeyInfo{APIKey: "sk-key", Provider: ProviderOpenAI}, "default/my-secret")

				s.DeleteBySecret("default/my-secret")
				s.SetModelKey("claude-3", ModelKeyInfo{APIKey: "sk-key", Provider: ProviderAnthropic}, "default/my-secret")

				_, foundOld := s.GetModelKey("gpt-4")
				info, foundNew := s.GetModelKey("claude-3")
				assert.False(t, foundOld, "old model mapping should be gone")
				assert.True(t, foundNew)
				assert.Equal(t, "sk-key", info.APIKey)
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
			secretKey := fmt.Sprintf("default/secret-%d", n)
			s.SetModelKey(model, ModelKeyInfo{APIKey: "key", Provider: ProviderOpenAI}, secretKey)
			s.GetModelKey(model)
			s.DeleteBySecret(secretKey)
		}(i)
	}
	wg.Wait()
}
