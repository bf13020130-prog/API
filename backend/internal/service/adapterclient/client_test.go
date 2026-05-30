package adapterclient

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
)

func TestRequestValidateRequiresSub2APIOwnershipContext(t *testing.T) {
	valid := Request{
		RequestID:       "req_123",
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
		Payload:         []byte(`{"prompt":"test"}`),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request returned error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "request id", mutate: func(r *Request) { r.RequestID = "" }},
		{name: "user id", mutate: func(r *Request) { r.UserID = 0 }},
		{name: "api key id", mutate: func(r *Request) { r.APIKeyID = 0 }},
		{name: "group id", mutate: func(r *Request) { r.GroupID = 0 }},
		{name: "provider", mutate: func(r *Request) { r.Provider = "" }},
		{name: "capability", mutate: func(r *Request) { r.Capability = "" }},
		{name: "route target", mutate: func(r *Request) { r.RouteTarget = capabilityrouter.TargetSub2APINative }},
		{name: "method", mutate: func(r *Request) { r.Method = "" }},
		{name: "path", mutate: func(r *Request) { r.Path = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.mutate(&req)
			if err := req.Validate(); err == nil {
				t.Fatal("Validate returned nil, want error")
			}
		})
	}
}

func TestFakeClientStoresRequestsAndReturnsResponses(t *testing.T) {
	client := NewFakeClient(Response{
		Status:        StatusSucceeded,
		AdapterStatus: 200,
		Usage: Usage{
			InputUnits:  10,
			OutputUnits: 2,
			CostUSD:     0.03,
		},
		Body: []byte(`{"ok":true}`),
	})

	req := Request{
		RequestID:       "req_456",
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
		Payload:         []byte(`{"prompt":"test"}`),
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.Status != StatusSucceeded {
		t.Fatalf("Status = %q, want %q", resp.Status, StatusSucceeded)
	}
	if len(client.Requests()) != 1 {
		t.Fatalf("Requests length = %d, want 1", len(client.Requests()))
	}
	if string(client.Requests()[0].Payload) != string(req.Payload) {
		t.Fatalf("stored Payload = %s, want %s", client.Requests()[0].Payload, req.Payload)
	}
}

func TestFakeClientRejectsInvalidRequest(t *testing.T) {
	client := NewFakeClient(Response{Status: StatusSucceeded})

	_, err := client.Do(context.Background(), Request{
		RequestID:   "req_invalid",
		Provider:    "midjourney",
		Capability:  "image_task",
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      "POST",
		Path:        "/v1/images/generations",
	})
	if err == nil {
		t.Fatal("Do returned nil error for invalid request")
	}
	if len(client.Requests()) != 0 {
		t.Fatalf("Requests length = %d, want 0", len(client.Requests()))
	}
}
