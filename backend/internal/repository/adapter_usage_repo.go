package repository

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/adapterusagerecord"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adapterUsageRepository struct {
	client *ent.Client
}

func NewAdapterUsageRepository(client *ent.Client) service.AdapterUsageRepository {
	return &adapterUsageRepository{client: client}
}

func (r *adapterUsageRepository) Create(ctx context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
	builder := r.client.AdapterUsageRecord.Create().
		SetRequestID(record.RequestID).
		SetUserID(record.UserID).
		SetAPIKeyID(record.APIKeyID).
		SetAdapterProviderID(record.AdapterProviderID).
		SetProvider(record.Provider).
		SetCapability(record.Capability).
		SetStatus(record.Status).
		SetInputUnits(record.InputUnits).
		SetOutputUnits(record.OutputUnits).
		SetBillableUnits(record.BillableUnits).
		SetCostUsd(record.CostUSD).
		SetBillableUnit(record.BillableUnit).
		SetBillingApplied(record.BillingApplied).
		SetMetadata(record.Metadata).
		SetCreatedAt(record.CreatedAt)

	if record.GroupID != nil {
		builder.SetGroupID(*record.GroupID)
	}
	if record.RoutePolicyID != nil {
		builder.SetRoutePolicyID(*record.RoutePolicyID)
	}
	if record.Model != "" {
		builder.SetModel(record.Model)
	}
	if record.Method != "" {
		builder.SetMethod(record.Method)
	}
	if record.Path != "" {
		builder.SetPath(record.Path)
	}
	if record.StatusCode != nil {
		builder.SetStatusCode(*record.StatusCode)
	}
	if record.DurationMS != nil {
		builder.SetDurationMs(*record.DurationMS)
	}
	if record.ErrorMessage != "" {
		builder.SetErrorMessage(record.ErrorMessage)
	}
	if record.BillingFingerprint != "" {
		builder.SetBillingFingerprint(record.BillingFingerprint)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return adapterUsageToService(created), nil
}

func (r *adapterUsageRepository) Update(ctx context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
	builder := r.client.AdapterUsageRecord.UpdateOneID(record.ID).
		SetStatus(record.Status).
		SetInputUnits(record.InputUnits).
		SetOutputUnits(record.OutputUnits).
		SetBillableUnits(record.BillableUnits).
		SetCostUsd(record.CostUSD).
		SetBillableUnit(record.BillableUnit).
		SetBillingApplied(record.BillingApplied).
		SetMetadata(record.Metadata)
	if record.StatusCode != nil {
		builder.SetStatusCode(*record.StatusCode)
	} else {
		builder.ClearStatusCode()
	}
	if record.DurationMS != nil {
		builder.SetDurationMs(*record.DurationMS)
	} else {
		builder.ClearDurationMs()
	}
	if record.ErrorMessage != "" {
		builder.SetErrorMessage(record.ErrorMessage)
	} else {
		builder.ClearErrorMessage()
	}
	if record.BillingFingerprint != "" {
		builder.SetBillingFingerprint(record.BillingFingerprint)
	} else {
		builder.ClearBillingFingerprint()
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return adapterUsageToService(updated), nil
}

func (r *adapterUsageRepository) List(ctx context.Context, filters service.AdapterUsageFilters) ([]service.AdapterUsageSafeView, error) {
	query := r.applyFilters(r.client.AdapterUsageRecord.Query(), filters)
	records, err := query.
		Order(ent.Desc(adapterusagerecord.FieldCreatedAt), ent.Desc(adapterusagerecord.FieldID)).
		Limit(filters.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.AdapterUsageSafeView, 0, len(records))
	for _, record := range records {
		out = append(out, adapterUsageToService(record).SafeView())
	}
	return out, nil
}

func (r *adapterUsageRepository) Summary(ctx context.Context, filters service.AdapterUsageFilters) (service.AdapterUsageSummary, error) {
	records, err := r.applyFilters(r.client.AdapterUsageRecord.Query(), filters).All(ctx)
	if err != nil {
		return service.AdapterUsageSummary{}, err
	}

	summary := service.AdapterUsageSummary{
		Providers: []service.AdapterUsageProviderSummary{},
	}
	providers := map[string]*service.AdapterUsageProviderSummary{}
	for _, record := range records {
		summary.TotalRequests++
		if record.Status == string(adapterUsageSucceededStatus()) {
			summary.SuccessRequests++
		} else {
			summary.FailedRequests++
		}
		summary.InputUnits += int64(record.InputUnits)
		summary.OutputUnits += int64(record.OutputUnits)
		summary.BillableUnits += int64(record.BillableUnits)
		summary.CostUSD += record.CostUsd

		provider := providers[record.Provider]
		if provider == nil {
			provider = &service.AdapterUsageProviderSummary{Provider: record.Provider}
			providers[record.Provider] = provider
		}
		provider.TotalRequests++
		if record.Status == string(adapterUsageSucceededStatus()) {
			provider.SuccessRequests++
		} else {
			provider.FailedRequests++
		}
		provider.InputUnits += int64(record.InputUnits)
		provider.OutputUnits += int64(record.OutputUnits)
		provider.BillableUnits += int64(record.BillableUnits)
		provider.CostUSD += record.CostUsd
	}
	for _, provider := range providers {
		summary.Providers = append(summary.Providers, *provider)
	}
	return summary, nil
}

func (r *adapterUsageRepository) applyFilters(query *ent.AdapterUsageRecordQuery, filters service.AdapterUsageFilters) *ent.AdapterUsageRecordQuery {
	if provider := strings.TrimSpace(filters.Provider); provider != "" {
		query = query.Where(adapterusagerecord.ProviderEQ(provider))
	}
	if requestID := strings.TrimSpace(filters.RequestID); requestID != "" {
		query = query.Where(adapterusagerecord.RequestIDContains(requestID))
	}
	switch strings.ToLower(strings.TrimSpace(filters.Status)) {
	case "success", "succeeded":
		query = query.Where(adapterusagerecord.StatusEQ("succeeded"))
	case "failed", "error":
		query = query.Where(adapterusagerecord.StatusNEQ("succeeded"))
	}
	return query
}

func adapterUsageSucceededStatus() string {
	return "succeeded"
}

func adapterUsageToService(record *ent.AdapterUsageRecord) *service.AdapterUsageRecord {
	if record == nil {
		return nil
	}
	return &service.AdapterUsageRecord{
		ID:                 record.ID,
		RequestID:          record.RequestID,
		UserID:             record.UserID,
		APIKeyID:           record.APIKeyID,
		GroupID:            int64PtrValue(record.GroupID),
		AdapterProviderID:  record.AdapterProviderID,
		RoutePolicyID:      int64PtrValue(record.RoutePolicyID),
		Provider:           record.Provider,
		Capability:         record.Capability,
		Model:              stringValue(record.Model),
		Method:             record.Method,
		Path:               record.Path,
		Status:             record.Status,
		StatusCode:         intPtrValue(record.StatusCode),
		DurationMS:         intPtrValue(record.DurationMs),
		ErrorMessage:       stringValue(record.ErrorMessage),
		InputUnits:         record.InputUnits,
		OutputUnits:        record.OutputUnits,
		BillableUnits:      record.BillableUnits,
		CostUSD:            record.CostUsd,
		BillableUnit:       record.BillableUnit,
		BillingApplied:     record.BillingApplied,
		BillingFingerprint: stringValue(record.BillingFingerprint),
		Metadata:           record.Metadata,
		CreatedAt:          record.CreatedAt,
	}
}
