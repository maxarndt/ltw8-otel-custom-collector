package froniusreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// FroniusScraper ist responsible für das Fetchen von Daten von der Fronius API.
type FroniusScraper struct {
	endpoint string
	timeout  time.Duration
	client   *http.Client
	metrics  MetricsConfig
	logger   *zap.Logger
}

// NewFroniusScraper erstellt einen neuen FroniusScraper.
func NewFroniusScraper(endpoint string, timeout time.Duration, metrics MetricsConfig, logger *zap.Logger) (*FroniusScraper, error) {
	return &FroniusScraper{
		endpoint: endpoint,
		timeout:  timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		metrics: metrics,
		logger:  logger,
	}, nil
}

// Scrape führt einen vollständigen Scrape-Zyklus durch.
// Reihenfolge der Calls ist relevant: InverterInfo zuerst (für Inverter-IDs).
func (s *FroniusScraper) Scrape(ctx context.Context) (*ScrapedMetrics, error) {
	start := time.Now()
	scraped := &ScrapedMetrics{
		Timestamp: start,
		Inverters: make(InverterRealtimeMap),
	}

	var errCount int64

	// 1. Inverter Info (zuerst — liefert Inverter-IDs für nachfolgende Calls)
	if s.metrics.InverterInfo {
		if info, err := s.getInverterInfo(ctx); err != nil {
			s.logger.Warn("failed to fetch InverterInfo", zap.Error(err))
			errCount++
		} else {
			scraped.Info = info
		}
	}

	// 2. PowerFlow Realtime Data (liefert ebenfalls Inverter-IDs als Fallback)
	if s.metrics.PowerFlow {
		if pf, err := s.getPowerFlowRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch PowerFlowRealtimeData", zap.Error(err))
			errCount++
		} else {
			scraped.PowerFlow = pf
		}
	}

	// 3. Inverter Realtime Data (CommonInverterData) — pro bekannter Inverter-ID
	if s.metrics.InverterRealtime {
		ids := s.collectInverterIDs(scraped)
		for _, id := range ids {
			if inv, err := s.getInverterRealtimeData(ctx, id); err != nil {
				s.logger.Warn("failed to fetch InverterRealtimeData",
					zap.String("device_id", id), zap.Error(err))
				errCount++
			} else {
				scraped.Inverters[id] = inv
			}
		}
	}

	// 4. Meter Realtime Data
	if s.metrics.MeterRealtime {
		if meter, err := s.getMeterRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch MeterRealtimeData", zap.Error(err))
			errCount++
		} else {
			scraped.Meter = meter
		}
	}

	// 5. Storage Realtime Data
	if s.metrics.StorageRealtime {
		if storage, err := s.getStorageRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch StorageRealtimeData", zap.Error(err))
			errCount++
		} else {
			scraped.Storage = storage
		}
	}

	// 6. Ohmpilot Realtime Data
	if s.metrics.OhmpilotRealtime {
		if ohm, err := s.getOhmpilotRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch OhmpilotRealtimeData", zap.Error(err))
			errCount++
		} else {
			scraped.Ohmpilot = ohm
		}
	}

	scraped.Stats = ScrapeStats{
		DurationSeconds: time.Since(start).Seconds(),
		Errors:          errCount,
		Success:         errCount == 0,
	}

	return scraped, nil
}

// collectInverterIDs sammelt Inverter-IDs aus Info bzw. PowerFlow.
// Fallback: ["1"] (Default Device ID).
func (s *FroniusScraper) collectInverterIDs(scraped *ScrapedMetrics) []string {
	idSet := map[string]struct{}{}
	if scraped.Info != nil {
		for id := range scraped.Info {
			idSet[id] = struct{}{}
		}
	}
	if scraped.PowerFlow != nil {
		for id := range scraped.PowerFlow.Inverters {
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return []string{"1"}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids
}

// getPowerFlowRealtimeData fetcht PowerFlow Daten von GetPowerFlowRealtimeData.fcgi.
func (s *FroniusScraper) getPowerFlowRealtimeData(ctx context.Context) (*PowerFlowRealtimeData, error) {
	path := "/solar_api/v1/GetPowerFlowRealtimeData.fcgi"
	data := &PowerFlowRealtimeData{}
	if err := s.fetchJSON(ctx, path, nil, data); err != nil {
		return nil, err
	}
	return data, nil
}

// getInverterRealtimeData fetcht Inverter Realtime Daten für eine spezifische Device-ID.
func (s *FroniusScraper) getInverterRealtimeData(ctx context.Context, deviceID string) (*InverterRealtimeData, error) {
	path := "/solar_api/v1/GetInverterRealtimeData.cgi"
	params := url.Values{
		"Scope":          {"Device"},
		"DeviceId":       {deviceID},
		"DataCollection": {"CommonInverterData"},
	}
	data := &InverterRealtimeData{}
	if err := s.fetchJSON(ctx, path, params, data); err != nil {
		return nil, err
	}
	return data, nil
}

// getMeterRealtimeData fetcht Smart Meter Daten.
func (s *FroniusScraper) getMeterRealtimeData(ctx context.Context) (MeterRealtimeData, error) {
	path := "/solar_api/v1/GetMeterRealtimeData.cgi"
	params := url.Values{
		"Scope": {"System"},
	}
	data := make(MeterRealtimeData)
	if err := s.fetchJSON(ctx, path, params, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// getStorageRealtimeData fetcht Storage/Battery Daten.
func (s *FroniusScraper) getStorageRealtimeData(ctx context.Context) (StorageRealtimeData, error) {
	path := "/solar_api/v1/GetStorageRealtimeData.cgi"
	params := url.Values{
		"Scope": {"System"},
	}
	data := make(StorageRealtimeData)
	if err := s.fetchJSON(ctx, path, params, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// getOhmpilotRealtimeData fetcht Ohmpilot Daten.
func (s *FroniusScraper) getOhmpilotRealtimeData(ctx context.Context) (OhmpilotRealtimeData, error) {
	path := "/solar_api/v1/GetOhmpilotRealtimeData.cgi"
	params := url.Values{
		"Scope": {"System"},
	}
	data := make(OhmpilotRealtimeData)
	if err := s.fetchJSON(ctx, path, params, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// getInverterInfo fetcht Inverter Info (Metadaten).
func (s *FroniusScraper) getInverterInfo(ctx context.Context) (InverterInfoData, error) {
	path := "/solar_api/v1/GetInverterInfo.cgi"
	data := make(InverterInfoData)
	if err := s.fetchJSON(ctx, path, nil, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// fetchJSON führt einen generischen HTTP GET Request durch und parst die JSON Response.
func (s *FroniusScraper) fetchJSON(ctx context.Context, path string, params url.Values, out interface{}) error {
	fullURL := s.endpoint + path
	if params != nil {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	// Parse generischen ResponseEnvelope
	var env ResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check Status Code in Response Header
	if env.Head.Status.Code != 0 {
		return fmt.Errorf("API error code %d: %s", env.Head.Status.Code, env.Head.Status.Reason)
	}

	// Re-marshal Data into target type
	if env.Body.Data == nil {
		return fmt.Errorf("empty data in response")
	}

	dataBytes, err := json.Marshal(env.Body.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, out); err != nil {
		return fmt.Errorf("failed to unmarshal into target type: %w", err)
	}

	return nil
}
