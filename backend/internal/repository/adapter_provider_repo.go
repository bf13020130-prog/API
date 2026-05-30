package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/adapterprovider"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adapterProviderRepository struct {
	client *ent.Client
}

func NewAdapterProviderRepository(client *ent.Client) service.AdapterProviderRepository {
	return &adapterProviderRepository{client: client}
}

func (r *adapterProviderRepository) List(ctx context.Context) ([]*service.AdapterProvider, error) {
	providers, err := r.client.AdapterProvider.Query().
		Order(ent.Asc(adapterprovider.FieldPriority), ent.Asc(adapterprovider.FieldSlug)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.AdapterProvider, 0, len(providers))
	for _, provider := range providers {
		out = append(out, r.toService(provider))
	}
	return out, nil
}

func (r *adapterProviderRepository) GetByID(ctx context.Context, id int64) (*service.AdapterProvider, error) {
	provider, err := r.client.AdapterProvider.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrAdapterProviderNotFound
		}
		return nil, err
	}
	return r.toService(provider), nil
}

func (r *adapterProviderRepository) Create(ctx context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	builder := r.client.AdapterProvider.Create().
		SetName(provider.Name).
		SetSlug(provider.Slug).
		SetStatus(provider.Status).
		SetAdapterType(provider.AdapterType).
		SetBaseURL(provider.BaseURL).
		SetCredentials(provider.Credentials).
		SetCapabilities(provider.Capabilities).
		SetPriority(provider.Priority).
		SetTimeoutMs(provider.TimeoutMS).
		SetExtra(provider.Extra)

	if provider.AuthMode != "" {
		builder.SetAuthMode(provider.AuthMode)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, mapAdapterProviderSaveError(err)
	}
	return r.toService(created), nil
}

func (r *adapterProviderRepository) Update(ctx context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	builder := r.client.AdapterProvider.UpdateOneID(provider.ID).
		SetName(provider.Name).
		SetSlug(provider.Slug).
		SetStatus(provider.Status).
		SetAdapterType(provider.AdapterType).
		SetBaseURL(provider.BaseURL).
		SetCredentials(provider.Credentials).
		SetCapabilities(provider.Capabilities).
		SetPriority(provider.Priority).
		SetTimeoutMs(provider.TimeoutMS).
		SetExtra(provider.Extra)

	if provider.AuthMode != "" {
		builder.SetAuthMode(provider.AuthMode)
	} else {
		builder.ClearAuthMode()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrAdapterProviderNotFound
		}
		return nil, mapAdapterProviderSaveError(err)
	}
	return r.toService(updated), nil
}

func (r *adapterProviderRepository) Delete(ctx context.Context, id int64) error {
	err := r.client.AdapterProvider.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return service.ErrAdapterProviderNotFound
		}
		return err
	}
	return nil
}

func (r *adapterProviderRepository) toService(provider *ent.AdapterProvider) *service.AdapterProvider {
	if provider == nil {
		return nil
	}
	authMode := ""
	if provider.AuthMode != nil {
		authMode = *provider.AuthMode
	}
	return &service.AdapterProvider{
		ID:           provider.ID,
		Name:         provider.Name,
		Slug:         provider.Slug,
		Status:       provider.Status,
		AdapterType:  provider.AdapterType,
		BaseURL:      provider.BaseURL,
		AuthMode:     authMode,
		Credentials:  provider.Credentials,
		Capabilities: provider.Capabilities,
		Priority:     provider.Priority,
		TimeoutMS:    provider.TimeoutMs,
		Extra:        provider.Extra,
		CreatedAt:    provider.CreatedAt,
		UpdatedAt:    provider.UpdatedAt,
	}
}

func mapAdapterProviderSaveError(err error) error {
	if ent.IsConstraintError(err) {
		return service.ErrAdapterProviderExists.WithCause(err)
	}
	return err
}
