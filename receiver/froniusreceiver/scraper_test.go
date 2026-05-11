package froniusreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestScraperPowerFlow(t *testing.T) {
	logger := zap.NewNop()

	// Create mock HTTP server
	mockResp := ResponseEnvelope{
		Head: Head{
			Status:    Status{Code: 0},
			TimeStamp: time.Now().Format(time.RFC3339),
		},
		Body: BodyWrapper{
			Data: PowerFlowRealtimeData{
				Site: PowerFlowSite{
					P_PV:    5000.0,
					P_Grid:  -500.0,
					P_Load:  4500.0,
					P_Akku:  0.0,
					E_Total: 1000000.0,
				},
				Inverters: map[string]PowerFlowInverter{
					"1": {
						P:       5000.0,
						E_Total: 1000000.0,
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	scraper, err := NewFroniusScraper(
		server.URL,
		10*time.Second,
		MetricsConfig{PowerFlow: true},
		logger,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result, err := scraper.getPowerFlowRealtimeData(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 5000.0, result.Site.P_PV)
}

func TestScraperOhmpilot(t *testing.T) {
	logger := zap.NewNop()

	mockResp := ResponseEnvelope{
		Head: Head{Status: Status{Code: 0}},
		Body: BodyWrapper{
			Data: OhmpilotRealtimeData{
				"0": {
					Details:                     OhmpilotDetails{Manufacturer: "Fronius", Model: "Ohmpilot", Serial: "X"},
					CodeOfState:                 0,
					EnergyReal_WAC_Sum_Consumed: 99999,
					PowerReal_PAC_Sum:           1234,
					Temperature_Channel_1:       55,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	scraper, err := NewFroniusScraper(server.URL, 10*time.Second, MetricsConfig{OhmpilotRealtime: true}, logger)
	assert.NoError(t, err)

	result, err := scraper.getOhmpilotRealtimeData(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1234.0, result["0"].PowerReal_PAC_Sum)
}

func TestScraperCollectInverterIDsFallback(t *testing.T) {
	s := &FroniusScraper{logger: zap.NewNop()}
	ids := s.collectInverterIDs(&ScrapedMetrics{})
	assert.Equal(t, []string{"1"}, ids, "Fallback-ID muss '1' sein")
}

func TestScraperCollectInverterIDsFromInfo(t *testing.T) {
	s := &FroniusScraper{logger: zap.NewNop()}
	ids := s.collectInverterIDs(&ScrapedMetrics{
		Info: InverterInfoData{
			"1": {UniqueID: "A"},
			"2": {UniqueID: "B"},
		},
	})
	assert.ElementsMatch(t, []string{"1", "2"}, ids)
}

func TestScraperErrorHandling(t *testing.T) {
	logger := zap.NewNop()

	// Create mock HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	scraper, err := NewFroniusScraper(
		server.URL,
		10*time.Second,
		MetricsConfig{PowerFlow: true},
		logger,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result, err := scraper.getPowerFlowRealtimeData(ctx)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestScraperTimeout(t *testing.T) {
	logger := zap.NewNop()

	// Create mock HTTP server that is slow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&ResponseEnvelope{
			Head: Head{Status: Status{Code: 0}},
		})
	}))
	defer server.Close()

	// Short timeout
	scraper, err := NewFroniusScraper(
		server.URL,
		5*time.Millisecond,
		MetricsConfig{PowerFlow: true},
		logger,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	result, err := scraper.getPowerFlowRealtimeData(ctx)
	assert.Error(t, err)
	assert.Nil(t, result)
}
