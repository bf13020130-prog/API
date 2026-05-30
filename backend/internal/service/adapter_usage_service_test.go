package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/stretchr/testify/require"
)

func TestAdapterUsageServiceRecordsAnalyticsFromAdapterResponse(t *testing.T) {
	repo := newMemoryAdapterUsageRepo()
	svc := NewAdapterUsageService(repo)
	now := time.Now().UTC()

	record, err := svc.RecordAdapterResult(context.Background(), AdapterUsageRecordInput{
		RequestID:         "req-usage",
		UserID:            10,
		APIKeyID:          20,
		GroupID:           ptrInt64(30),
		AdapterProviderID: 1,
		Provider:          "midjourney",
		Capability:        "image_generation",
		Model:             "mj-v6",
		RoutePolicyID:     ptrInt64(99),
		Method:            "POST",
		Path:              "/v1/images/generations",
		StatusCode:        ptrInt(200),
		DurationMS:        ptrInt(1234),
		AdapterStatus:     adapterclient.StatusSucceeded,
		Usage: adapterclient.Usage{
			InputUnits:   1200,
			OutputUnits:  2,
			BillableUnit: 2,
			CostUSD:      0.42,
		},
		BillingMetadata: map[string]any{
			"billing_applied":     true,
			"billing_fingerprint": "req-usage:20:abc",
		},
		Metadata:  map[string]any{"source": "adapter_enforcement"},
		CreatedAt: now,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), record.ID)
	require.Equal(t, "req-usage", record.RequestID)
	require.Equal(t, "midjourney", record.Provider)
	require.Equal(t, "mj-v6", record.Model)
	require.Equal(t, int64(99), *record.RoutePolicyID)
	require.Equal(t, 1200, record.InputUnits)
	require.Equal(t, 2, record.OutputUnits)
	require.Equal(t, 2, record.BillableUnits)
	require.InDelta(t, 0.42, record.CostUSD, 1e-12)
	require.True(t, record.BillingApplied)
	require.Equal(t, "req-usage:20:abc", record.BillingFingerprint)
	require.Equal(t, "adapter_enforcement", record.Metadata["source"])
	require.Equal(t, now, record.CreatedAt)
}

func TestAdapterUsageServiceSkipsInvalidOrUnmatchedRecords(t *testing.T) {
	repo := newMemoryAdapterUsageRepo()
	svc := NewAdapterUsageService(repo)

	_, err := svc.RecordAdapterResult(context.Background(), AdapterUsageRecordInput{
		RequestID:     "req-missing-provider",
		UserID:        10,
		APIKeyID:      20,
		Provider:      "",
		AdapterStatus: adapterclient.StatusSucceeded,
		Usage:         adapterclient.Usage{CostUSD: 0.42},
	})

	require.Error(t, err)
	require.Empty(t, repo.records)

	record, err := svc.RecordAdapterResult(context.Background(), AdapterUsageRecordInput{
		RequestID:     "req-free",
		UserID:        10,
		APIKeyID:      20,
		Provider:      "midjourney",
		Capability:    "image_generation",
		AdapterStatus: adapterclient.StatusSucceeded,
	})

	require.NoError(t, err)
	require.Equal(t, 1, len(repo.records))
	require.Equal(t, "req-free", record.RequestID)
	require.Zero(t, record.CostUSD)
}

func TestAdapterUsageServiceSummaryAggregatesByProvider(t *testing.T) {
	repo := newMemoryAdapterUsageRepo()
	svc := NewAdapterUsageService(repo)
	now := time.Now().UTC()
	for _, input := range []AdapterUsageRecordInput{
		{
			RequestID:     "req-1",
			UserID:        10,
			APIKeyID:      20,
			Provider:      "midjourney",
			Capability:    "image_generation",
			AdapterStatus: adapterclient.StatusSucceeded,
			Usage:         adapterclient.Usage{InputUnits: 100, OutputUnits: 1, BillableUnit: 1, CostUSD: 0.30},
			CreatedAt:     now.Add(-time.Hour),
		},
		{
			RequestID:     "req-2",
			UserID:        10,
			APIKeyID:      20,
			Provider:      "midjourney",
			Capability:    "image_generation",
			AdapterStatus: adapterclient.StatusFailed,
			Usage:         adapterclient.Usage{CostUSD: 0.10},
			ErrorMessage:  "failed",
			CreatedAt:     now,
		},
		{
			RequestID:     "req-3",
			UserID:        11,
			APIKeyID:      21,
			Provider:      "suno",
			Capability:    "audio_generation",
			AdapterStatus: adapterclient.StatusSucceeded,
			Usage:         adapterclient.Usage{InputUnits: 10, OutputUnits: 20, BillableUnit: 20, CostUSD: 0.20},
			CreatedAt:     now,
		},
	} {
		_, err := svc.RecordAdapterResult(context.Background(), input)
		require.NoError(t, err)
	}

	summary, err := svc.Summary(context.Background(), AdapterUsageFilters{Provider: "midjourney"})

	require.NoError(t, err)
	require.Equal(t, 2, summary.TotalRequests)
	require.Equal(t, 1, summary.SuccessRequests)
	require.Equal(t, 1, summary.FailedRequests)
	require.Equal(t, int64(100), summary.InputUnits)
	require.Equal(t, int64(1), summary.OutputUnits)
	require.Equal(t, int64(1), summary.BillableUnits)
	require.InDelta(t, 0.40, summary.CostUSD, 1e-12)
	require.Len(t, summary.Providers, 1)
	require.Equal(t, "midjourney", summary.Providers[0].Provider)
	require.Equal(t, 2, summary.Providers[0].TotalRequests)
}

type memoryAdapterUsageRepo struct {
	nextID  int64
	records []*AdapterUsageRecord
}

func newMemoryAdapterUsageRepo() *memoryAdapterUsageRepo {
	return &memoryAdapterUsageRepo{nextID: 1}
}

func (r *memoryAdapterUsageRepo) Create(_ context.Context, record *AdapterUsageRecord) (*AdapterUsageRecord, error) {
	created := record.Clone()
	created.ID = r.nextID
	r.nextID++
	r.records = append(r.records, created.Clone())
	return created, nil
}

func (r *memoryAdapterUsageRepo) Update(_ context.Context, record *AdapterUsageRecord) (*AdapterUsageRecord, error) {
	updated := record.Clone()
	for i := range r.records {
		if r.records[i].ID == updated.ID {
			r.records[i] = updated.Clone()
			return updated, nil
		}
	}
	r.records = append(r.records, updated.Clone())
	return updated, nil
}

func (r *memoryAdapterUsageRepo) List(_ context.Context, filters AdapterUsageFilters) ([]AdapterUsageSafeView, error) {
	limit := filters.Limit
	if limit <= 0 || limit > len(r.records) {
		limit = len(r.records)
	}
	out := make([]AdapterUsageSafeView, 0, limit)
	for i := len(r.records) - 1; i >= 0 && len(out) < limit; i-- {
		record := r.records[i]
		if filters.Provider != "" && record.Provider != filters.Provider {
			continue
		}
		out = append(out, record.SafeView())
	}
	return out, nil
}

func (r *memoryAdapterUsageRepo) Summary(_ context.Context, filters AdapterUsageFilters) (AdapterUsageSummary, error) {
	providers := map[string]*AdapterUsageProviderSummary{}
	summary := AdapterUsageSummary{}
	for _, record := range r.records {
		if filters.Provider != "" && record.Provider != filters.Provider {
			continue
		}
		summary.TotalRequests++
		if record.Status == string(adapterclient.StatusSucceeded) {
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
			provider = &AdapterUsageProviderSummary{Provider: record.Provider}
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

func ptrInt(v int) *int {
	return &v
}
