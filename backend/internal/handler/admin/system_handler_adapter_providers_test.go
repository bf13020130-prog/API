package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupSystemAdapterProviderRouter(repo service.AdapterProviderRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSystemHandler(nil, nil, service.NewAdapterProviderService(repo), nil)
	router.GET("/api/v1/admin/system/adapter-providers", handler.ListAdapterProviders)
	router.GET("/api/v1/admin/system/adapter-providers/diagnostics", handler.GetAdapterProviderDiagnostics)
	router.POST("/api/v1/admin/system/adapter-providers", handler.CreateAdapterProvider)
	router.PUT("/api/v1/admin/system/adapter-providers/:id", handler.UpdateAdapterProvider)
	router.DELETE("/api/v1/admin/system/adapter-providers/:id", handler.DeleteAdapterProvider)
	return router
}

func TestSystemHandlerAdapterProviderDiagnosticsEmptyDB(t *testing.T) {
	router := setupSystemAdapterProviderRouter(newSystemAdapterProviderRepoStub())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-providers/diagnostics", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data struct {
			ObserveOnly bool     `json:"observe_only"`
			ActiveSlugs []string `json:"active_slugs"`
			Providers   []any    `json:"providers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Data.ObserveOnly)
	require.Empty(t, resp.Data.ActiveSlugs)
	require.Empty(t, resp.Data.Providers)
}

func TestSystemHandlerAdapterProviderDiagnosticsShowsValidityWithoutCredentials(t *testing.T) {
	repo := newSystemAdapterProviderRepoStub()
	repo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         " MidJourney ",
		Status:       "active",
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/midjourney",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "secret-token"},
		Capabilities: []string{"image_task"},
		Priority:     10,
		TimeoutMS:    30000,
	}
	repo.providers[2] = &service.AdapterProvider{
		ID:           2,
		Name:         "Suno",
		Slug:         "suno",
		Status:       "disabled",
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/suno",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "disabled-secret"},
		Capabilities: []string{"audio_task"},
	}
	repo.providers[3] = &service.AdapterProvider{
		ID:           3,
		Name:         "OpenAI should stay native",
		Slug:         "openai",
		Status:       "active",
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/openai",
		Capabilities: []string{"chat"},
	}
	repo.providers[4] = &service.AdapterProvider{
		ID:           4,
		Name:         "Broken",
		Slug:         "broken",
		Status:       "active",
		AdapterType:  "new-api",
		BaseURL:      "ftp://adapter.internal/broken",
		Capabilities: []string{"image_task"},
	}
	router := setupSystemAdapterProviderRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-providers/diagnostics", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret-token")
	require.NotContains(t, rec.Body.String(), "disabled-secret")

	var resp struct {
		Data struct {
			ObserveOnly bool     `json:"observe_only"`
			ActiveSlugs []string `json:"active_slugs"`
			Providers   []struct {
				Name         string   `json:"name"`
				Slug         string   `json:"slug"`
				Status       string   `json:"status"`
				AdapterType  string   `json:"adapter_type"`
				BaseURL      string   `json:"base_url"`
				AuthMode     string   `json:"auth_mode,omitempty"`
				Capabilities []string `json:"capabilities"`
				Priority     int      `json:"priority"`
				TimeoutMS    int      `json:"timeout_ms"`
				Valid        bool     `json:"valid"`
				Enabled      bool     `json:"enabled"`
				Reason       string   `json:"reason,omitempty"`
			} `json:"providers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Data.ObserveOnly)
	require.Equal(t, []string{"midjourney"}, resp.Data.ActiveSlugs)
	require.Len(t, resp.Data.Providers, 4)

	require.Equal(t, "midjourney", resp.Data.Providers[0].Slug)
	require.True(t, resp.Data.Providers[0].Valid)
	require.True(t, resp.Data.Providers[0].Enabled)
	require.Equal(t, "enabled", resp.Data.Providers[0].Reason)

	require.True(t, resp.Data.Providers[1].Valid)
	require.False(t, resp.Data.Providers[1].Enabled)
	require.Equal(t, "provider_disabled", resp.Data.Providers[1].Reason)

	require.False(t, resp.Data.Providers[2].Valid)
	require.False(t, resp.Data.Providers[2].Enabled)
	require.Contains(t, resp.Data.Providers[2].Reason, "core provider slug")

	require.False(t, resp.Data.Providers[3].Valid)
	require.False(t, resp.Data.Providers[3].Enabled)
	require.Contains(t, resp.Data.Providers[3].Reason, "valid http base url")
}

func TestSystemHandlerListAdapterProvidersHidesCredentials(t *testing.T) {
	repo := newSystemAdapterProviderRepoStub()
	repo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       "disabled",
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/midjourney",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "secret-token"},
		Capabilities: []string{"image_task"},
		TimeoutMS:    30000,
	}
	router := setupSystemAdapterProviderRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-providers", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret-token")

	var resp struct {
		Data []struct {
			HasCredentials bool     `json:"has_credentials"`
			CredentialKeys []string `json:"credential_keys"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.True(t, resp.Data[0].HasCredentials)
	require.Equal(t, []string{"token"}, resp.Data[0].CredentialKeys)
}

func TestSystemHandlerCreateAdapterProviderRejectsCoreSlug(t *testing.T) {
	router := setupSystemAdapterProviderRouter(newSystemAdapterProviderRepoStub())
	body := []byte(`{
		"name":"OpenAI via adapter",
		"slug":"openai",
		"status":"active",
		"adapter_type":"new-api",
		"base_url":"https://adapter.internal/openai",
		"capabilities":["chat"]
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/adapter-providers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "core provider slug")
}

type systemAdapterProviderRepoStub struct {
	nextID    int64
	providers map[int64]*service.AdapterProvider
}

func newSystemAdapterProviderRepoStub() *systemAdapterProviderRepoStub {
	return &systemAdapterProviderRepoStub{
		nextID:    1,
		providers: map[int64]*service.AdapterProvider{},
	}
}

func (r *systemAdapterProviderRepoStub) List(context.Context) ([]*service.AdapterProvider, error) {
	ids := make([]int64, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*service.AdapterProvider, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.providers[id].Clone())
	}
	return out, nil
}

func (r *systemAdapterProviderRepoStub) GetByID(_ context.Context, id int64) (*service.AdapterProvider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, service.ErrAdapterProviderNotFound
	}
	return provider.Clone(), nil
}

func (r *systemAdapterProviderRepoStub) Create(_ context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	created := provider.Clone()
	created.ID = r.nextID
	r.nextID++
	r.providers[created.ID] = created.Clone()
	return created, nil
}

func (r *systemAdapterProviderRepoStub) Update(_ context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	if _, ok := r.providers[provider.ID]; !ok {
		return nil, service.ErrAdapterProviderNotFound
	}
	updated := provider.Clone()
	r.providers[provider.ID] = updated.Clone()
	return updated, nil
}

func (r *systemAdapterProviderRepoStub) Delete(_ context.Context, id int64) error {
	if _, ok := r.providers[id]; !ok {
		return service.ErrAdapterProviderNotFound
	}
	delete(r.providers, id)
	return nil
}
