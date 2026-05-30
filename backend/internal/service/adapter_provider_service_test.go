package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdapterProviderServiceRejectsCoreProviderSlug(t *testing.T) {
	repo := newMemoryAdapterProviderRepo()
	svc := NewAdapterProviderService(repo)

	_, err := svc.Create(context.Background(), &AdapterProvider{
		Name:         "OpenAI via adapter",
		Slug:         "openai",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	})

	require.Error(t, err)
	require.Len(t, repo.providers, 0)
}

func TestAdapterProviderServiceDiagnosticsUseDatabaseProviders(t *testing.T) {
	repo := newMemoryAdapterProviderRepo()
	repo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "secret-token"},
		Capabilities: []string{"image_generation"},
		Priority:     10,
		TimeoutMS:    45000,
	}
	svc := NewAdapterProviderService(repo)

	diagnostics := svc.Diagnostics()

	require.Equal(t, []string{"midjourney"}, diagnostics.ActiveSlugs)
	require.Len(t, diagnostics.Providers, 1)
	require.Equal(t, "midjourney", diagnostics.Providers[0].Slug)
	require.True(t, diagnostics.Providers[0].Enabled)
}

func TestAdapterProviderServiceSafeViewsHideCredentialValues(t *testing.T) {
	repo := newMemoryAdapterProviderRepo()
	repo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Suno",
		Slug:         "suno",
		Status:       AdapterProviderStatusDisabled,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		AuthMode:     "header",
		Credentials:  map[string]string{"x-api-key": "secret-key"},
		Capabilities: []string{"audio_generation"},
		TimeoutMS:    30000,
	}
	svc := NewAdapterProviderService(repo)

	providers, err := svc.ListSafe(context.Background())

	require.NoError(t, err)
	require.Len(t, providers, 1)
	require.True(t, providers[0].HasCredentials)
	require.Equal(t, []string{"x-api-key"}, providers[0].CredentialKeys)
	require.NotContains(t, providers[0].CredentialKeys, "secret-key")
}

func TestAdapterProviderServiceUpdateKeepsCredentialsWhenUnset(t *testing.T) {
	repo := newMemoryAdapterProviderRepo()
	repo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusDisabled,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Credentials:  map[string]string{"token": "original-secret"},
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	svc := NewAdapterProviderService(repo)

	updated, err := svc.Update(context.Background(), &AdapterProviderUpdate{
		ID:           1,
		Name:         "Midjourney Adapter",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com/v2",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    60000,
	})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"token": "original-secret"}, updated.Credentials)
	require.Equal(t, AdapterProviderStatusActive, updated.Status)
}

type memoryAdapterProviderRepo struct {
	nextID    int64
	providers map[int64]*AdapterProvider
}

func newMemoryAdapterProviderRepo() *memoryAdapterProviderRepo {
	return &memoryAdapterProviderRepo{
		nextID:    1,
		providers: make(map[int64]*AdapterProvider),
	}
}

func (r *memoryAdapterProviderRepo) List(context.Context) ([]*AdapterProvider, error) {
	out := make([]*AdapterProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		out = append(out, provider.Clone())
	}
	return out, nil
}

func (r *memoryAdapterProviderRepo) GetByID(_ context.Context, id int64) (*AdapterProvider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, ErrAdapterProviderNotFound
	}
	return provider.Clone(), nil
}

func (r *memoryAdapterProviderRepo) Create(_ context.Context, provider *AdapterProvider) (*AdapterProvider, error) {
	created := provider.Clone()
	created.ID = r.nextID
	r.nextID++
	r.providers[created.ID] = created.Clone()
	return created, nil
}

func (r *memoryAdapterProviderRepo) Update(_ context.Context, provider *AdapterProvider) (*AdapterProvider, error) {
	if _, ok := r.providers[provider.ID]; !ok {
		return nil, ErrAdapterProviderNotFound
	}
	updated := provider.Clone()
	r.providers[provider.ID] = updated.Clone()
	return updated, nil
}

func (r *memoryAdapterProviderRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.providers[id]; !ok {
		return ErrAdapterProviderNotFound
	}
	delete(r.providers, id)
	return nil
}
