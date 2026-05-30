package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupSystemAdapterRequestRouter(repo service.AdapterRequestRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSystemHandler(nil, nil, nil, nil, service.NewAdapterRequestService(repo))
	router.GET("/api/v1/admin/system/adapter-requests", handler.ListAdapterRequests)
	router.GET("/api/v1/admin/system/adapter-requests/count", handler.CountAdapterRequests)
	return router
}

func TestSystemHandlerListAdapterRequestsFiltersAndReturnsBillingMetadata(t *testing.T) {
	repo := newSystemAdapterRequestRepoStub()
	now := time.Now().UTC()
	_, err := repo.Create(context.Background(), &service.AdapterRequestRecord{
		RequestID:         "req-midjourney-ok",
		UserID:            10,
		APIKeyID:          20,
		GroupID:           ptrInt64Admin(30),
		AdapterProviderID: 1,
		Provider:          "midjourney",
		Capability:        "image_generation",
		RouteTarget:       "new_api_adapter",
		Method:            "POST",
		Path:              "/v1/images/generations",
		Model:             "mj-v6",
		StatusCode:        ptrIntAdmin(200),
		DurationMS:        ptrIntAdmin(123),
		Metadata:          map[string]any{"billing_applied": true, "cost_usd": 0.42},
		CreatedAt:         now,
	})
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), &service.AdapterRequestRecord{
		RequestID:         "req-suno-fail",
		UserID:            11,
		APIKeyID:          21,
		AdapterProviderID: 2,
		Provider:          "suno",
		Capability:        "audio_generation",
		RouteTarget:       "new_api_adapter",
		Method:            "POST",
		Path:              "/v1/audio",
		StatusCode:        ptrIntAdmin(502),
		ErrorMessage:      "adapter failed",
		Metadata:          map[string]any{"billing_skipped_reason": "not_billable"},
		CreatedAt:         now.Add(time.Second),
	})
	require.NoError(t, err)
	router := setupSystemAdapterRequestRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-requests?provider=midjourney&request_id=ok&limit=10", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []struct {
			RequestID    string         `json:"request_id"`
			Provider     string         `json:"provider"`
			StatusCode   *int           `json:"status_code"`
			DurationMS   *int           `json:"duration_ms"`
			ErrorMessage string         `json:"error_message,omitempty"`
			Metadata     map[string]any `json:"metadata"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "req-midjourney-ok", resp.Data[0].RequestID)
	require.Equal(t, "midjourney", resp.Data[0].Provider)
	require.Equal(t, 200, *resp.Data[0].StatusCode)
	require.Equal(t, 123, *resp.Data[0].DurationMS)
	require.Equal(t, true, resp.Data[0].Metadata["billing_applied"])
	require.InDelta(t, 0.42, resp.Data[0].Metadata["cost_usd"], 1e-12)
	require.NotContains(t, strings.ToLower(rec.Body.String()), "secret")
}

func TestSystemHandlerListAdapterRequestsSupportsOperatorFocusFilters(t *testing.T) {
	repo := newSystemAdapterRequestRepoStub()
	now := time.Now().UTC()
	_, err := repo.Create(context.Background(), &service.AdapterRequestRecord{
		RequestID:         "req-stream",
		UserID:            10,
		APIKeyID:          20,
		AdapterProviderID: 1,
		Provider:          "midjourney",
		Capability:        "image_generation",
		RouteTarget:       "new_api_adapter",
		Method:            "POST",
		Path:              "/v1/images/generations",
		StatusCode:        ptrIntAdmin(200),
		Metadata:          map[string]any{"stream_usage_finalized": true},
		CreatedAt:         now,
	})
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), &service.AdapterRequestRecord{
		RequestID:         "req-websocket",
		UserID:            10,
		APIKeyID:          20,
		AdapterProviderID: 1,
		Provider:          "runway",
		Capability:        "chat",
		RouteTarget:       "new_api_adapter",
		Method:            "GET",
		Path:              "/v1/responses",
		StatusCode:        ptrIntAdmin(101),
		Metadata:          map[string]any{"transport": "websocket"},
		CreatedAt:         now.Add(time.Second),
	})
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), &service.AdapterRequestRecord{
		RequestID:         "req-status-failed",
		UserID:            10,
		APIKeyID:          20,
		AdapterProviderID: 1,
		Provider:          "broken",
		Capability:        "chat",
		RouteTarget:       "new_api_adapter",
		Method:            "POST",
		Path:              "/v1/chat/completions",
		StatusCode:        ptrIntAdmin(502),
		Metadata:          map[string]any{},
		CreatedAt:         now.Add(2 * time.Second),
	})
	require.NoError(t, err)
	router := setupSystemAdapterRequestRouter(repo)

	for _, tc := range []struct {
		name    string
		query   string
		wantIDs []string
	}{
		{name: "stream focus", query: "focus=stream", wantIDs: []string{"req-stream"}},
		{name: "websocket focus", query: "focus=websocket", wantIDs: []string{"req-websocket"}},
		{name: "failed status includes status code failures", query: "status=failed", wantIDs: []string{"req-status-failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-requests?"+tc.query, nil)
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var resp struct {
				Data []struct {
					RequestID string `json:"request_id"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			gotIDs := make([]string, 0, len(resp.Data))
			for _, item := range resp.Data {
				gotIDs = append(gotIDs, item.RequestID)
			}
			require.Equal(t, tc.wantIDs, gotIDs)
		})
	}
}

func TestSystemHandlerListAdapterRequestsFiltersByCreatedWindow(t *testing.T) {
	repo := newSystemAdapterRequestRepoStub()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	for _, record := range []*service.AdapterRequestRecord{
		{
			RequestID:         "req-old",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{},
			CreatedAt:         base.Add(-48 * time.Hour),
		},
		{
			RequestID:         "req-window-1",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{},
			CreatedAt:         base.Add(-2 * time.Hour),
		},
		{
			RequestID:         "req-window-2",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "runway",
			Capability:        "chat",
			RouteTarget:       "new_api_adapter",
			Method:            "GET",
			Path:              "/v1/responses",
			StatusCode:        ptrIntAdmin(101),
			Metadata:          map[string]any{},
			CreatedAt:         base.Add(-time.Hour),
		},
	} {
		_, err := repo.Create(context.Background(), record)
		require.NoError(t, err)
	}
	router := setupSystemAdapterRequestRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-requests?created_from=2026-05-30T00:00:00Z&created_to=2026-05-30T12:00:00Z", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	gotIDs := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		gotIDs = append(gotIDs, item.RequestID)
	}
	require.Equal(t, []string{"req-window-2", "req-window-1"}, gotIDs)
}

func TestSystemHandlerListAdapterRequestsSupportsOffsetPagination(t *testing.T) {
	repo := newSystemAdapterRequestRepoStub()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		_, err := repo.Create(context.Background(), &service.AdapterRequestRecord{
			RequestID:         "req-page-" + strconv.Itoa(i),
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{},
			CreatedAt:         base.Add(time.Duration(i) * time.Second),
		})
		require.NoError(t, err)
	}
	router := setupSystemAdapterRequestRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-requests?limit=2&offset=2", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	gotIDs := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		gotIDs = append(gotIDs, item.RequestID)
	}
	require.Equal(t, []string{"req-page-2", "req-page-1"}, gotIDs)
}

func TestSystemHandlerCountAdapterRequestsUsesSameFiltersWithoutPagination(t *testing.T) {
	repo := newSystemAdapterRequestRepoStub()
	base := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	records := []*service.AdapterRequestRecord{
		{
			RequestID:         "req-stream-1",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{"stream_usage_finalized": true},
			CreatedAt:         base.Add(-time.Hour),
		},
		{
			RequestID:         "req-stream-2",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{"stream_usage_finalized": true},
			CreatedAt:         base.Add(-30 * time.Minute),
		},
		{
			RequestID:         "req-stream-old",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 1,
			Provider:          "midjourney",
			Capability:        "image_generation",
			RouteTarget:       "new_api_adapter",
			Method:            "POST",
			Path:              "/v1/images/generations",
			StatusCode:        ptrIntAdmin(200),
			Metadata:          map[string]any{"stream_usage_finalized": true},
			CreatedAt:         base.Add(-48 * time.Hour),
		},
		{
			RequestID:         "req-other-provider",
			UserID:            10,
			APIKeyID:          20,
			AdapterProviderID: 2,
			Provider:          "runway",
			Capability:        "chat",
			RouteTarget:       "new_api_adapter",
			Method:            "GET",
			Path:              "/v1/responses",
			StatusCode:        ptrIntAdmin(101),
			Metadata:          map[string]any{"transport": "websocket"},
			CreatedAt:         base,
		},
	}
	for _, record := range records {
		_, err := repo.Create(context.Background(), record)
		require.NoError(t, err)
	}
	router := setupSystemAdapterRequestRouter(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/adapter-requests/count?provider=midjourney&focus=stream&created_from=2026-05-30T00:00:00Z&limit=1&offset=1", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Data.Total)
}

type systemAdapterRequestRepoStub struct {
	nextID  int64
	records map[int64]*service.AdapterRequestRecord
}

func newSystemAdapterRequestRepoStub() *systemAdapterRequestRepoStub {
	return &systemAdapterRequestRepoStub{nextID: 1, records: map[int64]*service.AdapterRequestRecord{}}
}

func (r *systemAdapterRequestRepoStub) Create(_ context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
	created := record.Clone()
	created.ID = r.nextID
	r.nextID++
	r.records[created.ID] = created.Clone()
	return created, nil
}

func (r *systemAdapterRequestRepoStub) Update(_ context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
	updated := record.Clone()
	r.records[updated.ID] = updated.Clone()
	return updated, nil
}

func (r *systemAdapterRequestRepoStub) List(_ context.Context, filters service.AdapterRequestListFilters) ([]service.AdapterRequestSafeView, error) {
	items := r.filteredRecords(filters)
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	limit := filters.Limit
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	items = items[offset:]
	if limit > len(items) {
		limit = len(items)
	}
	out := make([]service.AdapterRequestSafeView, 0, limit)
	for _, record := range items[:limit] {
		out = append(out, record.SafeView())
	}
	return out, nil
}

func (r *systemAdapterRequestRepoStub) Count(_ context.Context, filters service.AdapterRequestListFilters) (int, error) {
	return len(r.filteredRecords(filters)), nil
}

func (r *systemAdapterRequestRepoStub) filteredRecords(filters service.AdapterRequestListFilters) []*service.AdapterRequestRecord {
	items := make([]*service.AdapterRequestRecord, 0, len(r.records))
	for _, record := range r.records {
		if filters.Provider != "" && record.Provider != filters.Provider {
			continue
		}
		if filters.RequestID != "" && !strings.Contains(record.RequestID, filters.RequestID) {
			continue
		}
		if !filters.CreatedFrom.IsZero() && record.CreatedAt.Before(filters.CreatedFrom) {
			continue
		}
		if !filters.CreatedTo.IsZero() && record.CreatedAt.After(filters.CreatedTo) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(filters.Status)) {
		case "success", "succeeded":
			if adapterRequestRecordFailed(record) {
				continue
			}
		case "failed", "error":
			if !adapterRequestRecordFailed(record) {
				continue
			}
		}
		switch strings.ToLower(strings.TrimSpace(filters.Focus)) {
		case "stream", "stream_finalized":
			if record.Metadata["stream_usage_finalized"] != true {
				continue
			}
		case "websocket", "ws":
			if record.Metadata["websocket"] != true && record.Metadata["transport"] != "websocket" {
				continue
			}
		}
		items = append(items, record.Clone())
	}
	return items
}

func adapterRequestRecordFailed(record *service.AdapterRequestRecord) bool {
	if record == nil {
		return false
	}
	if record.ErrorMessage != "" {
		return true
	}
	return record.StatusCode != nil && *record.StatusCode >= 400
}

func ptrIntAdmin(v int) *int {
	return &v
}

func ptrInt64Admin(v int64) *int64 {
	return &v
}
