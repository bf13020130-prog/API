package service

import (
	"context"
	"sort"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	"github.com/stretchr/testify/require"
)

func TestRoutePolicyServiceDefaultsToDisabledAndKeepsLongTailAdapterExplicit(t *testing.T) {
	providers := newMemoryAdapterProviderRepo()
	providers.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	svc := NewRoutePolicyService(newMemoryRoutePolicyRepo(), NewAdapterProviderService(providers))

	created, err := svc.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney image jobs",
		MatchMethod:       "post",
		MatchPath:         "/v1/images/generations",
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})

	require.NoError(t, err)
	require.Equal(t, RoutePolicyStatusDisabled, created.Status)
	require.Equal(t, "POST", created.MatchMethod)
	require.Equal(t, "/v1/images/generations", created.MatchPath)
	require.Equal(t, 50, created.Priority)
	require.Equal(t, int64(1), *created.AdapterProviderID)
}

func TestRoutePolicyServiceRejectsAdapterTargetWithoutValidProvider(t *testing.T) {
	providers := newMemoryAdapterProviderRepo()
	providers.providers[2] = &AdapterProvider{
		ID:           2,
		Name:         "OpenAI adapter should be rejected",
		Slug:         "openai",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	svc := NewRoutePolicyService(newMemoryRoutePolicyRepo(), NewAdapterProviderService(providers))

	_, err := svc.Create(context.Background(), &RoutePolicy{
		Name:   "Missing provider",
		Target: string(capabilityrouter.TargetNewAPIAdapter),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter provider id is required")

	_, err = svc.Create(context.Background(), &RoutePolicy{
		Name:              "Core provider adapter",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(2),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "core provider slug")
}

func TestRoutePolicyServiceRejectsCoreProvidersAsAdapterPolicyMatches(t *testing.T) {
	providers := newMemoryAdapterProviderRepo()
	providers.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	svc := NewRoutePolicyService(newMemoryRoutePolicyRepo(), NewAdapterProviderService(providers))

	_, err := svc.Create(context.Background(), &RoutePolicy{
		Name:              "OpenAI must remain native",
		MatchGroupPlatform: "openai",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "core provider platform cannot route to adapter")
}

type memoryRoutePolicyRepo struct {
	nextID   int64
	policies map[int64]*RoutePolicy
}

func newMemoryRoutePolicyRepo() *memoryRoutePolicyRepo {
	return &memoryRoutePolicyRepo{
		nextID:   1,
		policies: make(map[int64]*RoutePolicy),
	}
}

func (r *memoryRoutePolicyRepo) List(context.Context) ([]*RoutePolicy, error) {
	ids := make([]int64, 0, len(r.policies))
	for id := range r.policies {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*RoutePolicy, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.policies[id].Clone())
	}
	return out, nil
}

func (r *memoryRoutePolicyRepo) GetByID(_ context.Context, id int64) (*RoutePolicy, error) {
	policy, ok := r.policies[id]
	if !ok {
		return nil, ErrRoutePolicyNotFound
	}
	return policy.Clone(), nil
}

func (r *memoryRoutePolicyRepo) Create(_ context.Context, policy *RoutePolicy) (*RoutePolicy, error) {
	created := policy.Clone()
	created.ID = r.nextID
	r.nextID++
	r.policies[created.ID] = created.Clone()
	return created, nil
}

func (r *memoryRoutePolicyRepo) Update(_ context.Context, policy *RoutePolicy) (*RoutePolicy, error) {
	if _, ok := r.policies[policy.ID]; !ok {
		return nil, ErrRoutePolicyNotFound
	}
	updated := policy.Clone()
	r.policies[policy.ID] = updated.Clone()
	return updated, nil
}

func (r *memoryRoutePolicyRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.policies[id]; !ok {
		return ErrRoutePolicyNotFound
	}
	delete(r.policies, id)
	return nil
}

func ptrInt64(v int64) *int64 {
	return &v
}
