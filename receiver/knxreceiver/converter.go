package knxreceiver

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const scopeName = "github.com/maxarndt/ltw8-otel-custom-collector/receiver/knxreceiver"

// ConvertToMetrics builds a pmetric.Metrics from a decoded KNX group event value.
// physicalAddr is the sender's KNX individual address (e.g. "1.1.5") and is attached
// as a per-data-point attribute since it varies per telegram.
// startTs is the receiver's start timestamp; it is set on cumulative Sum data points
// so the Prometheus exporter can detect counter resets correctly.
// receiverID identifies this receiver instance and is stable for its lifetime; it is
// attached as a Resource attribute (service.instance.id).
func ConvertToMetrics(
	groupAddr string,
	cfg *AddressConfig,
	value float64,
	physicalAddr string,
	startTs pcommon.Timestamp,
	receiverID string,
) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	if receiverID != "" {
		rm.Resource().Attributes().PutStr("service.instance.id", receiverID)
	}

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(scopeName)

	m := sm.Metrics().AppendEmpty()
	m.SetName(cfg.Name)

	now := pcommon.NewTimestampFromTime(time.Now())

	switch cfg.MetricType {
	case MetricTypeGauge:
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(now)
		setAttributes(dp.Attributes(), groupAddr, physicalAddr, cfg.Labels)

	case MetricTypeSum:
		s := m.SetEmptySum()
		s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		s.SetIsMonotonic(true)
		dp := s.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(now)
		dp.SetStartTimestamp(startTs)
		setAttributes(dp.Attributes(), groupAddr, physicalAddr, cfg.Labels)
	}

	return md
}

func setAttributes(attrs pcommon.Map, groupAddr, physicalAddr string, labels map[string]string) {
	attrs.PutStr("knx.group_address", groupAddr)
	attrs.PutStr("knx.physical_address", physicalAddr)
	for k, v := range labels {
		attrs.PutStr(k, v)
	}
}
