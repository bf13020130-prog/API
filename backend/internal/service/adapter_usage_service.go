package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
)

type AdapterUsageRecordInput struct {
	RequestID         string
	UserID            int64
	APIKeyID          int64
	GroupID           *int64
	AdapterProviderID int64
	Provider          string
	Capability        string
	Model             string
	RoutePolicyID     *int64
	Method            string
	Path              string
	StatusCode        *int
	DurationMS        *int
	AdapterStatus     adapterclient.Status
	ErrorMessage      string
	Usage             adapterclient.Usage
	BillingMetadata   map[string]any
	Metadata          map[string]any
	CreatedAt         time.Time
}

type AdapterUsageRecord struct {
	ID                 int64
	RequestID          string
	UserID             int64
	APIKeyID           int64
	GroupID            *int64
	AdapterProviderID  int64
	RoutePolicyID      *int64
	Provider           string
	Capability         string
	Model              string
	Method             string
	Path               string
	Status             string
	StatusCode         *int
	DurationMS         *int
	ErrorMessage       string
	InputUnits         int
	OutputUnits        int
	BillableUnits      int
	CostUSD            float64
	BillableUnit       int
	BillingApplied     bool
	BillingFingerprint string
	Metadata           map[string]any
	CreatedAt          time.Time
}

type AdapterUsageSafeView struct {
	ID                 int64          `json:"id"`
	RequestID          string         `json:"request_id"`
	UserID             int64          `json:"user_id"`
	APIKeyID           int64          `json:"api_key_id"`
	GroupID            *int64         `json:"group_id,omitempty"`
	AdapterProviderID  int64          `json:"adapter_provider_id"`
	RoutePolicyID      *int64         `json:"route_policy_id,omitempty"`
	Provider           string         `json:"provider"`
	Capability         string         `json:"capability"`
	Model              string         `json:"model,omitempty"`
	Method             string         `json:"method"`
	Path               string         `json:"path"`
	Status             string         `json:"status"`
	StatusCode         *int           `json:"status_code,omitempty"`
	DurationMS         *int           `json:"duration_ms,omitempty"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	InputUnits         int            `json:"input_units"`
	OutputUnits        int            `json:"output_units"`
	BillableUnits      int            `json:"billable_units"`
	CostUSD            float64        `json:"cost_usd"`
	BillableUnit       int            `json:"billable_unit"`
	BillingApplied     bool           `json:"billing_applied"`
	BillingFingerprint string         `json:"billing_fingerprint,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

type AdapterUsageFilters struct {
	Provider  string
	RequestID string
	Status    string
	Limit     int
}

type AdapterUsageProviderSummary struct {
	Provider        string  `json:"provider"`
	TotalRequests   int     `json:"total_requests"`
	SuccessRequests int     `json:"success_requests"`
	FailedRequests  int     `json:"failed_requests"`
	InputUnits      int64   `json:"input_units"`
	OutputUnits     int64   `json:"output_units"`
	BillableUnits   int64   `json:"billable_units"`
	CostUSD         float64 `json:"cost_usd"`
}

type AdapterUsageSummary struct {
	TotalRequests   int                           `json:"total_requests"`
	SuccessRequests int                           `json:"success_requests"`
	FailedRequests  int                           `json:"failed_requests"`
	InputUnits      int64                         `json:"input_units"`
	OutputUnits     int64                         `json:"output_units"`
	BillableUnits   int64                         `json:"billable_units"`
	CostUSD         float64                       `json:"cost_usd"`
	Providers       []AdapterUsageProviderSummary `json:"providers"`
}

type AdapterUsageRepository interface {
	Create(ctx context.Context, record *AdapterUsageRecord) (*AdapterUsageRecord, error)
	Update(ctx context.Context, record *AdapterUsageRecord) (*AdapterUsageRecord, error)
	List(ctx context.Context, filters AdapterUsageFilters) ([]AdapterUsageSafeView, error)
	Summary(ctx context.Context, filters AdapterUsageFilters) (AdapterUsageSummary, error)
}

type AdapterUsageService struct {
	repo AdapterUsageRepository
}

func NewAdapterUsageService(repo AdapterUsageRepository) *AdapterUsageService {
	return &AdapterUsageService{repo: repo}
}

func (s *AdapterUsageService) RecordAdapterResult(ctx context.Context, input AdapterUsageRecordInput) (*AdapterUsageRecord, error) {
	record, err := adapterUsageRecordFromInput(input)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return record.Clone(), nil
	}
	return s.repo.Create(ctx, record)
}

func (s *AdapterUsageService) UpdateAdapterResult(ctx context.Context, record *AdapterUsageRecord) (*AdapterUsageRecord, error) {
	if record == nil || record.ID <= 0 {
		return nil, errors.New("adapter usage id is required")
	}
	if s == nil || s.repo == nil {
		return record.Clone(), nil
	}
	return s.repo.Update(ctx, record)
}

func (s *AdapterUsageService) List(ctx context.Context, filters AdapterUsageFilters) ([]AdapterUsageSafeView, error) {
	if s == nil || s.repo == nil {
		return []AdapterUsageSafeView{}, nil
	}
	return s.repo.List(ctx, normalizeAdapterUsageFilters(filters))
}

func (s *AdapterUsageService) Summary(ctx context.Context, filters AdapterUsageFilters) (AdapterUsageSummary, error) {
	if s == nil || s.repo == nil {
		return AdapterUsageSummary{}, nil
	}
	return s.repo.Summary(ctx, normalizeAdapterUsageFilters(filters))
}

func adapterUsageRecordFromInput(input AdapterUsageRecordInput) (*AdapterUsageRecord, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Capability = strings.TrimSpace(input.Capability)
	if input.RequestID == "" {
		return nil, errors.New("adapter usage request_id is required")
	}
	if input.UserID <= 0 || input.APIKeyID <= 0 {
		return nil, errors.New("adapter usage owner is required")
	}
	if input.Provider == "" {
		return nil, errors.New("adapter usage provider is required")
	}
	status := input.AdapterStatus
	if status == "" {
		status = adapterclient.StatusFailed
	}
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	metadata := cloneAnyMap(input.Metadata)
	billingFingerprint, _ := input.BillingMetadata["billing_fingerprint"].(string)
	return &AdapterUsageRecord{
		RequestID:          input.RequestID,
		UserID:             input.UserID,
		APIKeyID:           input.APIKeyID,
		GroupID:            cloneAdapterUsageInt64Ptr(input.GroupID),
		AdapterProviderID:  input.AdapterProviderID,
		RoutePolicyID:      cloneAdapterUsageInt64Ptr(input.RoutePolicyID),
		Provider:           input.Provider,
		Capability:         input.Capability,
		Model:              strings.TrimSpace(input.Model),
		Method:             strings.ToUpper(strings.TrimSpace(input.Method)),
		Path:               strings.TrimSpace(input.Path),
		Status:             string(status),
		StatusCode:         cloneIntPtr(input.StatusCode),
		DurationMS:         cloneIntPtr(input.DurationMS),
		ErrorMessage:       strings.TrimSpace(input.ErrorMessage),
		InputUnits:         adapterUsageInt(input.Usage.InputUnits),
		OutputUnits:        adapterUsageInt(input.Usage.OutputUnits),
		BillableUnits:      adapterUsageInt(input.Usage.BillableUnit),
		CostUSD:            input.Usage.CostUSD,
		BillableUnit:       adapterUsageInt(input.Usage.BillableUnit),
		BillingApplied:     boolValue(input.BillingMetadata["billing_applied"]),
		BillingFingerprint: strings.TrimSpace(billingFingerprint),
		Metadata:           metadata,
		CreatedAt:          createdAt,
	}, nil
}

func (r *AdapterUsageRecord) Clone() *AdapterUsageRecord {
	if r == nil {
		return nil
	}
	out := *r
	out.GroupID = cloneAdapterUsageInt64Ptr(r.GroupID)
	out.RoutePolicyID = cloneAdapterUsageInt64Ptr(r.RoutePolicyID)
	out.StatusCode = cloneIntPtr(r.StatusCode)
	out.DurationMS = cloneIntPtr(r.DurationMS)
	out.Metadata = cloneAnyMap(r.Metadata)
	return &out
}

func (r *AdapterUsageRecord) SafeView() AdapterUsageSafeView {
	if r == nil {
		return AdapterUsageSafeView{}
	}
	cloned := r.Clone()
	return AdapterUsageSafeView{
		ID:                 cloned.ID,
		RequestID:          cloned.RequestID,
		UserID:             cloned.UserID,
		APIKeyID:           cloned.APIKeyID,
		GroupID:            cloned.GroupID,
		AdapterProviderID:  cloned.AdapterProviderID,
		RoutePolicyID:      cloned.RoutePolicyID,
		Provider:           cloned.Provider,
		Capability:         cloned.Capability,
		Model:              cloned.Model,
		Method:             cloned.Method,
		Path:               cloned.Path,
		Status:             cloned.Status,
		StatusCode:         cloned.StatusCode,
		DurationMS:         cloned.DurationMS,
		ErrorMessage:       cloned.ErrorMessage,
		InputUnits:         cloned.InputUnits,
		OutputUnits:        cloned.OutputUnits,
		BillableUnits:      cloned.BillableUnits,
		CostUSD:            cloned.CostUSD,
		BillableUnit:       cloned.BillableUnit,
		BillingApplied:     cloned.BillingApplied,
		BillingFingerprint: cloned.BillingFingerprint,
		Metadata:           cloned.Metadata,
		CreatedAt:          cloned.CreatedAt,
	}
}

func normalizeAdapterUsageFilters(filters AdapterUsageFilters) AdapterUsageFilters {
	filters.Provider = strings.TrimSpace(filters.Provider)
	filters.RequestID = strings.TrimSpace(filters.RequestID)
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}
	return filters
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func cloneAdapterUsageInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func boolValue(value any) bool {
	out, _ := value.(bool)
	return out
}
