package syrreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	scopeName = "github.com/maxarndt/knxreceiver/receiver/syrreceiver"
)

// metricDef beschreibt eine bekannte SYR-Metrik.
type metricDef struct {
	// field ist der SYR API-Feldname (z.B. "FLO").
	field string
	// name ist der OTEL Metric-Name (z.B. "syr.flow_rate").
	name string
	// unit ist die Einheit des Messwertes.
	unit string
	// isSum gibt an ob die Metrik ein kumulativer Zähler (Sum, monoton) ist.
	// false = Gauge (Momentanwert).
	isSum bool
	// isEnabled prüft ob diese Metrik in der Config aktiviert ist.
	isEnabled func(*MetricsConfig) bool
}

// knownMetrics definiert alle bekannten SYR NeoSoft 2500 Metriken.
var knownMetrics = []metricDef{
	{
		field: "FLO", name: "syr.flow_rate", unit: "l/h", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.FLO.Enabled },
	},
	{
		field: "VOL", name: "syr.total_volume", unit: "l", isSum: true,
		isEnabled: func(m *MetricsConfig) bool { return m.VOL.Enabled },
	},
	{
		field: "RE1", name: "syr.reserve_capacity", unit: "l", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.RE1.Enabled },
	},
	{
		field: "IWH", name: "syr.input_hardness", unit: "dH", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.IWH.Enabled },
	},
	{
		field: "OWH", name: "syr.output_hardness", unit: "dH", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.OWH.Enabled },
	},
	{
		field: "SS1", name: "syr.salt_weeks_remaining", unit: "weeks", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.SS1.Enabled },
	},
	{
		field: "SV1", name: "syr.salt_kg", unit: "kg", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.SV1.Enabled },
	},
	{
		field: "ALA", name: "syr.alarm", unit: "1", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.ALA.Enabled },
	},
	{
		field: "RG1", name: "syr.regeneration_status", unit: "1", isSum: false,
		isEnabled: func(m *MetricsConfig) bool { return m.RG1.Enabled },
	},
}

// syrScraper ruft Metriken vom SYR NeoSoft 2500 per HTTP ab.
type syrScraper struct {
	cfg        *Config
	logger     *zap.Logger
	httpClient *http.Client
}

func newSyrScraper(cfg *Config, logger *zap.Logger) *syrScraper {
	return &syrScraper{
		cfg:    cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Scrape ruft alle aktivierten Metriken ab und gibt sie als pmetric.Metrics zurück.
// Einzelne fehlgeschlagene Abfragen werden geloggt und übersprungen; es wird kein
// Gesamtfehler zurückgegeben, damit der Rest der Metriken weiterhin geliefert wird.
func (s *syrScraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("syr.endpoint", s.cfg.Endpoint)

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(scopeName)

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, def := range knownMetrics {
		if !def.isEnabled(&s.cfg.Metrics) {
			continue
		}

		value, err := s.fetchField(ctx, def.field)
		if err != nil {
			s.logger.Warn("SYR field fetch failed",
				zap.String("field", def.field),
				zap.Error(err))
			continue
		}

		m := sm.Metrics().AppendEmpty()
		m.SetName(def.name)
		m.SetUnit(def.unit)

		if def.isSum {
			sum := m.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			sum.SetIsMonotonic(true)
			dp := sum.DataPoints().AppendEmpty()
			dp.SetDoubleValue(value)
			dp.SetTimestamp(now)
		} else {
			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(value)
			dp.SetTimestamp(now)
		}
	}

	return md, nil
}

// fetchField ruft einen einzelnen SYR-Feldwert per HTTP ab und parst die JSON-Antwort.
// Antwortformat: {"getFLO": 15.5} — Wert kann Number oder String sein.
func (s *syrScraper) fetchField(ctx context.Context, field string) (float64, error) {
	url := fmt.Sprintf("%s/neosoft/get/%s",
		strings.TrimRight(s.cfg.Endpoint, "/"), field)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d for field %s", resp.StatusCode, field)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}

	return parseSyrResponse(body, field)
}

// parseSyrResponse extrahiert den float64-Wert aus einer SYR JSON-Antwort.
// Die Antwort hat das Format {"get{FIELD}": value} — der Wert kann Number oder String sein.
func parseSyrResponse(body []byte, field string) (float64, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, fmt.Errorf("JSON unmarshal: %w", err)
	}

	key := "get" + field
	v, ok := raw[key]
	if !ok {
		return 0, fmt.Errorf("key %q not found in response", key)
	}

	return toFloat64(v)
}

// toFloat64 konvertiert json.Number, float64, string oder int-Typen zu float64.
// Strings werden zuerst als Dezimalzahl, dann als Hexadezimalzahl geparst.
// Das SYR-Protokoll liefert Alarmcodes als Hex-Strings (z.B. "FF" = 255 = kein Alarm).
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case json.Number:
		return val.Float64()
	case string:
		// Erst als Dezimalzahl versuchen.
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, nil
		}
		// Fallback: Hexadezimalzahl (SYR liefert z.B. "FF" für ALA).
		i, err := strconv.ParseInt(val, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as decimal or hex number", val)
		}
		return float64(i), nil
	case int:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", v)
	}
}
