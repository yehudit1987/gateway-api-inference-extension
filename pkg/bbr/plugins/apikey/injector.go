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

const (
	// Bearer-based providers: "Authorization: Bearer <key>"
	ProviderOpenAI   = "openai"
	ProviderBedrock  = "bedrock"
	ProviderCohere   = "cohere"
	ProviderMistral  = "mistral"
	ProviderDeepSeek = "deepseek"

	// Raw-header providers: key goes in a provider-specific header
	ProviderAnthropic = "anthropic" // x-api-key: <key>
	ProviderAzure     = "azure"     // api-key: <key>

	// DefaultProvider is used when a Secret has no provider annotation.
	DefaultProvider = ProviderOpenAI
)

// ApiKeyInjector defines how an API key is injected into request headers.
// Each provider (OpenAI, Anthropic, Azure, etc.) implements this interface
// to produce the correct set of headers. Returning a map allows providers
// that require multiple headers (e.g. key + version) to do so in one call.
// The caller iterates over the map and calls request.SetHeader for each entry.
type ApiKeyInjector interface {
	Inject(apiKey string) map[string]string
}

// BearerInjector sets "Authorization: Bearer <key>".
// Used by OpenAI, AWS Bedrock, Cohere, Mistral, and other
// providers that follow the OAuth2 Bearer token convention.
type BearerInjector struct{}

func (b *BearerInjector) Inject(apiKey string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
}

// RawHeaderInjector sets one or more configurable headers.
// Used by providers that expect the key in a non-standard header
// (e.g. Anthropic uses "x-api-key", Azure uses "api-key").
// ExtraHeaders allows injecting additional provider-specific headers.
type RawHeaderInjector struct {
	HeaderName   string
	ExtraHeaders map[string]string
}

func (r *RawHeaderInjector) Inject(apiKey string) map[string]string {
	headers := map[string]string{
		r.HeaderName: apiKey,
	}
	for k, v := range r.ExtraHeaders {
		headers[k] = v
	}
	return headers
}

// DefaultInjectors returns the built-in provider-to-injector registry.
// All known providers are registered explicitly so that lookups never
// rely on the fallback for a provider we actually support.
func DefaultInjectors() map[string]ApiKeyInjector {
	bearer := &BearerInjector{}
	return map[string]ApiKeyInjector{
		ProviderOpenAI:    bearer,
		ProviderBedrock:   bearer,
		ProviderCohere:    bearer,
		ProviderMistral:   bearer,
		ProviderDeepSeek:  bearer,
		ProviderAnthropic: &RawHeaderInjector{HeaderName: "x-api-key"},
		ProviderAzure:     &RawHeaderInjector{HeaderName: "api-key"},
	}
}
