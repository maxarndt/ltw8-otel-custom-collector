package knxreceiver

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConvertToMetrics_Gauge(t *testing.T) {
	cfg := &AddressConfig{
		Name:       "leistung_eg_kueche_w",
		DPT:        "14.056",
		MetricType: MetricTypeGauge,
		Labels:     map[string]string{"room": "kueche", "floor": "eg"},
	}

	md := ConvertToMetrics("1/1/2", cfg, 230.5, "1.1.5")

	if md.ResourceMetrics().Len() != 1 {
		t.Fatalf("expected 1 ResourceMetrics, got %d", md.ResourceMetrics().Len())
	}
	rm := md.ResourceMetrics().At(0)

	// Resource attribute
	physAddr, ok := rm.Resource().Attributes().Get("knx.physical_address")
	if !ok || physAddr.Str() != "1.1.5" {
		t.Errorf("knx.physical_address: got %v, want 1.1.5", physAddr)
	}

	sm := rm.ScopeMetrics().At(0)
	if sm.Scope().Name() != scopeName {
		t.Errorf("scope name: got %q, want %q", sm.Scope().Name(), scopeName)
	}

	m := sm.Metrics().At(0)
	if m.Name() != "leistung_eg_kueche_w" {
		t.Errorf("metric name: got %q", m.Name())
	}
	if m.Type() != pmetric.MetricTypeGauge {
		t.Errorf("expected Gauge, got %v", m.Type())
	}

	dp := m.Gauge().DataPoints().At(0)
	if dp.DoubleValue() != 230.5 {
		t.Errorf("value: got %v, want 230.5", dp.DoubleValue())
	}

	// Attributes
	ga, ok := dp.Attributes().Get("knx.group_address")
	if !ok || ga.Str() != "1/1/2" {
		t.Errorf("knx.group_address: got %v", ga)
	}
	room, ok := dp.Attributes().Get("room")
	if !ok || room.Str() != "kueche" {
		t.Errorf("room label: got %v", room)
	}
}

func TestConvertToMetrics_Sum(t *testing.T) {
	cfg := &AddressConfig{
		Name:       "strom_eg_kueche_wh",
		DPT:        "13.010",
		MetricType: MetricTypeSum,
		Labels:     map[string]string{},
	}

	md := ConvertToMetrics("1/1/1", cfg, 12345.0, "1.1.5")

	m := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	if m.Type() != pmetric.MetricTypeSum {
		t.Fatalf("expected Sum, got %v", m.Type())
	}

	s := m.Sum()
	if s.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		t.Errorf("expected Cumulative, got %v", s.AggregationTemporality())
	}
	if !s.IsMonotonic() {
		t.Error("expected IsMonotonic=true")
	}

	dp := s.DataPoints().At(0)
	if dp.DoubleValue() != 12345.0 {
		t.Errorf("value: got %v, want 12345.0", dp.DoubleValue())
	}
}

func TestConvertToMetrics_NoLabels(t *testing.T) {
	cfg := &AddressConfig{
		Name:       "temp",
		DPT:        "9.001",
		MetricType: MetricTypeGauge,
		Labels:     nil,
	}

	md := ConvertToMetrics("2/1/1", cfg, 22.0, "0.0.0")
	dp := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)

	// Only knx.group_address attribute (no user labels)
	if dp.Attributes().Len() != 1 {
		t.Errorf("expected 1 attribute, got %d", dp.Attributes().Len())
	}
}
