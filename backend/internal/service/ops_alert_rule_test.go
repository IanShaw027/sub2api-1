package service

import (
	"testing"
	"time"
)

func TestEvaluateRule_Legacy(t *testing.T) {
	rule := OpsAlertRule{
		MetricType: OpsMetricErrorRate,
		Operator:   ">",
		Threshold:  10.0,
	}

	metrics := []OpsMetrics{
		{ErrorRate: 15.0, RequestCount: 100, UpdatedAt: time.Now()},
		{ErrorRate: 12.0, RequestCount: 100, UpdatedAt: time.Now().Add(-1 * time.Minute)},
	}

	breached, value, ok := evaluateRule(rule, metrics)
	if !ok {
		t.Fatal("expected evaluation to succeed")
	}
	if !breached {
		t.Error("expected rule to be breached")
	}
	if value != 15.0 {
		t.Errorf("expected value 15.0, got %f", value)
	}
}

func TestEvaluateRule_WithAlertCategory(t *testing.T) {
	tests := []struct {
		name          string
		alertCategory string
		metricType    string
		operator      string
		threshold     float64
		metrics       []OpsMetrics
		wantBreached  bool
		wantValue     float64
	}{
		{
			name:          "error_rate category",
			alertCategory: "error_rate",
			metricType:    OpsMetricErrorRate,
			operator:      ">",
			threshold:     10.0,
			metrics: []OpsMetrics{
				{ErrorRate: 15.0, RequestCount: 100, UpdatedAt: time.Now()},
			},
			wantBreached: true,
			wantValue:    15.0,
		},
		{
			name:          "error_count category",
			alertCategory: "error_count",
			metricType:    OpsMetricErrorRate,
			operator:      "<",
			threshold:     5.0,
			metrics: []OpsMetrics{
				{ErrorRate: 3.0, RequestCount: 100, UpdatedAt: time.Now()},
			},
			wantBreached: true,
			wantValue:    3.0,
		},
		{
			name:          "latency category",
			alertCategory: "latency",
			metricType:    OpsMetricP95LatencyMs,
			operator:      ">",
			threshold:     1000.0,
			metrics: []OpsMetrics{
				{LatencyP95: 1500, UpdatedAt: time.Now()},
			},
			wantBreached: true,
			wantValue:    1500.0,
		},
		{
			name:          "account_status category",
			alertCategory: "account_status",
			metricType:    OpsMetricErrorRate,
			operator:      ">",
			threshold:     50.0,
			metrics: []OpsMetrics{
				{ErrorRate: 60.0, RequestCount: 100, UpdatedAt: time.Now()},
			},
			wantBreached: true,
			wantValue:    60.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := OpsAlertRule{
				AlertCategory: tt.alertCategory,
				MetricType:    tt.metricType,
				Operator:      tt.operator,
				Threshold:     tt.threshold,
			}

			breached, value, ok := evaluateRule(rule, tt.metrics)
			if !ok {
				t.Fatal("expected evaluation to succeed")
			}
			if breached != tt.wantBreached {
				t.Errorf("expected breached=%v, got %v", tt.wantBreached, breached)
			}
			if value != tt.wantValue {
				t.Errorf("expected value=%f, got %f", tt.wantValue, value)
			}
		})
	}
}

func TestEvaluateRule_EmptyMetrics(t *testing.T) {
	rule := OpsAlertRule{
		MetricType: OpsMetricErrorRate,
		Operator:   ">",
		Threshold:  10.0,
	}

	breached, value, ok := evaluateRule(rule, []OpsMetrics{})
	if ok {
		t.Error("expected evaluation to fail with empty metrics")
	}
	if breached {
		t.Error("expected not breached with empty metrics")
	}
	if value != 0 {
		t.Errorf("expected value 0, got %f", value)
	}
}

func TestEvaluateRule_BackwardCompatibility(t *testing.T) {
	// 测试没有 AlertCategory 的规则（向后兼容）
	rule := OpsAlertRule{
		MetricType: OpsMetricSuccessRate,
		Operator:   "<",
		Threshold:  95.0,
	}

	metrics := []OpsMetrics{
		{SuccessRate: 90.0, RequestCount: 100, UpdatedAt: time.Now()},
		{SuccessRate: 92.0, RequestCount: 100, UpdatedAt: time.Now().Add(-1 * time.Minute)},
	}

	breached, value, ok := evaluateRule(rule, metrics)
	if !ok {
		t.Fatal("expected evaluation to succeed")
	}
	if !breached {
		t.Error("expected rule to be breached")
	}
	if value != 90.0 {
		t.Errorf("expected value 90.0, got %f", value)
	}
}
