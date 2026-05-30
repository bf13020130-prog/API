package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupSystemAdapterUsageRouter(repo service.AdapterUsageRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSystemHandler(nil, nil, nil, nil, service.NewAdapterUsageService(repo))
	router.GET("/api/v1/admin/system/adapter-usages", handler.ListAdapterUsages)
	router.GET("/api/v1/admin/system/adapter-usages/summary", handler.GetAdapterUsageSummary)
	return router
}

func TestSystemHandlerAdapterUsageListAndSummary(t *testing.T) {
	repo := newSystemAdapterUsageRepoStub()
	now := time.Now().UTC()
	_, err := repo.Create(context.Background(), &service.AdapterUsageRecord{
		RequestID:          "req-midjourney-ok",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            ptrInt64Admin(30),
		AdapterProviderID:  1,
		RoutePolicyID:      ptrInt64Admin(99),
		Provider:           "midjourney",
		Capability:         "image_generation",
		Model:              "mj-v6",
		Method:             "POST",
		Path:               "/v1/images/generations",
		Status:             "succeeded",
		StatusCode:         ptrIntAdmin(200),
		DurationMS:         ptrIntAdmin(123),
		InputUnits:         1200,
		OutputUnits:        2,
		BillableUnits:      2,
		CostUSD:            0.42,
		BillableUnit:       2,
		BillingApplied:     true,
		BillingFingerprint: "fp-ok",
		Metadata:           map[string]any{"source": "adapter_enforcement"},
		CreatedAt:          now,
	})
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), &service.AdapterUsageRecord{
		RequestID:         "req-suno-fail",
		UserID:            11,
		APIKeyID:          21,
		AdapterProviderID: 2,
		Provider:          "suno",
		Capability:        "audio_generation",
		Status:            "failed",
		StatusCode:        ptrIntAdmin(502),
		ErrorMessage:      "adapter failed",
		CostUSD:           0.10,
		CreatedAt:         now.Add(time.Second),
	})
	require.NoError(t, err)
	router := setupSystemAdapterUsageRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-usages?provider=midjourney&limit=10", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var listResp struct {
		Data []struct {
			RequestID      string  `json:"request_id"`
			Provider       string  `json:"provider"`
			Status         string  `json:"status"`
			CostUSD        float64 `json:"cost_usd"`
			BillingApplied bool    `json:"billing_applied"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 1)
	require.Equal(t, "req-midjourney-ok", listResp.Data[0].RequestID)
	require.Equal(t, "succeeded", listResp.Data[0].Status)
	require.InDelta(t, 0.42, listResp.Data[0].CostUSD, 1e-12)
	require.True(t, listResp.Data[0].BillingApplied)
	require.NotContains(t, strings.ToLower(rec.Body.String()), "secret")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-usages/summary", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var summaryResp struct {
		Data service.AdapterUsageSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summaryResp))
	require.Equal(t, 2, summaryResp.Data.TotalRequests)
	require.Equal(t, 1, summaryResp.Data.SuccessRequests)
	require.Equal(t, 1, summaryResp.Data.FailedRequests)
	require.InDelta(t, 0.52, summaryResp.Data.CostUSD, 1e-12)
	require.Len(t, summaryResp.Data.Providers, 2)
}

type systemAdapterUsageRepoStub struct {
	nextID  int64
	records map[int64]*service.AdapterUsageRecord
}

func newSystemAdapterUsageRepoStub() *systemAdapterUsageRepoStub {
	return &systemAdapterUsageRepoStub{nextID: 1, records: map[int64]*service.AdapterUsageRecord{}}
}

func (r *systemAdapterUsageRepoStub) Create(_ context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
	created := record.Clone()
	created.ID = r.nextID
	r.nextID++
	r.records[created.ID] = created.Clone()
	return created, nil
}

func (r *systemAdapterUsageRepoStub) Update(_ context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
	updated := record.Clone()
	r.records[updated.ID] = updated.Clone()
	return updated, nil
}

func (r *systemAdapterUsageRepoStub) List(_ context.Context, filters service.AdapterUsageFilters) ([]service.AdapterUsageSafeView, error) {
	items := make([]*service.AdapterUsageRecord, 0, len(r.records))
	for _, record := range r.records {
		if filters.Provider != "" && record.Provider != filters.Provider {
			continue
		}
		if filters.RequestID != "" && !strings.Contains(record.RequestID, filters.RequestID) {
			continue
		}
		if filters.Status == "succeeded" && record.Status != "succeeded" {
			continue
		}
		items = append(items, record.Clone())
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	limit := filters.Limit
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]service.AdapterUsageSafeView, 0, limit)
	for _, record := range items[:limit] {
		out = append(out, record.SafeView())
	}
	return out, nil
}

func (r *systemAdapterUsageRepoStub) Summary(_ context.Context, filters service.AdapterUsageFilters) (service.AdapterUsageSummary, error) {
	summary := service.AdapterUsageSummary{}
	providers := map[string]*service.AdapterUsageProviderSummary{}
	for _, record := range r.records {
		if filters.Provider != "" && record.Provider != filters.Provider {
			continue
		}
		summary.TotalRequests++
		if record.Status == "succeeded" {
			summary.SuccessRequests++
		} else {
			summary.FailedRequests++
		}
		summary.InputUnits += int64(record.InputUnits)
		summary.OutputUnits += int64(record.OutputUnits)
		summary.BillableUnits += int64(record.BillableUnits)
		summary.CostUSD += record.CostUSD
		provider := providers[record.Provider]
		if provider == nil {
			provider = &service.AdapterUsageProviderSummary{Provider: record.Provider}
			providers[record.Provider] = provider
		}
		provider.TotalRequests++
		provider.CostUSD += record.CostUSD
	}
	for _, provider := range providers {
		summary.Providers = append(summary.Providers, *provider)
	}
	return summary, nil
}
