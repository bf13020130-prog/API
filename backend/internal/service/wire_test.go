package service

import (
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/stretchr/testify/require"
	"github.com/zeromicro/go-zero/core/collection"
)

func TestProvideTimingWheelService_ReturnsError(t *testing.T) {
	original := newTimingWheel
	t.Cleanup(func() { newTimingWheel = original })

	newTimingWheel = func(_ time.Duration, _ int, _ collection.Execute) (*collection.TimingWheel, error) {
		return nil, errors.New("boom")
	}

	svc, err := ProvideTimingWheelService()
	if err == nil {
		t.Fatalf("期望返回 error，但得到 nil")
	}
	if svc != nil {
		t.Fatalf("期望返回 nil svc，但得到非空")
	}
}

func TestProvideTimingWheelService_Success(t *testing.T) {
	svc, err := ProvideTimingWheelService()
	if err != nil {
		t.Fatalf("期望 err 为 nil，但得到: %v", err)
	}
	if svc == nil {
		t.Fatalf("期望 svc 非空，但得到 nil")
	}
	svc.Stop()
}

func TestProvideAdapterEnforcementServiceWiresDiagnosticSamplingConfig(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			AdapterEnforcement: config.GatewayAdapterEnforcementConfig{
				Enabled: true,
				DiagnosticSampling: config.GatewayAdapterDiagnosticSamplingConfig{
					Enabled:         true,
					Providers:       []string{"midjourney"},
					RequestIDs:      []string{"req-debug"},
					MaxPayloadBytes: 2048,
					MaxStringBytes:  128,
					MaxEvents:       4,
				},
			},
		},
	}

	svc := ProvideAdapterEnforcementService(
		cfg,
		nil,
		nil,
		nil,
		nil,
		adapterclient.NewFakeClient(adapterclient.Response{Status: adapterclient.StatusSucceeded}),
		nil,
	)

	require.NotNil(t, svc)
	require.True(t, svc.cfg.Enabled)
	require.True(t, svc.cfg.DiagnosticSampling.Enabled)
	require.Equal(t, []string{"midjourney"}, svc.cfg.DiagnosticSampling.Providers)
	require.Equal(t, []string{"req-debug"}, svc.cfg.DiagnosticSampling.RequestIDs)
	require.Equal(t, 2048, svc.cfg.DiagnosticSampling.MaxPayloadBytes)
	require.Equal(t, 128, svc.cfg.DiagnosticSampling.MaxStringBytes)
	require.Equal(t, 4, svc.cfg.DiagnosticSampling.MaxEvents)
}
