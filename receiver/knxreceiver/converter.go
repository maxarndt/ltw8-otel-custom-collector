package knxreceiver

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const scopeName = "github.com/maxarndt/knxreceiver"

// ConvertToMetrics builds a pmetric.Metrics from a decoded KNX group event value.
// physicalAddr is the sender's KNX individual address (e.g. "1.1.5").
func ConvertToMetrics(
	groupAddr string,
	cfg *AddressConfig,
	value float64,
	physicalAddr string,
) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("knx.physical_address", physicalAddr)

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
		setAttributes(dp.Attributes(), groupAddr, cfg.Labels)

	case MetricTypeSum:
		s := m.SetEmptySum()
		s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		s.SetIsMonotonic(true)
		dp := s.DataPoints().AppendEmpty()
		dp.SetDoubleValue(value)
		dp.SetTimestamp(now)
		setAttributes(dp.Attributes(), groupAddr, cfg.Labels)
	}

	return md
}

func setAttributes(attrs pcommon.Map, groupAddr string, labels map[string]string) {
	attrs.PutStr("knx.group_address", groupAddr)
	for k, v := range labels {
		attrs.PutStr(k, v)
	}
}
