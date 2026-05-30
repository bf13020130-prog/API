package adapterclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	coderws "github.com/coder/websocket"
)

func TestHTTPClientPostsSub2APIOwnershipEnvelope(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/internal/adapter/execute" {
			t.Fatalf("path = %s, want /internal/adapter/execute", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-secret" {
			t.Fatalf("Authorization = %q, want bearer credential", got)
		}
		if got := r.Header.Get("X-Sub2API-Request-ID"); got != "req_789" {
			t.Fatalf("X-Sub2API-Request-ID = %q, want req_789", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("Decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"succeeded",
			"adapter_status":202,
			"upstream_id":"up_123",
			"usage":{"input_units":12,"output_units":3,"billable_unit":1,"cost_usd":0.42},
			"body":{"job_id":"job_123"}
		}`))
	}))
	defer server.Close()

	client := NewHTTPClient(NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "Midjourney",
			Slug:         "midjourney",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      server.URL,
			AuthMode:     "bearer",
			Credentials:  map[string]string{"token": "provider-secret"},
			Capabilities: []string{"image_task"},
		},
	}), server.Client())

	resp, err := client.Do(context.Background(), Request{
		RequestID:       "req_789",
		UserID:          42,
		APIKeyID:        1001,
		GroupID:         7,
		Provider:        "midjourney",
		Capability:      "image_task",
		Model:           "mj-v6",
		RouteTarget:     capabilityrouter.TargetNewAPIAdapter,
		BillingCategory: "image",
		Method:          "POST",
		Path:            "/v1/images/generations",
		Headers:         map[string]string{"X-Client-Trace": "trace_1"},
		Payload:         []byte(`{"prompt":"test"}`),
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.Status != StatusSucceeded || resp.AdapterStatus != http.StatusAccepted {
		t.Fatalf("response status = %s/%d", resp.Status, resp.AdapterStatus)
	}
	if resp.UpstreamID != "up_123" {
		t.Fatalf("UpstreamID = %q, want up_123", resp.UpstreamID)
	}
	if resp.Usage.InputUnits != 12 || resp.Usage.CostUSD != 0.42 {
		t.Fatalf("Usage = %+v", resp.Usage)
	}
	if string(resp.Body) != `{"job_id":"job_123"}` {
		t.Fatalf("Body = %s", resp.Body)
	}

	if received["request_id"] != "req_789" {
		t.Fatalf("request_id = %#v", received["request_id"])
	}
	if received["user_id"] != float64(42) || received["api_key_id"] != float64(1001) || received["group_id"] != float64(7) {
		t.Fatalf("ownership fields = %#v", received)
	}
	if received["route_target"] != "new_api_adapter" || received["provider"] != "midjourney" || received["capability"] != "image_task" {
		t.Fatalf("route fields = %#v", received)
	}
	if received["method"] != "POST" || received["path"] != "/v1/images/generations" {
		t.Fatalf("request method/path = %#v", received)
	}
	if payload, ok := received["payload"].(map[string]any); !ok || payload["prompt"] != "test" {
		t.Fatalf("payload = %#v", received["payload"])
	}
}

func TestHTTPClientReturnsSSEStreamWithoutBuffering(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
	}))
	defer server.Close()

	client := NewHTTPClient(NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "SSE Adapter",
			Slug:         "sse-adapter",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      server.URL,
			Capabilities: []string{"chat"},
		},
	}), server.Client())

	resp, err := client.Do(context.Background(), Request{
		RequestID:   "req_sse",
		UserID:      42,
		APIKeyID:    1001,
		GroupID:     7,
		Provider:    "sse-adapter",
		Capability:  "chat",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "POST",
		Path:        "/v1/chat/completions",
	})

	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if !resp.Stream || resp.BodyStream == nil {
		t.Fatalf("expected streaming response, got Stream=%v BodyStream=%v", resp.Stream, resp.BodyStream)
	}
	if resp.ContentType != "text/event-stream" {
		t.Fatalf("ContentType = %q, want text/event-stream", resp.ContentType)
	}
	body, err := io.ReadAll(resp.BodyStream)
	if err != nil {
		t.Fatalf("ReadAll stream: %v", err)
	}
	if err := resp.BodyStream.Close(); err != nil {
		t.Fatalf("Close stream: %v", err)
	}
	if string(body) != "data: hello\n\n" {
		t.Fatalf("stream body = %q", body)
	}
}

func TestHTTPClientPreservesSSEUsageTrailer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Trailer", "X-Sub2API-Adapter-Usage")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: hello\n\n"))
		w.Header().Set("X-Sub2API-Adapter-Usage", `{"input_units":12,"output_units":4,"billable_unit":16,"cost_usd":0.008}`)
	}))
	defer server.Close()

	client := NewHTTPClient(NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "SSE Adapter",
			Slug:         "sse-adapter",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      server.URL,
			Capabilities: []string{"chat"},
		},
	}), server.Client())

	resp, err := client.Do(context.Background(), Request{
		RequestID:   "req_sse_trailer",
		UserID:      42,
		APIKeyID:    1001,
		GroupID:     7,
		Provider:    "sse-adapter",
		Capability:  "chat",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "POST",
		Path:        "/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if _, err := io.ReadAll(resp.BodyStream); err != nil {
		t.Fatalf("ReadAll stream: %v", err)
	}
	if err := resp.BodyStream.Close(); err != nil {
		t.Fatalf("Close stream: %v", err)
	}
	if got := resp.Trailers.Get("X-Sub2API-Adapter-Usage"); got != `{"input_units":12,"output_units":4,"billable_unit":16,"cost_usd":0.008}` {
		t.Fatalf("usage trailer = %q", got)
	}
}

func TestHTTPClientOpensAdapterWebSocketTunnel(t *testing.T) {
	var receivedPath string
	var receivedRequestID string
	var receivedRouteTarget string
	var receivedProvider string
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedRequestID = r.Header.Get("X-Sub2API-Request-ID")
		receivedRouteTarget = r.Header.Get("X-Sub2API-Route-Target")
		receivedProvider = r.Header.Get("X-Sub2API-Provider")
		receivedAuth = r.Header.Get("Authorization")
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		msgType, payload, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("read websocket payload: %v", err)
			return
		}
		if err := conn.Write(ctx, msgType, append([]byte("adapter:"), payload...)); err != nil {
			t.Errorf("write websocket payload: %v", err)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "WS Adapter",
			Slug:         "ws-adapter",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      server.URL,
			AuthMode:     "bearer",
			Credentials:  map[string]string{"token": "provider-secret"},
			Capabilities: []string{"chat"},
		},
	}), server.Client())

	tunnel, err := client.OpenWebSocket(context.Background(), Request{
		RequestID:   "req_ws",
		UserID:      42,
		APIKeyID:    1001,
		GroupID:     7,
		Provider:    "ws-adapter",
		Capability:  "chat",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "GET",
		Path:        "/v1/responses",
		Headers:     map[string]string{"Sec-WebSocket-Protocol": "realtime"},
	})
	if err != nil {
		t.Fatalf("OpenWebSocket returned error: %v", err)
	}
	defer tunnel.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tunnel.Write(ctx, coderws.MessageText, []byte(`{"hello":true}`)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	msgType, payload, err := tunnel.Read(ctx)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if msgType != coderws.MessageText || string(payload) != `adapter:{"hello":true}` {
		t.Fatalf("websocket echo = %v %s", msgType, payload)
	}

	if receivedPath != "/internal/adapter/ws" {
		t.Fatalf("path = %s, want /internal/adapter/ws", receivedPath)
	}
	if receivedRequestID != "req_ws" || receivedRouteTarget != "new_api_adapter" || receivedProvider != "ws-adapter" {
		t.Fatalf("ownership headers = request_id:%q route:%q provider:%q", receivedRequestID, receivedRouteTarget, receivedProvider)
	}
	if receivedAuth != "Bearer provider-secret" {
		t.Fatalf("Authorization = %q, want bearer credential", receivedAuth)
	}
}

func TestHTTPClientRejectsUnconfiguredProvider(t *testing.T) {
	client := NewHTTPClient(NewStaticProviderRegistry(nil), http.DefaultClient)

	_, err := client.Do(context.Background(), Request{
		RequestID:   "req_missing",
		UserID:      42,
		APIKeyID:    1001,
		GroupID:     7,
		Provider:    "midjourney",
		Capability:  "image_task",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "POST",
		Path:        "/v1/images/generations",
	})
	if err == nil {
		t.Fatal("Do returned nil error, want error")
	}
}

func TestHTTPClientMapsAdapterErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error_code":"adapter_bad_gateway","error_message":"upstream failed"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "Midjourney",
			Slug:         "midjourney",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      server.URL,
			Capabilities: []string{"image_task"},
		},
	}), server.Client())

	resp, err := client.Do(context.Background(), Request{
		RequestID:   "req_error",
		UserID:      42,
		APIKeyID:    1001,
		GroupID:     7,
		Provider:    "midjourney",
		Capability:  "image_task",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "POST",
		Path:        "/v1/images/generations",
	})
	if err != nil {
		t.Fatalf("Do returned transport error: %v", err)
	}
	if resp.Status != StatusFailed || resp.AdapterStatus != http.StatusBadGateway {
		t.Fatalf("response status = %s/%d", resp.Status, resp.AdapterStatus)
	}
	if resp.ErrorCode != "adapter_bad_gateway" || resp.ErrorMessage != "upstream failed" {
		t.Fatalf("error fields = %q/%q", resp.ErrorCode, resp.ErrorMessage)
	}
}
