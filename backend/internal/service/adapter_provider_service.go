package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
)

const (
	AdapterProviderStatusActive   = "active"
	AdapterProviderStatusDisabled = "disabled"
	AdapterProviderTypeNewAPI     = "new-api"
)

var (
	ErrAdapterProviderNotFound = errors.NotFound("ADAPTER_PROVIDER_NOT_FOUND", "adapter provider not found")
	ErrAdapterProviderExists   = errors.Conflict("ADAPTER_PROVIDER_EXISTS", "adapter provider already exists")
)

// AdapterProvider is the service-level representation of a long-tail adapter provider.
type AdapterProvider struct {
	ID           int64
	Name         string
	Slug         string
	Status       string
	AdapterType  string
	BaseURL      string
	AuthMode     string
	Credentials  map[string]string
	Capabilities []string
	Priority     int
	TimeoutMS    int
	Extra        map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (p *AdapterProvider) Clone() *AdapterProvider {
	if p == nil {
		return nil
	}
	out := *p
	out.Credentials = cloneStringMap(p.Credentials)
	out.Capabilities = append([]string(nil), p.Capabilities...)
	out.Extra = cloneAnyMap(p.Extra)
	return &out
}

func (p *AdapterProvider) providerConfig() adapterclient.ProviderConfig {
	if p == nil {
		return adapterclient.ProviderConfig{}
	}
	return adapterclient.ProviderConfig{
		Name:         p.Name,
		Slug:         p.Slug,
		Status:       adapterclient.ProviderStatus(p.Status),
		AdapterType:  p.AdapterType,
		BaseURL:      p.BaseURL,
		AuthMode:     p.AuthMode,
		Credentials:  cloneStringMap(p.Credentials),
		Capabilities: append([]string(nil), p.Capabilities...),
		Priority:     p.Priority,
		TimeoutMS:    p.TimeoutMS,
	}
}

func (p *AdapterProvider) normalizeDefaults() {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	p.Status = strings.ToLower(strings.TrimSpace(p.Status))
	if p.Status == "" {
		p.Status = AdapterProviderStatusDisabled
	}
	p.AdapterType = strings.TrimSpace(p.AdapterType)
	if p.AdapterType == "" {
		p.AdapterType = AdapterProviderTypeNewAPI
	}
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	p.AuthMode = strings.ToLower(strings.TrimSpace(p.AuthMode))
	p.Capabilities = normalizeStringSlice(p.Capabilities)
	if p.TimeoutMS == 0 {
		p.TimeoutMS = 30000
	}
	if p.Priority == 0 {
		p.Priority = 50
	}
	if p.Credentials == nil {
		p.Credentials = map[string]string{}
	}
	if p.Extra == nil {
		p.Extra = map[string]any{}
	}
}

func (p *AdapterProvider) validate() error {
	if p == nil {
		return errors.BadRequest("ADAPTER_PROVIDER_INVALID", "adapter provider is required")
	}
	p.normalizeDefaults()
	if p.AdapterType != AdapterProviderTypeNewAPI {
		return errors.BadRequest("ADAPTER_PROVIDER_INVALID_ADAPTER_TYPE", "adapter provider type must be new-api")
	}
	if err := p.providerConfig().Validate(); err != nil {
		return errors.BadRequest("ADAPTER_PROVIDER_INVALID", err.Error())
	}
	return nil
}

// AdapterProviderUpdate carries a partial-update request. Nil Credentials means keep existing values.
type AdapterProviderUpdate struct {
	ID           int64
	Name         string
	Slug         string
	Status       string
	AdapterType  string
	BaseURL      string
	AuthMode     string
	Credentials  *map[string]string
	Capabilities []string
	Priority     int
	TimeoutMS    int
	Extra        map[string]any
}

type AdapterProviderSafeView struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Slug           string         `json:"slug"`
	Status         string         `json:"status"`
	AdapterType    string         `json:"adapter_type"`
	BaseURL        string         `json:"base_url"`
	AuthMode       string         `json:"auth_mode,omitempty"`
	Capabilities   []string       `json:"capabilities"`
	Priority       int            `json:"priority"`
	TimeoutMS      int            `json:"timeout_ms"`
	Extra          map[string]any `json:"extra,omitempty"`
	HasCredentials bool           `json:"has_credentials"`
	CredentialKeys []string       `json:"credential_keys"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// AdapterProviderRepository defines DB access for long-tail adapter providers.
type AdapterProviderRepository interface {
	List(ctx context.Context) ([]*AdapterProvider, error)
	GetByID(ctx context.Context, id int64) (*AdapterProvider, error)
	Create(ctx context.Context, provider *AdapterProvider) (*AdapterProvider, error)
	Update(ctx context.Context, provider *AdapterProvider) (*AdapterProvider, error)
	Delete(ctx context.Context, id int64) error
}

type AdapterProviderService struct {
	repo AdapterProviderRepository
}

func NewAdapterProviderService(repo AdapterProviderRepository) *AdapterProviderService {
	return &AdapterProviderService{repo: repo}
}

func (s *AdapterProviderService) List(ctx context.Context) ([]*AdapterProvider, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	providers, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, provider := range providers {
		provider.normalizeDefaults()
	}
	return providers, nil
}

func (s *AdapterProviderService) ListSafe(ctx context.Context) ([]AdapterProviderSafeView, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdapterProviderSafeView, 0, len(providers))
	for _, provider := range providers {
		out = append(out, provider.SafeView())
	}
	return out, nil
}

func (s *AdapterProviderService) GetByID(ctx context.Context, id int64) (*AdapterProvider, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAdapterProviderNotFound
	}
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	provider.normalizeDefaults()
	return provider, nil
}

func (s *AdapterProviderService) GetSafeByID(ctx context.Context, id int64) (*AdapterProviderSafeView, error) {
	provider, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := provider.SafeView()
	return &view, nil
}

func (s *AdapterProviderService) Create(ctx context.Context, provider *AdapterProvider) (*AdapterProvider, error) {
	if err := provider.validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, provider)
	if err != nil {
		return nil, err
	}
	created.normalizeDefaults()
	return created, nil
}

func (s *AdapterProviderService) Update(ctx context.Context, update *AdapterProviderUpdate) (*AdapterProvider, error) {
	if update == nil || update.ID <= 0 {
		return nil, errors.BadRequest("ADAPTER_PROVIDER_INVALID", "adapter provider id is required")
	}
	existing, err := s.GetByID(ctx, update.ID)
	if err != nil {
		return nil, err
	}
	next := &AdapterProvider{
		ID:           update.ID,
		Name:         update.Name,
		Slug:         update.Slug,
		Status:       update.Status,
		AdapterType:  update.AdapterType,
		BaseURL:      update.BaseURL,
		AuthMode:     update.AuthMode,
		Credentials:  existing.Credentials,
		Capabilities: update.Capabilities,
		Priority:     update.Priority,
		TimeoutMS:    update.TimeoutMS,
		Extra:        update.Extra,
		CreatedAt:    existing.CreatedAt,
		UpdatedAt:    existing.UpdatedAt,
	}
	if update.Credentials != nil {
		next.Credentials = cloneStringMap(*update.Credentials)
	}
	if err := next.validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, next)
	if err != nil {
		return nil, err
	}
	updated.normalizeDefaults()
	return updated, nil
}

func (s *AdapterProviderService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrAdapterProviderNotFound
	}
	return s.repo.Delete(ctx, id)
}

func (s *AdapterProviderService) ProviderDiagnostics(ctx context.Context) (adapterclient.ProviderDiagnostics, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return adapterclient.ProviderDiagnostics{
			ObserveOnly:        true,
			EnforcementEnabled: false,
		}, err
	}
	configs := adapterProvidersToConfigs(providers)
	return adapterclient.ProviderDiagnostics{
		ObserveOnly:        true,
		EnforcementEnabled: false,
		ActiveSlugs:        adapterclient.ActiveProviderSlugs(configs),
		Providers:          adapterclient.BuildProviderDiagnostics(configs),
	}, nil
}

// Providers implements adapterclient.ProviderRegistry for runtime components that cannot carry a request context.
func (s *AdapterProviderService) Providers() []adapterclient.ProviderConfig {
	providers, err := s.List(context.Background())
	if err != nil {
		return nil
	}
	return adapterProvidersToConfigs(providers)
}

// Diagnostics implements adapterclient.ProviderRegistry for runtime components that cannot carry a request context.
func (s *AdapterProviderService) Diagnostics() adapterclient.ProviderDiagnostics {
	diagnostics, err := s.ProviderDiagnostics(context.Background())
	if err != nil {
		return adapterclient.ProviderDiagnostics{
			ObserveOnly:        true,
			EnforcementEnabled: false,
		}
	}
	return diagnostics
}

func (p *AdapterProvider) SafeView() AdapterProviderSafeView {
	if p == nil {
		return AdapterProviderSafeView{}
	}
	p.normalizeDefaults()
	keys := credentialKeys(p.Credentials)
	return AdapterProviderSafeView{
		ID:             p.ID,
		Name:           p.Name,
		Slug:           p.Slug,
		Status:         p.Status,
		AdapterType:    p.AdapterType,
		BaseURL:        p.BaseURL,
		AuthMode:       p.AuthMode,
		Capabilities:   append([]string(nil), p.Capabilities...),
		Priority:       p.Priority,
		TimeoutMS:      p.TimeoutMS,
		Extra:          cloneAnyMap(p.Extra),
		HasCredentials: len(keys) > 0,
		CredentialKeys: keys,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func adapterProvidersToConfigs(providers []*AdapterProvider) []adapterclient.ProviderConfig {
	configs := make([]adapterclient.ProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		configs = append(configs, provider.providerConfig())
	}
	return configs
}

func credentialKeys(credentials map[string]string) []string {
	keys := make([]string, 0, len(credentials))
	for key := range credentials {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
