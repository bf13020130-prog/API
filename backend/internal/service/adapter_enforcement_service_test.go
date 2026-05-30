package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestAdapterEnforcementDisabledDoesNotCallAdapter(t *testing.T) {
	client := adapterclient.NewFakeClient(adapterclient.Response{Status: adapterclient.StatusSucceeded})
	svc := NewAdapterEnforcementService(AdapterEnforcementConfig{Enabled: false}, nil, nil, nil, client)

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-disabled",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Payload:       []byte(`{"model":"mj-v6"}`),
	})

	require.NoError(t, err)
	require.False(t, result.Handled)
	require.Empty(t, client.Requests())
}

func TestAdapterEnforcementRoutesActiveLongTailPolicyAndRecordsAudit(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:               "Midjourney images",
		Status:             RoutePolicyStatusActive,
		MatchMethod:        "POST",
		MatchPath:          "/v1/images/generations",
		MatchCapability:    "image_generation",
		MatchGroupPlatform: "midjourney",
		Target:             string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID:  ptrInt64(1),
		Priority:           10,
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: 201,
		Body:          []byte(`{"id":"adapter-ok"}`),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-1",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "post",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Model:         "mj-v6",
		Headers:       map[string]string{"content-type": "application/json"},
		Payload:       []byte(`{"model":"mj-v6"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	require.Equal(t, 201, result.Response.AdapterStatus)
	require.Equal(t, int64(1), result.Provider.ID)
	require.Equal(t, int64(1), result.Policy.ID)

	requests := client.Requests()
	require.Len(t, requests, 1)
	require.Equal(t, "req-1", requests[0].RequestID)
	require.Equal(t, int64(10), requests[0].UserID)
	require.Equal(t, int64(20), requests[0].APIKeyID)
	require.Equal(t, int64(30), requests[0].GroupID)
	require.Equal(t, "midjourney", requests[0].Provider)
	require.Equal(t, "image_generation", requests[0].Capability)
	require.Equal(t, capabilityrouter.TargetNewAPIAdapter, requests[0].RouteTarget)
	require.Equal(t, []byte(`{"model":"mj-v6"}`), requests[0].Payload)

	records := auditRepo.records
	require.Len(t, records, 1)
	require.Equal(t, "req-1", records[0].RequestID)
	require.Equal(t, int64(1), records[0].AdapterProviderID)
	require.Equal(t, "midjourney", records[0].Provider)
	require.Equal(t, 201, *records[0].StatusCode)
	require.Empty(t, records[0].ErrorMessage)
}

func TestAdapterEnforcementDiagnosticSamplingRequiresExplicitMatch(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		Body:          []byte(`{"id":"ok"}`),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled: true,
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	_, err = svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-not-sampled",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Model:         "mj-v6",
		Headers:       map[string]string{"Authorization": "Bearer should-not-appear"},
		Payload:       []byte(`{"prompt":"should not appear"}`),
	})

	require.NoError(t, err)
	require.Len(t, auditRepo.records, 1)
	require.NotContains(t, auditRepo.records[0].Metadata, "diagnostic")
}

func TestAdapterEnforcementDiagnosticRecordsFailuresByDefault(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusFailed,
		AdapterStatus: http.StatusTooManyRequests,
		ErrorMessage:  "provider says quota exhausted for Bearer provider-secret",
		Body:          []byte(`{"error":{"message":"quota exhausted","token":"provider-secret"}}`),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	_, err = svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-default-failure-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Model:         "mj-v6",
		Headers:       map[string]string{"Authorization": "Bearer user-secret"},
		Payload:       []byte(`{"model":"mj-v6","prompt":"private failing prompt","api_key":"sk-live-user-secret"}`),
	})

	require.NoError(t, err)
	require.Len(t, auditRepo.records, 1)
	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	request := requireNestedMap(t, diagnostic, "request")
	require.Equal(t, "req-default-failure-diag", request["request_id"])
	require.Equal(t, "midjourney", request["provider"])
	errDiag := requireNestedMap(t, diagnostic, "error")
	require.Equal(t, "adapter_response_error", errDiag["root_cause"])
	require.Equal(t, float64(http.StatusTooManyRequests), errDiag["adapter_status"])
	raw := mustJSON(t, diagnostic)
	require.NotContains(t, raw, "user-secret")
	require.NotContains(t, raw, "provider-secret")
	require.NotContains(t, raw, "private failing prompt")
	require.Contains(t, raw, `"model":"mj-v6"`)
}

func TestAdapterEnforcementDiagnosticSamplingCapturesRedactedRequestAndResponseFailure(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusFailed,
		AdapterStatus: http.StatusBadGateway,
		ErrorMessage:  "upstream returned 502 for Bearer sk-live-secret-token",
		Body:          []byte(`{"error":{"message":"quota exhausted","access_token":"provider-secret"}}`),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled:         true,
				Providers:       []string{"midjourney"},
				MaxPayloadBytes: 256,
				MaxStringBytes:  64,
				MaxEvents:       2,
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	_, err = svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-provider-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Model:         "mj-v6",
		Headers: map[string]string{
			"Authorization": "Bearer user-secret",
			"Cookie":        "session=secret",
			"Content-Type":  "application/json",
		},
		Payload: []byte(`{"model":"mj-v6","prompt":"full user prompt must not be stored","api_key":"sk-live-user-secret","messages":[{"role":"user","content":"private user content"}]}`),
	})

	require.NoError(t, err)
	require.Len(t, auditRepo.records, 1)
	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	request := requireNestedMap(t, diagnostic, "request")
	require.Equal(t, "POST", request["method"])
	require.Equal(t, "/v1/images/generations", request["path"])
	require.Equal(t, "mj-v6", request["model"])
	require.Equal(t, "image_generation", request["capability"])
	require.Equal(t, "midjourney", request["provider"])
	require.Equal(t, "req-provider-diag", request["request_id"])
	headers := requireNestedMap(t, request, "headers")
	require.Equal(t, "[redacted]", headers["Authorization"])
	require.Equal(t, "[redacted]", headers["Cookie"])
	require.Equal(t, "application/json", headers["Content-Type"])
	errDiag := requireNestedMap(t, diagnostic, "error")
	require.Equal(t, "adapter_response_error", errDiag["root_cause"])
	require.Equal(t, float64(http.StatusBadGateway), errDiag["adapter_status"])

	raw := mustJSON(t, diagnostic)
	require.NotContains(t, raw, "user-secret")
	require.NotContains(t, raw, "session=secret")
	require.NotContains(t, raw, "full user prompt")
	require.NotContains(t, raw, "private user content")
	require.NotContains(t, raw, "provider-secret")
	require.Contains(t, raw, `"model":"mj-v6"`)
}

func TestAdapterEnforcementDiagnosticSamplingRedactsOversizedJSONPayloadBeforeTruncating(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		Body:          []byte(`{"id":"ok"}`),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled:         true,
				Providers:       []string{"midjourney"},
				MaxPayloadBytes: 48,
				MaxStringBytes:  32,
				MaxEvents:       2,
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	_, err = svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-oversized-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		Model:         "mj-v6",
		Payload:       []byte(`{"prompt":"oversized private prompt should never be logged","model":"mj-v6","api_key":"sk-live-user-secret","metadata":{"safe":"shape-only"}}`),
	})

	require.NoError(t, err)
	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	request := requireNestedMap(t, diagnostic, "request")
	payload := requireNestedMap(t, request, "payload")
	require.Equal(t, true, payload["truncated"])
	requireContainsPath(t, payload, "json_key_paths", "prompt")
	raw := mustJSON(t, diagnostic)
	require.NotContains(t, raw, "oversized private prompt")
	require.NotContains(t, raw, "sk-live-user-secret")
	require.Contains(t, raw, `"prompt":"[redacted]"`)
	require.Contains(t, raw, `"model":"mj-v6"`)
}

func TestAdapterEnforcementDiagnosticSamplingMatchesRequestIDAndCapturesCallFailure(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "claude-adapter")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Claude long-tail",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := &errorAdapterClient{err: errors.New("dial tcp 10.0.0.8:443: connection refused")}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled:    true,
				RequestIDs: []string{"req-call-diag"},
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-call-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "claude-adapter",
		Method:        "POST",
		Path:          "/v1/messages",
		Capability:    "chat",
		Model:         "claude-longtail",
		Headers:       map[string]string{"Authorization": "Bearer user-secret"},
		Payload:       []byte(`{"model":"claude-longtail","messages":[{"role":"user","content":"private text"}]}`),
	})

	require.Error(t, err)
	require.True(t, result.Handled)
	require.Len(t, auditRepo.records, 1)
	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	errDiag := requireNestedMap(t, diagnostic, "error")
	require.Equal(t, "adapter_call_error", errDiag["root_cause"])
	require.Contains(t, errDiag["message"], "connection refused")
	require.NotContains(t, mustJSON(t, diagnostic), "user-secret")
	require.NotContains(t, mustJSON(t, diagnostic), "private text")
}

func TestAdapterEnforcementOpensWebSocketTunnelForLongTailPolicy(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "realtime-adapter")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Realtime adapter",
		Status:            RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	tunnel := &fakeAdapterWSTunnel{}
	wsClient := &fakeWebSocketAdapterClient{tunnel: tunnel}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		wsClient,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.EnforceWebSocket(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-ws",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "realtime-adapter",
		Method:        "GET",
		Path:          "/v1/responses",
		Capability:    "chat",
		Model:         "longtail-realtime",
		Headers:       map[string]string{"Upgrade": "websocket"},
		APIKey:        &APIKey{ID: 20, UserID: 10},
		User:          &User{ID: 10},
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	require.NotNil(t, result.Tunnel)
	require.NoError(t, result.Tunnel.Write(context.Background(), coderws.MessageText, []byte("client-ping")))
	require.Equal(t, []byte("client-ping"), tunnel.writes[0])
	require.Len(t, wsClient.wsRequests, 1)
	require.Equal(t, "req-ws", wsClient.wsRequests[0].RequestID)
	require.Equal(t, "realtime-adapter", wsClient.wsRequests[0].Provider)
	require.Equal(t, "chat", wsClient.wsRequests[0].Capability)
	require.Equal(t, "GET", wsClient.wsRequests[0].Method)
	require.Equal(t, "/v1/responses", wsClient.wsRequests[0].Path)
	require.Empty(t, wsClient.httpRequests)
	require.Len(t, auditRepo.records, 1)
	require.Equal(t, "pending", auditRepo.records[0].Metadata["status"])
	require.Equal(t, true, auditRepo.records[0].Metadata["websocket"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, "pending", usageRepo.records[0].Status)
	require.Equal(t, true, usageRepo.records[0].Metadata["websocket"])
}

func TestAdapterEnforcementWebSocketNeverRoutesCoreGroupPlatformToAdapter(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "realtime-adapter")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Realtime adapter",
		Status:            RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	wsClient := &fakeWebSocketAdapterClient{tunnel: &fakeAdapterWSTunnel{}}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(newMemoryAdapterRequestRepo()),
		wsClient,
	)

	result, err := svc.EnforceWebSocket(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-core-ws",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "openai",
		Method:        "GET",
		Path:          "/v1/responses",
		Capability:    "chat",
	})

	require.NoError(t, err)
	require.False(t, result.Handled)
	require.Empty(t, wsClient.wsRequests)
}

func TestAdapterEnforcementFinalizesWebSocketUsageFromResponseCompletedEvent(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "realtime-adapter")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Realtime adapter",
		Status:            RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	wsClient := &fakeWebSocketAdapterClient{tunnel: &fakeAdapterWSTunnel{
		readType:    coderws.MessageText,
		readPayload: []byte(`{"type":"response.completed","response":{"id":"resp_ws","usage":{"input_tokens":7,"output_tokens":3,"cost_usd":0.02}}}`),
	}}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		wsClient,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.EnforceWebSocket(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-ws-final-usage",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "realtime-adapter",
		Method:             "GET",
		Path:               "/v1/responses",
		Capability:         "chat",
		Model:              "longtail-realtime",
		Headers:            map[string]string{"Upgrade": "websocket"},
		APIKey:             &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10},
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"longtail-realtime"}`)),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	msgType, payload, err := result.Tunnel.Read(context.Background())
	require.NoError(t, err)
	require.Equal(t, coderws.MessageText, msgType)
	require.Contains(t, string(payload), "response.completed")
	require.NoError(t, result.Tunnel.Close())

	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd)
	require.Equal(t, "req-ws-final-usage", billingRepo.lastCmd.RequestID)
	require.Equal(t, 7, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 3, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.02, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.Len(t, auditRepo.records, 1)
	require.Equal(t, true, auditRepo.records[0].Metadata["websocket_usage_finalized"])
	require.Equal(t, "websocket_event", auditRepo.records[0].Metadata["usage_source"])
	require.InDelta(t, 0.02, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, "succeeded", usageRepo.records[0].Status)
	require.Equal(t, 7, usageRepo.records[0].InputUnits)
	require.Equal(t, 3, usageRepo.records[0].OutputUnits)
	require.InDelta(t, 0.02, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, true, usageRepo.records[0].Metadata["websocket_usage_finalized"])
	require.Equal(t, "websocket_event", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementDiagnosticSamplingCapturesWebSocketEventShape(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "realtime-adapter")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Realtime adapter",
		Status:            RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	wsClient := &fakeWebSocketAdapterClient{tunnel: &fakeAdapterWSTunnel{
		readType:    coderws.MessageText,
		readPayload: []byte(`{"type":"response.completed","response":{"output":[{"content":[{"text":"secret ws answer"}]}],"usage":{"input_tokens":7,"output_tokens":3,"cost_usd":0.02}}}`),
	}}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled:         true,
				RequestIDs:      []string{"req-ws-diag"},
				MaxPayloadBytes: 512,
				MaxStringBytes:  64,
				MaxEvents:       2,
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		wsClient,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.EnforceWebSocket(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-ws-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "realtime-adapter",
		Method:        "GET",
		Path:          "/v1/responses",
		Capability:    "chat",
		Model:         "longtail-realtime",
		Headers:       map[string]string{"Upgrade": "websocket", "Authorization": "Bearer ws-secret"},
		APIKey:        &APIKey{ID: 20, UserID: 10, Quota: 100},
		User:          &User{ID: 10},
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	_, _, err = result.Tunnel.Read(context.Background())
	require.NoError(t, err)
	require.NoError(t, result.Tunnel.Close())

	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	websocket := requireNestedMap(t, diagnostic, "websocket")
	require.Equal(t, true, websocket["open"])
	require.Equal(t, "websocket_event", websocket["usage_source"])
	events := requireAnySlice(t, websocket, "events")
	require.Len(t, events, 1)
	firstEvent, ok := events[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "response.completed", firstEvent["event_type"])
	require.Equal(t, true, firstEvent["usage_detected"])
	requireContainsPath(t, firstEvent, "json_key_paths", "response.usage.input_tokens")
	requireContainsPath(t, firstEvent, "json_key_paths", "response.output[0].content[0].text")
	raw := mustJSON(t, diagnostic)
	require.NotContains(t, raw, "ws-secret")
	require.NotContains(t, raw, "secret ws answer")
}

func TestAdapterEnforcementBillsSuccessfulAdapterUsageOnce(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: 200,
		Usage: adapterclient.Usage{
			InputUnits:   1200,
			OutputUnits:  2,
			BillableUnit: 2,
			CostUSD:      0.42,
		},
		Body: []byte(`{"id":"adapter-ok"}`),
	})
	apiKey := &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10}
	usageRepo := newMemoryAdapterUsageRepo()
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-bill",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "midjourney",
		Method:             "POST",
		Path:               "/v1/images/generations",
		Capability:         "image_generation",
		Model:              "mj-v6",
		APIKey:             apiKey,
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"mj-v6"}`)),
		Payload:            []byte(`{"model":"mj-v6"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd)
	require.Equal(t, "req-bill", billingRepo.lastCmd.RequestID)
	require.Equal(t, int64(20), billingRepo.lastCmd.APIKeyID)
	require.Equal(t, int64(10), billingRepo.lastCmd.UserID)
	require.Equal(t, "mj-v6", billingRepo.lastCmd.Model)
	require.Equal(t, 1200, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 2, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.42, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.InDelta(t, 0.42, billingRepo.lastCmd.APIKeyQuotaCost, 1e-12)
	require.InDelta(t, 0.42, billingRepo.lastCmd.APIKeyRateLimitCost, 1e-12)
	require.Equal(t, HashUsageRequestPayload([]byte(`{"model":"mj-v6"}`)), billingRepo.lastCmd.RequestPayloadHash)
	require.Len(t, auditRepo.records, 1)
	require.Equal(t, true, auditRepo.records[0].Metadata["billing_applied"])
	require.InDelta(t, 0.42, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, "req-bill", usageRepo.records[0].RequestID)
	require.Equal(t, "midjourney", usageRepo.records[0].Provider)
	require.Equal(t, "succeeded", usageRepo.records[0].Status)
	require.Equal(t, 1200, usageRepo.records[0].InputUnits)
	require.Equal(t, 2, usageRepo.records[0].OutputUnits)
	require.InDelta(t, 0.42, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
}

func TestAdapterEnforcementFinalizesStreamingUsageFromTrailer(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-chat")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail chat",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader("data: hello\n\n")),
		Trailers: http.Header{
			"X-Sub2API-Adapter-Usage": []string{`{"input_units":12,"output_units":4,"billable_unit":16,"cost_usd":0.008}`},
		},
	})
	apiKey := &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-stream-bill",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "longtail-chat",
		Method:             "POST",
		Path:               "/v1/chat/completions",
		Capability:         "chat",
		Model:              "longtail-model",
		APIKey:             apiKey,
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"longtail-model"}`)),
		Payload:            []byte(`{"model":"longtail-model"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	require.True(t, result.Response.Stream)
	require.Zero(t, billingRepo.calls)
	require.Len(t, auditRepo.records, 1)
	require.Len(t, usageRepo.records, 1)
	require.Zero(t, usageRepo.records[0].CostUSD)

	body, err := io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.Equal(t, "data: hello\n\n", string(body))
	require.NoError(t, result.Response.BodyStream.Close())

	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd)
	require.Equal(t, "req-stream-bill", billingRepo.lastCmd.RequestID)
	require.Equal(t, 12, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 4, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.008, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.Len(t, auditRepo.records, 1)
	require.InDelta(t, 0.008, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Equal(t, true, auditRepo.records[0].Metadata["billing_applied"])
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, 12, usageRepo.records[0].InputUnits)
	require.Equal(t, 4, usageRepo.records[0].OutputUnits)
	require.Equal(t, 16, usageRepo.records[0].BillableUnits)
	require.InDelta(t, 0.008, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, "trailer", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementFinalizesStreamingUsageFromOpenAISSEFinalChunk(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-chat")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail chat",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	streamBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hi"}}],"usage":null}`,
		"",
		`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16,"cost_usd":0.008}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	apiKey := &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-stream-openai-usage",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "longtail-chat",
		Method:             "POST",
		Path:               "/v1/chat/completions",
		Capability:         "chat",
		Model:              "longtail-model",
		APIKey:             apiKey,
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"longtail-model"}`)),
		Payload:            []byte(`{"model":"longtail-model"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	require.True(t, result.Response.Stream)
	require.Zero(t, billingRepo.calls)
	require.Len(t, auditRepo.records, 1)
	require.Len(t, usageRepo.records, 1)
	require.Zero(t, usageRepo.records[0].CostUSD)

	body, err := io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.Equal(t, streamBody, string(body))
	require.NoError(t, result.Response.BodyStream.Close())

	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd)
	require.Equal(t, "req-stream-openai-usage", billingRepo.lastCmd.RequestID)
	require.Equal(t, 12, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 4, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.008, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.Len(t, auditRepo.records, 1)
	require.InDelta(t, 0.008, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Equal(t, true, auditRepo.records[0].Metadata["billing_applied"])
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, 12, usageRepo.records[0].InputUnits)
	require.Equal(t, 4, usageRepo.records[0].OutputUnits)
	require.Equal(t, 16, usageRepo.records[0].BillableUnits)
	require.InDelta(t, 0.008, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, "sse_final_chunk", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementFinalizesStreamingUsageFromGeminiUsageMetadata(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-gemini")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail Gemini",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	streamBody := strings.Join([]string{
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":30,"thoughtsTokenCount":20,"cachedContentTokenCount":10}}}`,
		"",
		`data: {"response":{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":40,"thoughtsTokenCount":25,"cachedContentTokenCount":10}}}`,
		"",
	}, "\n")
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-stream-gemini-usage",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "longtail-gemini",
		Method:             "POST",
		Path:               "/v1beta/models/gemini:streamGenerateContent",
		Capability:         "chat",
		Model:              "gemini-longtail",
		APIKey:             &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10},
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"gemini-longtail"}`)),
		Payload:            []byte(`{"model":"gemini-longtail"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	_, err = io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.NoError(t, result.Response.BodyStream.Close())

	require.Zero(t, billingRepo.calls)
	require.Equal(t, 90, usageRepo.records[0].InputUnits)
	require.Equal(t, 65, usageRepo.records[0].OutputUnits)
	require.Equal(t, 155, usageRepo.records[0].BillableUnits)
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Equal(t, "not_billable", auditRepo.records[0].Metadata["billing_skipped_reason"])
	require.Equal(t, "sse_final_chunk", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementFinalizesStreamingUsageFromClaudeMessageEvents(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-claude")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail Claude",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	streamBody := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":35576,"cache_read_input_tokens":12000,"output_tokens":1}}}`,
		"",
		`event: message_delta`,
		`data: {"type":"message_delta","usage":{"output_tokens":816,"cost_usd":0.123}}`,
		"",
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-stream-claude-usage",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "longtail-claude",
		Method:             "POST",
		Path:               "/v1/messages",
		Capability:         "chat",
		Model:              "claude-longtail",
		APIKey:             &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10},
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"claude-longtail"}`)),
		Payload:            []byte(`{"model":"claude-longtail"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	_, err = io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.NoError(t, result.Response.BodyStream.Close())

	require.Equal(t, 1, billingRepo.calls)
	require.Equal(t, 35576, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 816, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.123, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Equal(t, "sse_final_chunk", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementDiagnosticSamplingCapturesSSEUsageEventShape(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-gemini")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail Gemini",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	streamBody := strings.Join([]string{
		`event: candidate`,
		`data: {"response":{"candidates":[{"content":{"parts":[{"text":"secret streamed text"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3}}}`,
		"",
		`event: done`,
		`data: {"response":{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}}`,
		"",
	}, "\n")
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{
			Enabled: true,
			DiagnosticSampling: AdapterDiagnosticSamplingConfig{
				Enabled:         true,
				Providers:       []string{"longtail-gemini"},
				MaxPayloadBytes: 512,
				MaxStringBytes:  64,
				MaxEvents:       2,
			},
		},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-sse-diag",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "longtail-gemini",
		Method:        "POST",
		Path:          "/v1beta/models/gemini:streamGenerateContent",
		Capability:    "chat",
		Model:         "gemini-longtail",
		APIKey:        &APIKey{ID: 20, UserID: 10, Quota: 100},
		User:          &User{ID: 10},
		Payload:       []byte(`{"model":"gemini-longtail","contents":[{"parts":[{"text":"private request"}]}]}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	_, err = io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.NoError(t, result.Response.BodyStream.Close())

	diagnostic := requireDiagnosticMap(t, auditRepo.records[0].Metadata)
	stream := requireNestedMap(t, diagnostic, "stream")
	require.Equal(t, "sse", stream["transport"])
	require.Equal(t, "sse_final_chunk", stream["usage_source"])
	events := requireAnySlice(t, stream, "events")
	require.Len(t, events, 2)
	firstEvent, ok := events[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "candidate", firstEvent["event_type"])
	require.Equal(t, true, firstEvent["usage_detected"])
	requireContainsPath(t, firstEvent, "json_key_paths", "response.usageMetadata.promptTokenCount")
	requireContainsPath(t, firstEvent, "json_key_paths", "response.candidates[0].content.parts[0].text")
	raw := mustJSON(t, diagnostic)
	require.NotContains(t, raw, "secret streamed text")
	require.NotContains(t, raw, "private request")
}

func TestAdapterEnforcementRecordsFreeOpenAISSEFinalChunkUsageWithoutBilling(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "longtail-chat")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Longtail chat",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	streamBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"hi"}}],"usage":null}`,
		"",
		`data: {"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	auditRepo := newMemoryAdapterRequestRepo()
	usageRepo := newMemoryAdapterUsageRepo()
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	svc.SetAdapterUsageService(NewAdapterUsageService(usageRepo))

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:          "req-stream-openai-free-usage",
		UserID:             10,
		APIKeyID:           20,
		GroupID:            30,
		GroupPlatform:      "longtail-chat",
		Method:             "POST",
		Path:               "/v1/chat/completions",
		Capability:         "chat",
		Model:              "longtail-model",
		APIKey:             &APIKey{ID: 20, UserID: 10, Quota: 100, RateLimit1d: 10},
		User:               &User{ID: 10},
		RequestPayloadHash: HashUsageRequestPayload([]byte(`{"model":"longtail-model"}`)),
		Payload:            []byte(`{"model":"longtail-model"}`),
	})

	require.NoError(t, err)
	require.True(t, result.Handled)
	_, err = io.ReadAll(result.Response.BodyStream)
	require.NoError(t, err)
	require.NoError(t, result.Response.BodyStream.Close())

	require.Zero(t, billingRepo.calls)
	require.Len(t, auditRepo.records, 1)
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Equal(t, "not_billable", auditRepo.records[0].Metadata["billing_skipped_reason"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, 12, usageRepo.records[0].InputUnits)
	require.Equal(t, 4, usageRepo.records[0].OutputUnits)
	require.Equal(t, 16, usageRepo.records[0].BillableUnits)
	require.Zero(t, usageRepo.records[0].CostUSD)
	require.False(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, "sse_final_chunk", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementSkipsBillingForFailedOrFreeAdapterUsage(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		response adapterclient.Response
	}{
		{name: "failed", response: adapterclient.Response{Status: adapterclient.StatusFailed, AdapterStatus: 502, Usage: adapterclient.Usage{CostUSD: 0.42}}},
		{name: "free", response: adapterclient.Response{Status: adapterclient.StatusSucceeded, AdapterStatus: 200}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
			svc := NewAdapterEnforcementService(
				AdapterEnforcementConfig{Enabled: true},
				NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
				NewAdapterProviderService(providerRepo),
				NewAdapterRequestService(newMemoryAdapterRequestRepo()),
				adapterclient.NewFakeClient(tc.response),
				billingRepo,
			)

			_, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
				RequestID:     "req-" + tc.name,
				UserID:        10,
				APIKeyID:      20,
				GroupID:       30,
				GroupPlatform: "midjourney",
				Method:        "POST",
				Path:          "/v1/images/generations",
				Capability:    "image_generation",
				APIKey:        &APIKey{ID: 20, UserID: 10, Quota: 100},
				User:          &User{ID: 10},
			})

			require.NoError(t, err)
			require.Zero(t, billingRepo.calls)
		})
	}
}

func TestAdapterEnforcementBillsSubscriptionUsageAsSubscriptionCost(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = activeTestAdapterProvider(1, "midjourney")
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	billingRepo := &adapterUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	groupID := int64(30)
	subscriptionID := int64(70)
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(newMemoryAdapterRequestRepo()),
		adapterclient.NewFakeClient(adapterclient.Response{
			Status:        adapterclient.StatusSucceeded,
			AdapterStatus: 200,
			Usage:         adapterclient.Usage{CostUSD: 0.25},
		}),
		billingRepo,
	)

	_, err = svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-sub",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       groupID,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
		APIKey: &APIKey{
			ID:      20,
			UserID:  10,
			GroupID: &groupID,
			Group:   &Group{ID: groupID, Platform: "midjourney", SubscriptionType: SubscriptionTypeSubscription},
		},
		User:         &User{ID: 10},
		Subscription: &UserSubscription{ID: subscriptionID},
	})

	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd.SubscriptionID)
	require.Equal(t, subscriptionID, *billingRepo.lastCmd.SubscriptionID)
	require.InDelta(t, 0.25, billingRepo.lastCmd.SubscriptionCost, 1e-12)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
}

func TestAdapterEnforcementNeverRoutesCoreGroupPlatformToAdapter(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Wildcard adapter policy",
		Status:            RoutePolicyStatusActive,
		MatchMethod:       "POST",
		MatchPath:         "/v1/images/generations",
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
		Priority:          10,
	})
	require.NoError(t, err)
	client := adapterclient.NewFakeClient(adapterclient.Response{Status: adapterclient.StatusSucceeded})
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(newMemoryAdapterRequestRepo()),
		client,
	)

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-core",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "openai",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
	})

	require.NoError(t, err)
	require.False(t, result.Handled)
	require.Empty(t, client.Requests())
}

func activeTestAdapterProvider(id int64, slug string) *AdapterProvider {
	return &AdapterProvider{
		ID:           id,
		Name:         slug,
		Slug:         slug,
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
}

func TestAdapterEnforcementAuditsAdapterClientFailure(t *testing.T) {
	providerRepo := newMemoryAdapterProviderRepo()
	providerRepo.providers[1] = &AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       AdapterProviderStatusActive,
		AdapterType:  AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	policyRepo := newMemoryRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &RoutePolicy{
		Name:              "Midjourney images",
		Status:            RoutePolicyStatusActive,
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64(1),
	})
	require.NoError(t, err)
	auditRepo := newMemoryAdapterRequestRepo()
	client := &errorAdapterClient{err: errors.New("adapter unavailable")}
	svc := NewAdapterEnforcementService(
		AdapterEnforcementConfig{Enabled: true},
		NewRoutePolicyService(policyRepo, NewAdapterProviderService(providerRepo)),
		NewAdapterProviderService(providerRepo),
		NewAdapterRequestService(auditRepo),
		client,
	)

	result, err := svc.Enforce(context.Background(), AdapterEnforcementInput{
		RequestID:     "req-fail",
		UserID:        10,
		APIKeyID:      20,
		GroupID:       30,
		GroupPlatform: "midjourney",
		Method:        "POST",
		Path:          "/v1/images/generations",
		Capability:    "image_generation",
	})

	require.Error(t, err)
	require.True(t, result.Handled)
	require.Len(t, auditRepo.records, 1)
	require.Contains(t, auditRepo.records[0].ErrorMessage, "adapter unavailable")
	require.Nil(t, auditRepo.records[0].StatusCode)
}

type errorAdapterClient struct {
	err error
}

func (c *errorAdapterClient) Do(_ context.Context, req adapterclient.Request) (adapterclient.Response, error) {
	if err := req.Validate(); err != nil {
		return adapterclient.Response{}, err
	}
	return adapterclient.Response{}, c.err
}

type fakeWebSocketAdapterClient struct {
	tunnel       adapterclient.WSTunnel
	wsErr        error
	wsRequests   []adapterclient.Request
	httpRequests []adapterclient.Request
}

func (c *fakeWebSocketAdapterClient) Do(_ context.Context, req adapterclient.Request) (adapterclient.Response, error) {
	c.httpRequests = append(c.httpRequests, req)
	return adapterclient.Response{Status: adapterclient.StatusSucceeded}, nil
}

func (c *fakeWebSocketAdapterClient) OpenWebSocket(_ context.Context, req adapterclient.Request) (adapterclient.WSTunnel, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	c.wsRequests = append(c.wsRequests, req)
	return c.tunnel, c.wsErr
}

type fakeAdapterWSTunnel struct {
	readType    coderws.MessageType
	readPayload []byte
	readErr     error
	writes      [][]byte
	closed      bool
}

func (t *fakeAdapterWSTunnel) Read(context.Context) (coderws.MessageType, []byte, error) {
	return t.readType, append([]byte(nil), t.readPayload...), t.readErr
}

func (t *fakeAdapterWSTunnel) Write(_ context.Context, _ coderws.MessageType, payload []byte) error {
	t.writes = append(t.writes, append([]byte(nil), payload...))
	return nil
}

func (t *fakeAdapterWSTunnel) Close() error {
	t.closed = true
	return nil
}

type memoryAdapterRequestRepo struct {
	records []*AdapterRequestRecord
}

func newMemoryAdapterRequestRepo() *memoryAdapterRequestRepo {
	return &memoryAdapterRequestRepo{}
}

func (r *memoryAdapterRequestRepo) Create(_ context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error) {
	created := record.Clone()
	created.ID = int64(len(r.records) + 1)
	r.records = append(r.records, created.Clone())
	return created, nil
}

func (r *memoryAdapterRequestRepo) Update(_ context.Context, record *AdapterRequestRecord) (*AdapterRequestRecord, error) {
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

func (r *memoryAdapterRequestRepo) List(_ context.Context, filters AdapterRequestListFilters) ([]AdapterRequestSafeView, error) {
	limit := filters.Limit
	if limit <= 0 || limit > len(r.records) {
		limit = len(r.records)
	}
	out := make([]AdapterRequestSafeView, 0, limit)
	for i := len(r.records) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.records[i].SafeView())
	}
	return out, nil
}

func (r *memoryAdapterRequestRepo) Count(_ context.Context, _ AdapterRequestListFilters) (int, error) {
	return len(r.records), nil
}

type adapterUsageBillingRepoStub struct {
	result  *UsageBillingApplyResult
	err     error
	calls   int
	lastCmd *UsageBillingCommand
}

func (r *adapterUsageBillingRepoStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.calls++
	r.lastCmd = cmd
	return r.result, r.err
}

func requireDiagnosticMap(t *testing.T, metadata map[string]any) map[string]any {
	t.Helper()
	raw, ok := metadata["diagnostic"]
	require.True(t, ok)
	diagnostic, ok := raw.(map[string]any)
	require.True(t, ok)
	return diagnostic
}

func requireNestedMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := parent[key]
	require.True(t, ok)
	nested, ok := raw.(map[string]any)
	require.True(t, ok)
	return nested
}

func requireAnySlice(t *testing.T, parent map[string]any, key string) []any {
	t.Helper()
	raw, ok := parent[key]
	require.True(t, ok)
	values, ok := raw.([]any)
	require.True(t, ok)
	return values
}

func requireContainsPath(t *testing.T, parent map[string]any, key string, want string) {
	t.Helper()
	raw, ok := parent[key]
	require.True(t, ok)
	values, ok := raw.([]string)
	require.True(t, ok)
	require.Contains(t, values, want)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}
