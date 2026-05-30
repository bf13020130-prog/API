package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
)

const (
	RoutePolicyStatusActive   = "active"
	RoutePolicyStatusDisabled = "disabled"
)

var (
	ErrRoutePolicyNotFound = errors.NotFound("ROUTE_POLICY_NOT_FOUND", "route policy not found")
)

type RoutePolicy struct {
	ID                 int64
	Name               string
	Status             string
	MatchMethod        string
	MatchPath          string
	MatchModel         string
	MatchCapability    string
	MatchGroupPlatform string
	Target             string
	Platform           string
	AdapterProviderID  *int64
	Priority           int
	Conditions         map[string]any
	Description        string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (p *RoutePolicy) Clone() *RoutePolicy {
	if p == nil {
		return nil
	}
	out := *p
	if p.AdapterProviderID != nil {
		id := *p.AdapterProviderID
		out.AdapterProviderID = &id
	}
	out.Conditions = cloneAnyMap(p.Conditions)
	return &out
}

type RoutePolicySafeView struct {
	ID                 int64          `json:"id"`
	Name               string         `json:"name"`
	Status             string         `json:"status"`
	MatchMethod        string         `json:"match_method,omitempty"`
	MatchPath          string         `json:"match_path,omitempty"`
	MatchModel         string         `json:"match_model,omitempty"`
	MatchCapability    string         `json:"match_capability,omitempty"`
	MatchGroupPlatform string         `json:"match_group_platform,omitempty"`
	Target             string         `json:"target"`
	Platform           string         `json:"platform,omitempty"`
	AdapterProviderID  *int64         `json:"adapter_provider_id,omitempty"`
	Priority           int            `json:"priority"`
	Conditions         map[string]any `json:"conditions,omitempty"`
	Description        string         `json:"description,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type RoutePolicyRepository interface {
	List(ctx context.Context) ([]*RoutePolicy, error)
	GetByID(ctx context.Context, id int64) (*RoutePolicy, error)
	Create(ctx context.Context, policy *RoutePolicy) (*RoutePolicy, error)
	Update(ctx context.Context, policy *RoutePolicy) (*RoutePolicy, error)
	Delete(ctx context.Context, id int64) error
}

type RoutePolicyService struct {
	repo               RoutePolicyRepository
	adapterProviderSvc *AdapterProviderService
}

func NewRoutePolicyService(repo RoutePolicyRepository, adapterProviderSvc *AdapterProviderService) *RoutePolicyService {
	return &RoutePolicyService{repo: repo, adapterProviderSvc: adapterProviderSvc}
}

func (s *RoutePolicyService) List(ctx context.Context) ([]*RoutePolicy, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	policies, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range policies {
		policy.normalizeDefaults()
	}
	return policies, nil
}

func (s *RoutePolicyService) ListSafe(ctx context.Context) ([]RoutePolicySafeView, error) {
	policies, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RoutePolicySafeView, 0, len(policies))
	for _, policy := range policies {
		out = append(out, policy.SafeView())
	}
	return out, nil
}

func (s *RoutePolicyService) GetByID(ctx context.Context, id int64) (*RoutePolicy, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRoutePolicyNotFound
	}
	policy, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	policy.normalizeDefaults()
	return policy, nil
}

func (s *RoutePolicyService) GetSafeByID(ctx context.Context, id int64) (*RoutePolicySafeView, error) {
	policy, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := policy.SafeView()
	return &view, nil
}

func (s *RoutePolicyService) Create(ctx context.Context, policy *RoutePolicy) (*RoutePolicy, error) {
	if err := s.validate(ctx, policy); err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, policy)
	if err != nil {
		return nil, err
	}
	created.normalizeDefaults()
	return created, nil
}

func (s *RoutePolicyService) Update(ctx context.Context, policy *RoutePolicy) (*RoutePolicy, error) {
	if policy == nil || policy.ID <= 0 {
		return nil, errors.BadRequest("ROUTE_POLICY_INVALID", "route policy id is required")
	}
	if _, err := s.GetByID(ctx, policy.ID); err != nil {
		return nil, err
	}
	if err := s.validate(ctx, policy); err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, policy)
	if err != nil {
		return nil, err
	}
	updated.normalizeDefaults()
	return updated, nil
}

func (s *RoutePolicyService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return ErrRoutePolicyNotFound
	}
	return s.repo.Delete(ctx, id)
}

func (s *RoutePolicyService) validate(ctx context.Context, policy *RoutePolicy) error {
	if policy == nil {
		return errors.BadRequest("ROUTE_POLICY_INVALID", "route policy is required")
	}
	policy.normalizeDefaults()
	if policy.Name == "" {
		return errors.BadRequest("ROUTE_POLICY_INVALID", "route policy name is required")
	}
	switch policy.Status {
	case RoutePolicyStatusActive, RoutePolicyStatusDisabled:
	default:
		return errors.BadRequest("ROUTE_POLICY_INVALID_STATUS", "route policy status must be active or disabled")
	}
	switch capabilityrouter.Target(policy.Target) {
	case capabilityrouter.TargetSub2APINative, capabilityrouter.TargetSub2APIUpstream, capabilityrouter.TargetNewAPIAdapter, capabilityrouter.TargetUnsupported:
	default:
		return errors.BadRequest("ROUTE_POLICY_INVALID_TARGET", "route policy target is invalid")
	}
	if capabilityrouter.Target(policy.Target) == capabilityrouter.TargetNewAPIAdapter {
		if isCorePlatform(policy.MatchGroupPlatform) {
			return errors.BadRequest("ROUTE_POLICY_CORE_ADAPTER_FORBIDDEN", "core provider platform cannot route to adapter")
		}
		if policy.AdapterProviderID == nil || *policy.AdapterProviderID <= 0 {
			return errors.BadRequest("ROUTE_POLICY_ADAPTER_PROVIDER_REQUIRED", "adapter provider id is required for new_api_adapter route policies")
		}
		if s == nil || s.adapterProviderSvc == nil {
			return errors.BadRequest("ROUTE_POLICY_ADAPTER_PROVIDER_REQUIRED", "adapter provider service is required")
		}
		provider, err := s.adapterProviderSvc.GetByID(ctx, *policy.AdapterProviderID)
		if err != nil {
			return err
		}
		if err := provider.validate(); err != nil {
			return err
		}
		return nil
	}
	policy.AdapterProviderID = nil
	return nil
}

func (p *RoutePolicy) normalizeDefaults() {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Status = strings.ToLower(strings.TrimSpace(p.Status))
	if p.Status == "" {
		p.Status = RoutePolicyStatusDisabled
	}
	p.MatchMethod = strings.ToUpper(strings.TrimSpace(p.MatchMethod))
	p.MatchPath = normalizePolicyPath(p.MatchPath)
	p.MatchModel = strings.ToLower(strings.TrimSpace(p.MatchModel))
	p.MatchCapability = strings.ToLower(strings.TrimSpace(p.MatchCapability))
	p.MatchGroupPlatform = normalizePolicyPlatform(p.MatchGroupPlatform)
	p.Target = strings.ToLower(strings.TrimSpace(p.Target))
	p.Platform = normalizePolicyPlatform(p.Platform)
	if p.Priority == 0 {
		p.Priority = 50
	}
	if p.Conditions == nil {
		p.Conditions = map[string]any{}
	}
	p.Description = strings.TrimSpace(p.Description)
}

func (p *RoutePolicy) SafeView() RoutePolicySafeView {
	if p == nil {
		return RoutePolicySafeView{}
	}
	p.normalizeDefaults()
	return RoutePolicySafeView{
		ID:                 p.ID,
		Name:               p.Name,
		Status:             p.Status,
		MatchMethod:        p.MatchMethod,
		MatchPath:          p.MatchPath,
		MatchModel:         p.MatchModel,
		MatchCapability:    p.MatchCapability,
		MatchGroupPlatform: p.MatchGroupPlatform,
		Target:             p.Target,
		Platform:           p.Platform,
		AdapterProviderID:  p.AdapterProviderID,
		Priority:           p.Priority,
		Conditions:         cloneAnyMap(p.Conditions),
		Description:        p.Description,
		CreatedAt:          p.CreatedAt,
		UpdatedAt:          p.UpdatedAt,
	}
}

func normalizePolicyPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.ToLower(strings.TrimRight(path, "/"))
}

func normalizePolicyPlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "claude" {
		return domain.PlatformAnthropic
	}
	return platform
}

func isCorePlatform(platform string) bool {
	switch normalizePolicyPlatform(platform) {
	case domain.PlatformOpenAI, domain.PlatformAnthropic, domain.PlatformGemini, domain.PlatformAntigravity:
		return true
	default:
		return false
	}
}
