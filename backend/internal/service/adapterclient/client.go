package adapterclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	coderws "github.com/coder/websocket"
)

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusPending   Status = "pending"
)

var ErrInvalidRequest = errors.New("invalid adapter request")

type Client interface {
	Do(ctx context.Context, req Request) (Response, error)
}

type WebSocketClient interface {
	OpenWebSocket(ctx context.Context, req Request) (WSTunnel, error)
}

type WSTunnel interface {
	Read(ctx context.Context) (coderws.MessageType, []byte, error)
	Write(ctx context.Context, msgType coderws.MessageType, payload []byte) error
	Close() error
}

type Request struct {
	RequestID       string
	UserID          int64
	APIKeyID        int64
	GroupID         int64
	Provider        string
	Capability      string
	Model           string
	RouteTarget     capabilityrouter.Target
	BillingCategory string
	Method          string
	Path            string
	Headers         map[string]string
	Payload         []byte
}

type Usage struct {
	InputUnits   int64   `json:"input_units"`
	OutputUnits  int64   `json:"output_units"`
	BillableUnit int64   `json:"billable_unit"`
	CostUSD      float64 `json:"cost_usd"`
}

type Response struct {
	Status        Status
	AdapterStatus int
	UpstreamID    string
	Usage         Usage
	Body          []byte
	ContentType   string
	Stream        bool
	BodyStream    io.ReadCloser
	Trailers      http.Header
	ErrorCode     string
	ErrorMessage  string
}

func (r Request) Validate() error {
	switch {
	case strings.TrimSpace(r.RequestID) == "":
		return errors.Join(ErrInvalidRequest, errors.New("request_id is required"))
	case r.UserID <= 0:
		return errors.Join(ErrInvalidRequest, errors.New("user_id is required"))
	case r.APIKeyID <= 0:
		return errors.Join(ErrInvalidRequest, errors.New("api_key_id is required"))
	case r.GroupID <= 0:
		return errors.Join(ErrInvalidRequest, errors.New("group_id is required"))
	case strings.TrimSpace(r.Provider) == "":
		return errors.Join(ErrInvalidRequest, errors.New("provider is required"))
	case strings.TrimSpace(r.Capability) == "":
		return errors.Join(ErrInvalidRequest, errors.New("capability is required"))
	case r.RouteTarget != capabilityrouter.TargetNewAPIAdapter:
		return errors.Join(ErrInvalidRequest, errors.New("route_target must be new_api_adapter"))
	case strings.TrimSpace(r.Method) == "":
		return errors.Join(ErrInvalidRequest, errors.New("method is required"))
	case strings.TrimSpace(r.Path) == "":
		return errors.Join(ErrInvalidRequest, errors.New("path is required"))
	default:
		return nil
	}
}

type FakeClient struct {
	mu       sync.Mutex
	response Response
	requests []Request
}

func NewFakeClient(response Response) *FakeClient {
	return &FakeClient{response: response}
}

func (c *FakeClient) Do(_ context.Context, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, cloneRequest(req))
	return c.response, nil
}

func (c *FakeClient) Requests() []Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Request, len(c.requests))
	for i := range c.requests {
		out[i] = cloneRequest(c.requests[i])
	}
	return out
}

func cloneRequest(req Request) Request {
	if req.Headers != nil {
		headers := make(map[string]string, len(req.Headers))
		for key, value := range req.Headers {
			headers[key] = value
		}
		req.Headers = headers
	}
	if req.Payload != nil {
		req.Payload = append([]byte(nil), req.Payload...)
	}
	return req
}
