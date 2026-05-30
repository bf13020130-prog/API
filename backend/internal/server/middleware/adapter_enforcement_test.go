package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdapterEnforcementDisabledContinuesAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := adapterclient.NewFakeClient(adapterclient.Response{Status: adapterclient.StatusSucceeded})
	enforcer := service.NewAdapterEnforcementService(service.AdapterEnforcementConfig{Enabled: false}, nil, nil, nil, client)
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("midjourney"))
	router.Use(AdapterEnforcement(enforcer))
	router.POST("/v1/images/generations", func(c *gin.Context) {
		body := make([]byte, c.Request.ContentLength)
		_, _ = c.Request.Body.Read(body)
		c.String(http.StatusAccepted, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"mj-v6"}`))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, `{"model":"mj-v6"}`, w.Body.String())
	require.Empty(t, client.Requests())
}

func TestAdapterEnforcementMiddlewareShortCircuitsAdapterResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"image_generation"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "Midjourney images",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "POST",
		MatchPath:         "/v1/images/generations",
		MatchCapability:   "image_generation",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
	})
	require.NoError(t, err)
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusCreated,
		Body:          []byte(`{"id":"adapter-ok"}`),
	})
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(nil),
		client,
	)
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("midjourney"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-mw"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.POST("/v1/images/generations", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native handler should not run")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"mj-v6"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.JSONEq(t, `{"id":"adapter-ok"}`, w.Body.String())
	requests := client.Requests()
	require.Len(t, requests, 1)
	require.Equal(t, "req-mw", requests[0].RequestID)
	require.Equal(t, "midjourney", requests[0].Provider)
	require.Equal(t, "image_generation", requests[0].Capability)
	require.Equal(t, []byte(`{"model":"mj-v6"}`), requests[0].Payload)
}

func TestAdapterEnforcementMiddlewareStreamsAdapterResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "SSE Chat Adapter",
		Slug:         "sse-chat",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "SSE chat",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "POST",
		MatchPath:         "/v1/chat/completions",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
	})
	require.NoError(t, err)
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader("data: hello\n\n")),
	})
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(nil),
		client,
	)
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("sse-chat"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-stream"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native handler should not run")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"longtail-chat"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	require.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	require.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
	require.Equal(t, "data: hello\n\n", w.Body.String())
	requests := client.Requests()
	require.Len(t, requests, 1)
	require.Equal(t, "req-stream", requests[0].RequestID)
	require.Equal(t, "sse-chat", requests[0].Provider)
	require.Equal(t, "chat", requests[0].Capability)
}

func TestAdapterEnforcementMiddlewareFinalizesStreamingUsageTrailer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "SSE Chat Adapter",
		Slug:         "sse-chat",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "SSE chat",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "POST",
		MatchPath:         "/v1/chat/completions",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
	})
	require.NoError(t, err)
	auditRepo := newMiddlewareAdapterRequestRepo()
	usageRepo := newMiddlewareAdapterUsageRepo()
	billingRepo := &middlewareUsageBillingRepoStub{result: &service.UsageBillingApplyResult{Applied: true}}
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
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	enforcer.SetAdapterUsageService(service.NewAdapterUsageService(usageRepo))
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("sse-chat"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-stream-finalize"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native handler should not run")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"longtail-chat"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "data: hello\n\n", w.Body.String())
	require.Equal(t, 1, billingRepo.calls)
	require.Len(t, auditRepo.records, 1)
	require.InDelta(t, 0.008, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, 12, usageRepo.records[0].InputUnits)
	require.Equal(t, 4, usageRepo.records[0].OutputUnits)
	require.InDelta(t, 0.008, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
}

func TestAdapterEnforcementMiddlewareFinalizesStreamingUsageFromOpenAISSEFinalChunk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "SSE Chat Adapter",
		Slug:         "sse-chat",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      "https://adapter.example.com",
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "SSE chat",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "POST",
		MatchPath:         "/v1/chat/completions",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
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
	auditRepo := newMiddlewareAdapterRequestRepo()
	usageRepo := newMiddlewareAdapterUsageRepo()
	billingRepo := &middlewareUsageBillingRepoStub{result: &service.UsageBillingApplyResult{Applied: true}}
	client := adapterclient.NewFakeClient(adapterclient.Response{
		Status:        adapterclient.StatusSucceeded,
		AdapterStatus: http.StatusOK,
		ContentType:   "text/event-stream",
		Stream:        true,
		BodyStream:    io.NopCloser(strings.NewReader(streamBody)),
	})
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(auditRepo),
		client,
		billingRepo,
	)
	enforcer.SetAdapterUsageService(service.NewAdapterUsageService(usageRepo))
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("sse-chat"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-stream-openai-finalize"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native handler should not run")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"longtail-chat"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, streamBody, w.Body.String())
	require.Equal(t, 1, billingRepo.calls)
	require.Len(t, auditRepo.records, 1)
	require.InDelta(t, 0.008, auditRepo.records[0].Metadata["cost_usd"], 1e-12)
	require.Equal(t, true, auditRepo.records[0].Metadata["stream_usage_finalized"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, 12, usageRepo.records[0].InputUnits)
	require.Equal(t, 4, usageRepo.records[0].OutputUnits)
	require.InDelta(t, 0.008, usageRepo.records[0].CostUSD, 1e-12)
	require.True(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, "sse_final_chunk", usageRepo.records[0].Metadata["usage_source"])
}

func TestAdapterEnforcementMiddlewareProxiesAdapterWebSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adapterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/adapter/ws", r.URL.Path)
		require.Equal(t, "req-ws-mw", r.Header.Get("X-Sub2API-Request-ID"))
		require.Equal(t, "ws-chat", r.Header.Get("X-Sub2API-Provider"))
		conn, err := coderws.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		msgType, payload, err := conn.Read(ctx)
		require.NoError(t, err)
		require.NoError(t, conn.Write(ctx, msgType, append([]byte("adapter:"), payload...)))
	}))
	defer adapterServer.Close()

	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "WS Chat Adapter",
		Slug:         "ws-chat",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      adapterServer.URL,
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "WS chat",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
	})
	require.NoError(t, err)
	auditRepo := newMiddlewareAdapterRequestRepo()
	usageRepo := newMiddlewareAdapterUsageRepo()
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(auditRepo),
		adapterclient.NewHTTPClient(adapterclient.NewStaticProviderRegistry([]adapterclient.ProviderConfig{
			{
				Name:         "WS Chat Adapter",
				Slug:         "ws-chat",
				Status:       adapterclient.ProviderStatusActive,
				AdapterType:  "new-api",
				BaseURL:      adapterServer.URL,
				Capabilities: []string{"chat"},
			},
		}), adapterServer.Client()),
	)
	enforcer.SetAdapterUsageService(service.NewAdapterUsageService(usageRepo))
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("ws-chat"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-ws-mw"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.GET("/v1/responses", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native websocket handler should not run")
	})
	gatewayServer := httptest.NewServer(router)
	defer gatewayServer.Close()

	wsURL := "ws" + strings.TrimPrefix(gatewayServer.URL, "http") + "/v1/responses"
	clientConn, _, err := coderws.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer clientConn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, clientConn.Write(ctx, coderws.MessageText, []byte(`{"hello":true}`)))
	msgType, payload, err := clientConn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, coderws.MessageText, msgType)
	require.Equal(t, `adapter:{"hello":true}`, string(payload))

	require.Len(t, auditRepo.records, 1)
	require.Equal(t, true, auditRepo.records[0].Metadata["websocket"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, true, usageRepo.records[0].Metadata["websocket"])
}

func TestAdapterEnforcementMiddlewareFinalizesWebSocketUsageEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adapterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/internal/adapter/ws", r.URL.Path)
		conn, err := coderws.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_ws","usage":{"input_tokens":7,"output_tokens":3,"cost_usd":0.02}}}`)))
	}))
	defer adapterServer.Close()

	providerRepo := newMiddlewareAdapterProviderRepo()
	providerRepo.providers[1] = &service.AdapterProvider{
		ID:           1,
		Name:         "WS Chat Adapter",
		Slug:         "ws-chat",
		Status:       service.AdapterProviderStatusActive,
		AdapterType:  service.AdapterProviderTypeNewAPI,
		BaseURL:      adapterServer.URL,
		Capabilities: []string{"chat"},
		TimeoutMS:    30000,
	}
	policyRepo := newMiddlewareRoutePolicyRepo()
	_, err := policyRepo.Create(context.Background(), &service.RoutePolicy{
		Name:              "WS chat",
		Status:            service.RoutePolicyStatusActive,
		MatchMethod:       "GET",
		MatchPath:         "/v1/responses",
		MatchCapability:   "chat",
		Target:            string(capabilityrouter.TargetNewAPIAdapter),
		AdapterProviderID: ptrInt64Middleware(1),
	})
	require.NoError(t, err)
	auditRepo := newMiddlewareAdapterRequestRepo()
	usageRepo := newMiddlewareAdapterUsageRepo()
	billingRepo := &middlewareUsageBillingRepoStub{result: &service.UsageBillingApplyResult{Applied: true}}
	enforcer := service.NewAdapterEnforcementService(
		service.AdapterEnforcementConfig{Enabled: true},
		service.NewRoutePolicyService(policyRepo, service.NewAdapterProviderService(providerRepo)),
		service.NewAdapterProviderService(providerRepo),
		service.NewAdapterRequestService(auditRepo),
		adapterclient.NewHTTPClient(adapterclient.NewStaticProviderRegistry([]adapterclient.ProviderConfig{
			{
				Name:         "WS Chat Adapter",
				Slug:         "ws-chat",
				Status:       adapterclient.ProviderStatusActive,
				AdapterType:  "new-api",
				BaseURL:      adapterServer.URL,
				Capabilities: []string{"chat"},
			},
		}), adapterServer.Client()),
		billingRepo,
	)
	enforcer.SetAdapterUsageService(service.NewAdapterUsageService(usageRepo))
	router := gin.New()
	router.Use(seedAdapterEnforcementAPIKey("ws-chat"))
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-ws-finalize"))
		c.Next()
	})
	router.Use(AdapterEnforcement(enforcer))
	router.GET("/v1/responses", func(c *gin.Context) {
		c.String(http.StatusTeapot, "native websocket handler should not run")
	})
	gatewayServer := httptest.NewServer(router)
	defer gatewayServer.Close()

	wsURL := "ws" + strings.TrimPrefix(gatewayServer.URL, "http") + "/v1/responses"
	clientConn, _, err := coderws.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msgType, payload, err := clientConn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, coderws.MessageText, msgType)
	require.Contains(t, string(payload), "response.completed")
	_ = clientConn.Close(coderws.StatusNormalClosure, "done")

	require.Eventually(t, func() bool {
		return billingRepo.calls == 1 && len(auditRepo.records) == 1 && auditRepo.records[0].Metadata["websocket_usage_finalized"] == true
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, 7, billingRepo.lastCmd.InputTokens)
	require.Equal(t, 3, billingRepo.lastCmd.OutputTokens)
	require.InDelta(t, 0.02, billingRepo.lastCmd.BalanceCost, 1e-12)
	require.Equal(t, "websocket_event", auditRepo.records[0].Metadata["usage_source"])
	require.Len(t, usageRepo.records, 1)
	require.Equal(t, "succeeded", usageRepo.records[0].Status)
	require.Equal(t, 7, usageRepo.records[0].InputUnits)
	require.Equal(t, 3, usageRepo.records[0].OutputUnits)
	require.True(t, usageRepo.records[0].BillingApplied)
	require.Equal(t, true, usageRepo.records[0].Metadata["websocket_usage_finalized"])
}

func seedAdapterEnforcementAPIKey(platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := int64(30)
		group := &service.Group{ID: groupID, Platform: platform, Status: service.StatusActive, Hydrated: true}
		c.Set(string(ContextKeyAPIKey), &service.APIKey{
			ID:      20,
			UserID:  10,
			GroupID: &groupID,
			Group:   group,
			User:    &service.User{ID: 10},
		})
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Next()
	}
}

type middlewareRoutePolicyRepo struct {
	policies map[int64]*service.RoutePolicy
	nextID   int64
}

func newMiddlewareRoutePolicyRepo() *middlewareRoutePolicyRepo {
	return &middlewareRoutePolicyRepo{policies: map[int64]*service.RoutePolicy{}, nextID: 1}
}

func (r *middlewareRoutePolicyRepo) List(context.Context) ([]*service.RoutePolicy, error) {
	out := make([]*service.RoutePolicy, 0, len(r.policies))
	for id := int64(1); id <= int64(len(r.policies)); id++ {
		if policy := r.policies[id]; policy != nil {
			out = append(out, policy.Clone())
		}
	}
	return out, nil
}

func (r *middlewareRoutePolicyRepo) GetByID(_ context.Context, id int64) (*service.RoutePolicy, error) {
	if policy := r.policies[id]; policy != nil {
		return policy.Clone(), nil
	}
	return nil, service.ErrRoutePolicyNotFound
}

func (r *middlewareRoutePolicyRepo) Create(_ context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	created := policy.Clone()
	created.ID = r.nextID
	r.nextID++
	r.policies[created.ID] = created.Clone()
	return created, nil
}

func (r *middlewareRoutePolicyRepo) Update(_ context.Context, policy *service.RoutePolicy) (*service.RoutePolicy, error) {
	r.policies[policy.ID] = policy.Clone()
	return policy.Clone(), nil
}

func (r *middlewareRoutePolicyRepo) Delete(_ context.Context, id int64) error {
	delete(r.policies, id)
	return nil
}

type middlewareAdapterProviderRepo struct {
	providers map[int64]*service.AdapterProvider
}

func newMiddlewareAdapterProviderRepo() *middlewareAdapterProviderRepo {
	return &middlewareAdapterProviderRepo{providers: map[int64]*service.AdapterProvider{}}
}

func (r *middlewareAdapterProviderRepo) List(context.Context) ([]*service.AdapterProvider, error) {
	out := make([]*service.AdapterProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		out = append(out, provider.Clone())
	}
	return out, nil
}

func (r *middlewareAdapterProviderRepo) GetByID(_ context.Context, id int64) (*service.AdapterProvider, error) {
	if provider := r.providers[id]; provider != nil {
		return provider.Clone(), nil
	}
	return nil, service.ErrAdapterProviderNotFound
}

func (r *middlewareAdapterProviderRepo) Create(_ context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	r.providers[provider.ID] = provider.Clone()
	return provider.Clone(), nil
}

func (r *middlewareAdapterProviderRepo) Update(_ context.Context, provider *service.AdapterProvider) (*service.AdapterProvider, error) {
	r.providers[provider.ID] = provider.Clone()
	return provider.Clone(), nil
}

func (r *middlewareAdapterProviderRepo) Delete(_ context.Context, id int64) error {
	delete(r.providers, id)
	return nil
}

func ptrInt64Middleware(v int64) *int64 {
	return &v
}

type middlewareAdapterRequestRepo struct {
	records []*service.AdapterRequestRecord
}

func newMiddlewareAdapterRequestRepo() *middlewareAdapterRequestRepo {
	return &middlewareAdapterRequestRepo{}
}

func (r *middlewareAdapterRequestRepo) Create(_ context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
	created := record.Clone()
	created.ID = int64(len(r.records) + 1)
	r.records = append(r.records, created.Clone())
	return created, nil
}

func (r *middlewareAdapterRequestRepo) Update(_ context.Context, record *service.AdapterRequestRecord) (*service.AdapterRequestRecord, error) {
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

func (r *middlewareAdapterRequestRepo) List(_ context.Context, filters service.AdapterRequestListFilters) ([]service.AdapterRequestSafeView, error) {
	limit := filters.Limit
	if limit <= 0 || limit > len(r.records) {
		limit = len(r.records)
	}
	out := make([]service.AdapterRequestSafeView, 0, limit)
	for i := len(r.records) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.records[i].SafeView())
	}
	return out, nil
}

func (r *middlewareAdapterRequestRepo) Count(_ context.Context, _ service.AdapterRequestListFilters) (int, error) {
	return len(r.records), nil
}

type middlewareAdapterUsageRepo struct {
	nextID  int64
	records []*service.AdapterUsageRecord
}

func newMiddlewareAdapterUsageRepo() *middlewareAdapterUsageRepo {
	return &middlewareAdapterUsageRepo{nextID: 1}
}

func (r *middlewareAdapterUsageRepo) Create(_ context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
	created := record.Clone()
	created.ID = r.nextID
	r.nextID++
	r.records = append(r.records, created.Clone())
	return created, nil
}

func (r *middlewareAdapterUsageRepo) Update(_ context.Context, record *service.AdapterUsageRecord) (*service.AdapterUsageRecord, error) {
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

func (r *middlewareAdapterUsageRepo) List(_ context.Context, filters service.AdapterUsageFilters) ([]service.AdapterUsageSafeView, error) {
	limit := filters.Limit
	if limit <= 0 || limit > len(r.records) {
		limit = len(r.records)
	}
	out := make([]service.AdapterUsageSafeView, 0, limit)
	for i := len(r.records) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.records[i].SafeView())
	}
	return out, nil
}

func (r *middlewareAdapterUsageRepo) Summary(_ context.Context, filters service.AdapterUsageFilters) (service.AdapterUsageSummary, error) {
	summary := service.AdapterUsageSummary{}
	for _, record := range r.records {
		summary.TotalRequests++
		if record.Status == string(adapterclient.StatusSucceeded) {
			summary.SuccessRequests++
		} else {
			summary.FailedRequests++
		}
		summary.CostUSD += record.CostUSD
	}
	return summary, nil
}

type middlewareUsageBillingRepoStub struct {
	result  *service.UsageBillingApplyResult
	err     error
	calls   int
	lastCmd *service.UsageBillingCommand
}

func (r *middlewareUsageBillingRepoStub) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.calls++
	r.lastCmd = cmd
	return r.result, r.err
}
