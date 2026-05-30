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

func setupSystemRoutePolicyRouter(routeRepo service.RoutePolicyRepository, providerRepo service.AdapterProviderRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adapterProviderSvc := service.NewAdapterProviderService(providerRepo)
	handler := NewSystemHandler(nil, nil, adapterProviderSvc, service.NewRoutePolicyService(routeRepo, adapterProviderSvc))
	router.GET("/api/v1/admin/system/route-policies", handler.ListRoutePolicies)
	router.GET("/api/v1/admin/system/route-policies/:id", handler.GetRoutePolicy)
	router.POST("/api/v1/admin/system/route-policies", handler.CreateRoutePolicy)
	router.PUT("/api/v1/admin/system/route-policies/:id", handler.UpdateRoutePolicy)
	router.DELETE("/api/v1/admin/system/route-policies/:id", handler.DeleteRoutePolicy)
	return router
}

func TestSystemHandlerCreateRoutePolicyDefaultsDisabled(t *testing.T) {
	providers := newSystemAdapterProviderRepoStub()
	providers.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	router := setupSystemRoutePolicyRouter(newSystemRoutePolicyRepoStub(), providers)
	body := []byte(`{
		"name":"Midjourney image jobs",
		"match_method":"post",
		"match_path":"/v1/images/generations",
		"match_capability":"image_generation",
		"target":"new_api_adapter",
		"adapter_provider_id":1
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/route-policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var resp struct {
		Data struct {
			Status            string `json:"status"`
			MatchMethod       string `json:"match_method"`
			AdapterProviderID *int64 `json:"adapter_provider_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, service.RoutePolicyStatusDisabled, resp.Data.Status)
	require.Equal(t, "POST", resp.Data.MatchMethod)
	require.NotNil(t, resp.Data.AdapterProviderID)
	require.Equal(t, int64(1), *resp.Data.AdapterProviderID)
}

func TestSystemHandlerCreateRoutePolicyRejectsCoreAdapterMatch(t *testing.T) {
	providers := newSystemAdapterProviderRepoStub()
	providers.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	router := setupSystemRoutePolicyRouter(newSystemRoutePolicyRepoStub(), providers)
	body := []byte(`{
		"name":"OpenAI must remain native",
		"match_group_platform":"openai",
		"target":"new_api_adapter",
		"adapter_provider_id":1
	}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/route-policies", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "core provider platform")
}

type systemRoutePolicyRepoStub struct {
	nextID   int64
	policies map[int64]*service.RoutePolicy
}

func newSystemRoutePolicyRepoStub() *systemRoutePolicyRepoStub {
	return &systemRoutePolicyRepoStub{nextID: 1, policies: map[int64]*service.RoutePolicy{}}
}

func (r *systemRoutePolicyRepoStub) List(context.Context) ([]*service.RoutePolicy, error) {
	ids := make([]int64, 0, len(r.policies))
	for id := range r.policies {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*service.RoutePolicy, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.policies[id].Clone())
	}
	return out, nil
}

func (r *systemRoutePolicyRepoStub) GetByID(_ context.Context, id int64) (*service.RoutePolicy, error) {
	policy, ok := r.policies[id]
	if !ok {
		return nil, service.ErrRoutePolicyNotFound
	}
	return policy.Clone(), nil
}

func (r *systemRoutePolicyRepoStub) Create(_ context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	created := policy.Clone()
	created.ID = r.nextID
	r.nextID++
	r.policies[created.ID] = created.Clone()
	return created, nil
}

func (r *systemRoutePolicyRepoStub) Update(_ context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	if _, ok := r.policies[policy.ID]; !ok {
		return nil, service.ErrRoutePolicyNotFound
	}
	updated := policy.Clone()
	r.policies[policy.ID] = updated.Clone()
	return updated, nil
}

func (r *systemRoutePolicyRepoStub) Delete(_ context.Context, id int64) error {
	if _, ok := r.policies[id]; !ok {
		return service.ErrRoutePolicyNotFound
	}
	delete(r.policies, id)
	return nil
}
