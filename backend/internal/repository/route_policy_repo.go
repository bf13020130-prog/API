package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/routepolicy"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type routePolicyRepository struct {
	client *ent.Client
}

func NewRoutePolicyRepository(client *ent.Client) service.RoutePolicyRepository {
	return &routePolicyRepository{client: client}
}

func (r *routePolicyRepository) List(ctx context.Context) ([]*service.RoutePolicy, error) {
	policies, err := r.client.RoutePolicy.Query().
		Order(ent.Asc(routepolicy.FieldPriority), ent.Asc(routepolicy.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.RoutePolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, r.toService(policy))
	}
	return out, nil
}

func (r *routePolicyRepository) GetByID(ctx context.Context, id int64) (*service.RoutePolicy, error) {
	policy, err := r.client.RoutePolicy.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrRoutePolicyNotFound
		}
		return nil, err
	}
	return r.toService(policy), nil
}

func (r *routePolicyRepository) Create(ctx context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	builder := r.client.RoutePolicy.Create().
		SetName(policy.Name).
		SetStatus(policy.Status).
		SetTarget(policy.Target).
		SetPriority(policy.Priority).
		SetConditions(policy.Conditions)

	applyRoutePolicyCreateOptional(builder, policy)

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toService(created), nil
}

func (r *routePolicyRepository) Update(ctx context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	builder := r.client.RoutePolicy.UpdateOneID(policy.ID).
		SetName(policy.Name).
		SetStatus(policy.Status).
		SetTarget(policy.Target).
		SetPriority(policy.Priority).
		SetConditions(policy.Conditions)

	applyRoutePolicyUpdateOptional(builder, policy)

	updated, err := builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrRoutePolicyNotFound
		}
		return nil, err
	}
	return r.toService(updated), nil
}

func (r *routePolicyRepository) Delete(ctx context.Context, id int64) error {
	err := r.client.RoutePolicy.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return service.ErrRoutePolicyNotFound
		}
		return err
	}
	return nil
}

func (r *routePolicyRepository) toService(policy *ent.RoutePolicy) *service.RoutePolicy {
	if policy == nil {
		return nil
	}
	return &service.RoutePolicy{
		ID:                 policy.ID,
		Name:               policy.Name,
		Status:             policy.Status,
		MatchMethod:        stringValue(policy.MatchMethod),
		MatchPath:          stringValue(policy.MatchPath),
		MatchModel:         stringValue(policy.MatchModel),
		MatchCapability:    stringValue(policy.MatchCapability),
		MatchGroupPlatform: stringValue(policy.MatchGroupPlatform),
		Target:             policy.Target,
		Platform:           stringValue(policy.Platform),
		AdapterProviderID:  int64PtrValue(policy.AdapterProviderID),
		Priority:           policy.Priority,
		Conditions:         policy.Conditions,
		Description:        stringValue(policy.Description),
		CreatedAt:          policy.CreatedAt,
		UpdatedAt:          policy.UpdatedAt,
	}
}

func applyRoutePolicyCreateOptional(builder *ent.RoutePolicyCreate, policy *service.RoutePolicy) {
	if policy.MatchMethod != "" {
		builder.SetMatchMethod(policy.MatchMethod)
	}
	if policy.MatchPath != "" {
		builder.SetMatchPath(policy.MatchPath)
	}
	if policy.MatchModel != "" {
		builder.SetMatchModel(policy.MatchModel)
	}
	if policy.MatchCapability != "" {
		builder.SetMatchCapability(policy.MatchCapability)
	}
	if policy.MatchGroupPlatform != "" {
		builder.SetMatchGroupPlatform(policy.MatchGroupPlatform)
	}
	if policy.Platform != "" {
		builder.SetPlatform(policy.Platform)
	}
	if policy.AdapterProviderID != nil {
		builder.SetAdapterProviderID(*policy.AdapterProviderID)
	}
	if policy.Description != "" {
		builder.SetDescription(policy.Description)
	}
}

func applyRoutePolicyUpdateOptional(builder *ent.RoutePolicyUpdateOne, policy *service.RoutePolicy) {
	if policy.MatchMethod != "" {
		builder.SetMatchMethod(policy.MatchMethod)
	} else {
		builder.ClearMatchMethod()
	}
	if policy.MatchPath != "" {
		builder.SetMatchPath(policy.MatchPath)
	} else {
		builder.ClearMatchPath()
	}
	if policy.MatchModel != "" {
		builder.SetMatchModel(policy.MatchModel)
	} else {
		builder.ClearMatchModel()
	}
	if policy.MatchCapability != "" {
		builder.SetMatchCapability(policy.MatchCapability)
	} else {
		builder.ClearMatchCapability()
	}
	if policy.MatchGroupPlatform != "" {
		builder.SetMatchGroupPlatform(policy.MatchGroupPlatform)
	} else {
		builder.ClearMatchGroupPlatform()
	}
	if policy.Platform != "" {
		builder.SetPlatform(policy.Platform)
	} else {
		builder.ClearPlatform()
	}
	if policy.AdapterProviderID != nil {
		builder.SetAdapterProviderID(*policy.AdapterProviderID)
	} else {
		builder.ClearAdapterProviderID()
	}
	if policy.Description != "" {
		builder.SetDescription(policy.Description)
	} else {
		builder.ClearDescription()
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int64PtrValue(value *int64) *int64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
