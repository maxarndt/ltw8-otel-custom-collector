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
func (s *FroniusScraper) Scrape(ctx context.Context) (*ScrapedMetrics, error) {
	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
	}

	// 1. PowerFlow Realtime Data
	if s.metrics.PowerFlow {
		if pf, err := s.getPowerFlowRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch PowerFlowRealtimeData", zap.Error(err))
		} else {
			scraped.PowerFlow = pf
		}
	}

	// 2. Inverter Realtime Data (CommonInverterData)
	if s.metrics.InverterRealtime {
		if inv, err := s.getInverterRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch InverterRealtimeData", zap.Error(err))
		} else {
			scraped.Inverter = inv
		}
	}

	// 3. Meter Realtime Data
	if s.metrics.MeterRealtime {
		if meter, err := s.getMeterRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch MeterRealtimeData", zap.Error(err))
		} else {
			scraped.Meter = meter
		}
	}

	// 4. Storage Realtime Data
	if s.metrics.StorageRealtime {
		if storage, err := s.getStorageRealtimeData(ctx); err != nil {
			s.logger.Warn("failed to fetch StorageRealtimeData", zap.Error(err))
		} else {
			scraped.Storage = storage
		}
	}

	// 5. Inverter Info
	if s.metrics.InverterInfo {
		if info, err := s.getInverterInfo(ctx); err != nil {
			s.logger.Warn("failed to fetch InverterInfo", zap.Error(err))
		} else {
			scraped.Info = info
		}
	}

	return scraped, nil
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

// getInverterRealtimeData fetcht Inverter Realtime Daten.
func (s *FroniusScraper) getInverterRealtimeData(ctx context.Context) (*InverterRealtimeData, error) {
	path := "/solar_api/v1/GetInverterRealtimeData.cgi"
	params := url.Values{
		"Scope":          {"Device"},
		"DeviceId":       {"1"},
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
