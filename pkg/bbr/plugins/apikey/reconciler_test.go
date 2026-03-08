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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/bbr/framework"
)

func TestReconcile(t *testing.T) {
	tests := []struct {
		name         string
		secret       *corev1.Secret
		preSeed      map[string]ModelKeyInfo
		wantModel    string
		wantInfo     ModelKeyInfo
		wantFound    bool
		wantErr      bool
		secretName   string
	}{
		{
			name: "creates mapping with default provider (openai)",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openai-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4",
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("sk-live-xxx"),
				},
			},
			wantModel: "gpt-4",
			wantInfo:  ModelKeyInfo{APIKey: "sk-live-xxx", Provider: ProviderOpenAI},
			wantFound: true,
		},
		{
			name: "creates mapping with explicit anthropic provider",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "anthropic-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "claude-3",
						ProviderAnnotation:  ProviderAnthropic,
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("ant-key-123"),
				},
			},
			wantModel: "claude-3",
			wantInfo:  ModelKeyInfo{APIKey: "ant-key-123", Provider: ProviderAnthropic},
			wantFound: true,
		},
		{
			name: "creates mapping with explicit azure provider",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "azure-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4-azure",
						ProviderAnnotation:  ProviderAzure,
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("azure-key-456"),
				},
			},
			wantModel: "gpt-4-azure",
			wantInfo:  ModelKeyInfo{APIKey: "azure-key-456", Provider: ProviderAzure},
			wantFound: true,
		},
		{
			name: "creates mapping with host annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "httpbin-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "httpbin-model",
						ProviderAnnotation:  ProviderOpenAI,
						HostAnnotation:      "httpbin.org",
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("sk-dummy"),
				},
			},
			wantModel: "httpbin-model",
			wantInfo:  ModelKeyInfo{APIKey: "sk-dummy", Provider: ProviderOpenAI, Host: "httpbin.org"},
			wantFound: true,
		},
		{
			name: "creates mapping without host annotation — host is empty",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-host-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4",
						ProviderAnnotation:  ProviderOpenAI,
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("sk-key"),
				},
			},
			wantModel: "gpt-4",
			wantInfo:  ModelKeyInfo{APIKey: "sk-key", Provider: ProviderOpenAI, Host: ""},
			wantFound: true,
		},
		{
			name: "updates existing mapping on Secret change",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openai-key",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4",
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("sk-new-key"),
				},
			},
			preSeed: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-old-key", Provider: ProviderOpenAI},
			},
			wantModel: "gpt-4",
			wantInfo:  ModelKeyInfo{APIKey: "sk-new-key", Provider: ProviderOpenAI},
			wantFound: true,
		},
		{
			name:       "Secret not found — no error, no store change",
			secret:     nil,
			secretName: "gone",
			wantModel:  "gpt-4",
			wantFound:  false,
		},
		{
			name: "Secret missing model-name annotation — skips",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-annotation",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("some-key"),
				},
			},
			wantModel: "",
			wantFound: false,
		},
		{
			name: "Secret missing api-key data — skips",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-data",
					Namespace: "default",
					Labels:    map[string]string{framework.ManagedLabel: "true"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4",
					},
				},
				Data: map[string][]byte{},
			},
			wantModel: "gpt-4",
			wantFound: false,
		},
		{
			name: "Secret marked for deletion — removes key from store",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting",
					Namespace:         "default",
					Labels:            map[string]string{framework.ManagedLabel: "true"},
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
					Annotations: map[string]string{
						ModelNameAnnotation: "gpt-4",
					},
				},
				Data: map[string][]byte{
					SecretDataKey: []byte("sk-key"),
				},
			},
			preSeed: map[string]ModelKeyInfo{
				"gpt-4": {APIKey: "sk-key", Provider: ProviderOpenAI},
			},
			wantModel: "gpt-4",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSecretStore()
			for model, info := range tt.preSeed {
				store.SetModelKey(model, info)
			}

			builder := fake.NewClientBuilder()
			if tt.secret != nil {
				builder = builder.WithObjects(tt.secret)
			}
			fakeClient := builder.Build()

			reconciler := &SecretReconciler{
				Reader: fakeClient,
				Store:  store,
			}

			name := tt.secretName
			if name == "" && tt.secret != nil {
				name = tt.secret.Name
			}
			if name == "" {
				name = "test-secret"
			}

			ns := "default"
			if tt.secret != nil {
				ns = tt.secret.Namespace
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: ns,
				},
			})

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.wantModel != "" {
				info, found := store.GetModelKey(tt.wantModel)
				assert.Equal(t, tt.wantFound, found)
				if tt.wantFound {
					assert.Equal(t, tt.wantInfo, info)
				}
			}
		})
	}
}
