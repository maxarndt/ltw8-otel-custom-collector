package syrreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// newTestScraper erstellt einen Scraper der gegen den übergebenen Server-URL zeigt.
func newTestScraper(endpoint string, metrics MetricsConfig) *syrScraper {
	return &syrScraper{
		cfg: &Config{
			Endpoint: endpoint,
			Timeout:  5 * time.Second,
			Metrics:  metrics,
		},
		logger:     zap.NewNop(),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// syrMockServer erstellt einen HTTP-Test-Server der SYR API-Antworten liefert.
// responses mappt Feldnamen ("FLO") auf ihren numerischen Wert.
func syrMockServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Erwartet: GET /neosoft/get/{field}
		parts := []byte(r.URL.Path)
		field := string(parts[len("/neosoft/get/"):])

		val, ok := responses[field]
		if !ok {
			http.Error(w, "unknown field", http.StatusNotFound)
			return
		}

		key := "get" + field
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{key: val})
	}))
}

func TestParseSyrResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		want    float64
		wantErr bool
	}{
		{
			name:  "float value",
			body:  `{"getFLO": 15.5}`,
			field: "FLO",
			want:  15.5,
		},
		{
			name:  "integer value",
			body:  `{"getVOL": 12345}`,
			field: "VOL",
			want:  12345.0,
		},
		{
			name:  "string value",
			body:  `{"getIWH": "14"}`,
			field: "IWH",
			want:  14.0,
		},
		{
			name:  "zero value",
			body:  `{"getALA": 0}`,
			field: "ALA",
			want:  0.0,
		},
		{
			name:    "missing key",
			body:    `{"getFOO": 1}`,
			field:   "FLO",
			wantErr: true,
		},
		{
			name:    "invalid json",
			body:    `not-json`,
			field:   "FLO",
			wantErr: true,
		},
		{
			name:  "hex string FF (ALA = no alarm)",
			body:  `{"getALA": "FF"}`,
			field: "ALA",
			want:  255.0,
		},
		{
			name:  "hex string 00 (ALA = no alarm variant)",
			body:  `{"getALA": "00"}`,
			field: "ALA",
			want:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSyrResponse([]byte(tt.body), tt.field)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScraper_Scrape(t *testing.T) {
	srv := syrMockServer(t, map[string]interface{}{
		"VOL": 99999.0,
		"RE1": 500.0,
		"SS1": 8.0,
		"SV1": 12.5,
		"ALA": 0.0,
		"RG1": 0.0,
	})
	defer srv.Close()

	// FLO, IWH, OWH deaktiviert (wie vom User gewünscht)
	metrics := MetricsConfig{
		FLO: MetricConfig{Enabled: false},
		VOL: MetricConfig{Enabled: true},
		RE1: MetricConfig{Enabled: true},
		IWH: MetricConfig{Enabled: false},
		OWH: MetricConfig{Enabled: false},
		SS1: MetricConfig{Enabled: true},
		SV1: MetricConfig{Enabled: true},
		ALA: MetricConfig{Enabled: true},
		RG1: MetricConfig{Enabled: true},
	}

	s := newTestScraper(srv.URL, metrics)
	md, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("Scrape error: %v", err)
	}

	// 6 Metriken aktiviert (VOL, RE1, SS1, SV1, ALA, RG1)
	if md.DataPointCount() != 6 {
		t.Errorf("expected 6 data points, got %d", md.DataPointCount())
	}

	// VOL muss ein Sum (monoton, kumulativ) sein
	sm := md.ResourceMetrics().At(0).ScopeMetrics().At(0)
	var volMetric pmetric.Metric
	for i := 0; i < sm.Metrics().Len(); i++ {
		m := sm.Metrics().At(i)
		if m.Name() == "syr.total_volume" {
			volMetric = m
		}
	}
	if volMetric.Type() != pmetric.MetricTypeSum {
		t.Errorf("syr.total_volume: expected Sum, got %v", volMetric.Type())
	}
	if !volMetric.Sum().IsMonotonic() {
		t.Error("syr.total_volume: expected IsMonotonic=true")
	}
	if volMetric.Sum().DataPoints().At(0).DoubleValue() != 99999.0 {
		t.Errorf("syr.total_volume value: got %v", volMetric.Sum().DataPoints().At(0).DoubleValue())
	}
}

func TestScraper_PartialFailure(t *testing.T) {
	// Server liefert nur FLO — alle anderen Felder geben 404
	srv := syrMockServer(t, map[string]interface{}{
		"FLO": 10.0,
	})
	defer srv.Close()

	allEnabled := MetricConfig{Enabled: true}
	metrics := MetricsConfig{
		FLO: allEnabled, VOL: allEnabled, RE1: allEnabled,
		IWH: allEnabled, OWH: allEnabled, SS1: allEnabled,
		SV1: allEnabled, ALA: allEnabled, RG1: allEnabled,
	}

	s := newTestScraper(srv.URL, metrics)
	md, err := s.Scrape(context.Background())
	// Kein Gesamtfehler — fehlgeschlagene Einzelabfragen werden geloggt und übersprungen
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Nur FLO hat geantwortet
	if md.DataPointCount() != 1 {
		t.Errorf("expected 1 data point, got %d", md.DataPointCount())
	}
}

func TestScraper_AllDisabled(t *testing.T) {
	srv := syrMockServer(t, map[string]interface{}{})
	defer srv.Close()

	disabled := MetricConfig{Enabled: false}
	metrics := MetricsConfig{
		FLO: disabled, VOL: disabled, RE1: disabled,
		IWH: disabled, OWH: disabled, SS1: disabled,
		SV1: disabled, ALA: disabled, RG1: disabled,
	}

	s := newTestScraper(srv.URL, metrics)
	md, err := s.Scrape(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if md.DataPointCount() != 0 {
		t.Errorf("expected 0 data points, got %d", md.DataPointCount())
	}
}
