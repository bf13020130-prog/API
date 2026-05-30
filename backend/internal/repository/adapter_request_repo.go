package repository

import (
	"context"
	"strings"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/adapterrequest"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adapterRequestRepository struct {
	client *ent.Client
}

func NewAdapterRequestRepository(client *ent.Client) service.AdapterRequestRepository {
	return &adapterRequestRepository{client: client}
}

func (r *adapterRequestRepository) Create(ctx context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
	builder := r.client.AdapterRequest.Create().
		SetRequestID(record.RequestID).
		SetUserID(record.UserID).
		SetAPIKeyID(record.APIKeyID).
		SetAdapterProviderID(record.AdapterProviderID).
		SetProvider(record.Provider).
		SetCapability(record.Capability).
		SetRouteTarget(record.RouteTarget).
		SetMethod(record.Method).
		SetPath(record.Path).
		SetMetadata(record.Metadata)

	if record.GroupID != nil {
		builder.SetGroupID(*record.GroupID)
	}
	if record.Model != "" {
		builder.SetModel(record.Model)
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

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return adapterRequestToService(created), nil
}

func (r *adapterRequestRepository) Update(ctx context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
	builder := r.client.AdapterRequest.UpdateOneID(record.ID).
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
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return adapterRequestToService(updated), nil
}

func (r *adapterRequestRepository) List(ctx context.Context, filters service.AdapterRequestListFilters) ([]service.AdapterRequestSafeView, error) {
	query := applyAdapterRequestFilters(r.client.AdapterRequest.Query(), filters)
	records, err := query.
		Order(ent.Desc(adapterrequest.FieldCreatedAt), ent.Desc(adapterrequest.FieldID)).
		Offset(filters.Offset).
		Limit(filters.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.AdapterRequestSafeView, 0, len(records))
	for _, record := range records {
		out = append(out, adapterRequestToService(record).SafeView())
	}
	return out, nil
}

func (r *adapterRequestRepository) Count(ctx context.Context, filters service.AdapterRequestListFilters) (int, error) {
	return applyAdapterRequestFilters(r.client.AdapterRequest.Query(), filters).Count(ctx)
}

func applyAdapterRequestFilters(query *ent.AdapterRequestQuery, filters service.AdapterRequestListFilters) *ent.AdapterRequestQuery {
	if provider := strings.TrimSpace(filters.Provider); provider != "" {
		query = query.Where(adapterrequest.ProviderEQ(provider))
	}
	if requestID := strings.TrimSpace(filters.RequestID); requestID != "" {
		query = query.Where(adapterrequest.RequestIDContains(requestID))
	}
	if !filters.CreatedFrom.IsZero() {
		query = query.Where(adapterrequest.CreatedAtGTE(filters.CreatedFrom))
	}
	if !filters.CreatedTo.IsZero() {
		query = query.Where(adapterrequest.CreatedAtLTE(filters.CreatedTo))
	}
	switch strings.ToLower(strings.TrimSpace(filters.Status)) {
	case "success", "succeeded":
		query = query.Where(adapterrequest.And(
			adapterrequest.ErrorMessageIsNil(),
			adapterrequest.Or(
				adapterrequest.StatusCodeIsNil(),
				adapterrequest.StatusCodeLT(400),
			),
		))
	case "failed", "error":
		query = query.Where(adapterRequestFailedPredicate())
	}
	switch strings.ToLower(strings.TrimSpace(filters.Focus)) {
	case "failed", "error":
		query = query.Where(adapterRequestFailedPredicate())
	case "stream", "stream_finalized", "streaming":
		query = query.Where(adapterRequestMetadataValueEQ("stream_usage_finalized", true))
	case "websocket", "ws":
		query = query.Where(adapterrequest.Or(
			adapterRequestMetadataValueEQ("websocket", true),
			adapterRequestMetadataValueEQ("transport", "websocket"),
		))
	}
	return query
}

func adapterRequestFailedPredicate() predicate.AdapterRequest {
	return adapterrequest.Or(
		adapterrequest.ErrorMessageNotNil(),
		adapterrequest.StatusCodeGTE(400),
	)
}

func adapterRequestMetadataValueEQ(key string, value any) predicate.AdapterRequest {
	return predicate.AdapterRequest(func(s *sql.Selector) {
		s.Where(sqljson.ValueEQ(adapterrequest.FieldMetadata, value, sqljson.Path(key)))
	})
}

func adapterRequestToService(record *ent.AdapterRequest) *service.AdapterRequestRecord {
	if record == nil {
		return nil
	}
	return &service.AdapterRequestRecord{
		ID:                record.ID,
		RequestID:         record.RequestID,
		UserID:            record.UserID,
		APIKeyID:          record.APIKeyID,
		GroupID:           int64PtrValue(record.GroupID),
		AdapterProviderID: record.AdapterProviderID,
		Provider:          record.Provider,
		Capability:        record.Capability,
		RouteTarget:       record.RouteTarget,
		Method:            record.Method,
		Path:              record.Path,
		Model:             stringValue(record.Model),
		StatusCode:        intPtrValue(record.StatusCode),
		DurationMS:        intPtrValue(record.DurationMs),
		ErrorMessage:      stringValue(record.ErrorMessage),
		Metadata:          record.Metadata,
		CreatedAt:         record.CreatedAt,
	}
}

func intPtrValue(value *int) *int {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
