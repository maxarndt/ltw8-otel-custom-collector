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
