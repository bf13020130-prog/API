package service

import (
	"context"
	"errors"
	"time"
)

type AdapterRequestRecord struct {
	ID                int64
	RequestID         string
	UserID            int64
	APIKeyID          int64
	GroupID           *int64
	AdapterProviderID int64
	Provider          string
	Capability        string
	RouteTarget       string
	Method            string
	Path              string
	Model             string
	StatusCode        *int
	DurationMS        *int
	ErrorMessage      string
	Metadata          map[string]any
	CreatedAt         time.Time
}

type AdapterRequestListFilters struct {
	Provider    string
	RequestID   string
	Status      string
	Focus       string
	CreatedFrom time.Time
	CreatedTo   time.Time
	Offset      int
	Limit       int
}

type AdapterRequestSafeView struct {
	ID                int64          `json:"id"`
	RequestID         string         `json:"request_id"`
	UserID            int64          `json:"user_id"`
	APIKeyID          int64          `json:"api_key_id"`
	GroupID           *int64         `json:"group_id,omitempty"`
	AdapterProviderID int64          `json:"adapter_provider_id"`
	Provider          string         `json:"provider"`
	Capability        string         `json:"capability"`
	RouteTarget       string         `json:"route_target"`
	Method            string         `json:"method"`
	Path              string         `json:"path"`
	Model             string         `json:"model,omitempty"`
	StatusCode        *int           `json:"status_code,omitempty"`
	DurationMS        *int           `json:"duration_ms,omitempty"`
	ErrorMessage      string         `json:"error_message,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

func (r *AdapterRequestRecord) Clone() *AdapterRequestRecord {
	if r == nil {
		return nil
	}
	out := *r
	if r.GroupID != nil {
		id := *r.GroupID
		out.GroupID = &id
	}
	if r.StatusCode != nil {
		status := *r.StatusCode
		out.StatusCode = &status
	}
	if r.DurationMS != nil {
		duration := *r.DurationMS
		out.DurationMS = &duration
	}
	out.Metadata = cloneAnyMap(r.Metadata)
	return &out
}

func (r *AdapterRequestRecord) SafeView() AdapterRequestSafeView {
	if r == nil {
		return AdapterRequestSafeView{}
	}
	cloned := r.Clone()
	return AdapterRequestSafeView{
		ID:                cloned.ID,
		RequestID:         cloned.RequestID,
		UserID:            cloned.UserID,
		APIKeyID:          cloned.APIKeyID,
		GroupID:           cloned.GroupID,
		AdapterProviderID: cloned.AdapterProviderID,
		Provider:          cloned.Provider,
		Capability:        cloned.Capability,
		RouteTarget:       cloned.RouteTarget,
		Method:            cloned.Method,
		Path:              cloned.Path,
		Model:             cloned.Model,
		StatusCode:        cloned.StatusCode,
		DurationMS:        cloned.DurationMS,
		ErrorMessage:      cloned.ErrorMessage,
		Metadata:          cloned.Metadata,
		CreatedAt:         cloned.CreatedAt,
	}
}

type AdapterRequestRepository interface {
	Create(ctx context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error)
	Update(ctx context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error)
	List(ctx context.Context, filters AdapterRequestListFilters) ([]AdapterRequestSafeView, error)
	Count(ctx context.Context, filters AdapterRequestListFilters) (int, error)
}

type AdapterRequestService struct {
	repo AdapterRequestRepository
}

func NewAdapterRequestService(repo AdapterRequestRepository) *AdapterRequestService {
	return &AdapterRequestService{repo: repo}
}

func (s *AdapterRequestService) Create(ctx context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error) {
	if s == nil || s.repo == nil {
		return record.Clone(), nil
	}
	if record != nil && record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	return s.repo.Create(ctx, record)
}

func (s *AdapterRequestService) Update(ctx context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error) {
	if record == nil || record.ID <= 0 {
		return nil, errors.New("adapter request id is required")
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	if s == nil || s.repo == nil {
		return record.Clone(), nil
	}
	return s.repo.Update(ctx, record)
}

func (s *AdapterRequestService) List(ctx context.Context, filters AdapterRequestListFilters) ([]AdapterRequestSafeView, error) {
	if s == nil || s.repo == nil {
		return []AdapterRequestSafeView{}, nil
	}
	filters = normalizeAdapterRequestListFilters(filters)
	return s.repo.List(ctx, filters)
}

func (s *AdapterRequestService) Count(ctx context.Context, filters AdapterRequestListFilters) (int, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	filters = normalizeAdapterRequestListFilters(filters)
	return s.repo.Count(ctx, filters)
}

func normalizeAdapterRequestListFilters(filters AdapterRequestListFilters) AdapterRequestListFilters {
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 500 {
		filters.Limit = 500
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}
	return filters
}
