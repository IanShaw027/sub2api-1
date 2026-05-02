package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestProvideAISkillRuntimeGateway_UsesRealGateway(t *testing.T) {
	gateway := ProvideAISkillRuntimeGateway(&OpenAIGatewayService{})
	if gateway == nil {
		t.Fatalf("期望 gateway 非空，但得到 nil")
	}

	_, err := gateway.Execute(context.Background(), AISkillExecutionRequest{Type: "unknown"})
	if !errors.Is(err, ErrAISkillExecutionSpecInvalid) {
		t.Fatalf("期望返回 ErrAISkillExecutionSpecInvalid，但得到: %v", err)
	}
}
