package froniusreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestConverterPowerFlow(t *testing.T) {
	logger := zap.NewNop()
	converter := NewConverter(logger)

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		PowerFlow: &PowerFlowRealtimeData{
			Site: PowerFlowSite{
				P_PV:                5000.0,
				P_Grid:              -500.0,
				P_Load:              4500.0,
				P_Akku:              0.0,
				E_Total:             1000000.0,
				E_Year:              50000.0,
				E_Day:               5000.0,
				Rel_Autonomy:        90.0,
				Rel_SelfConsumption: 85.0,
			},
			Inverters: map[string]PowerFlowInverter{
				"1": {
					P:       5000.0,
					SOC:     80.0,
					E_Total: 1000000.0,
					E_Year:  50000.0,
					E_Day:   5000.0,
				},
			},
		},
	}

	ctx := context.Background()
	metrics := converter.ConvertToMetrics(ctx, scraped)

	// Check that metrics were created
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.MetricCount(), 0)

	// Verify we have site and inverter metrics
	resourceMetrics := metrics.ResourceMetrics()
	assert.Equal(t, 1, resourceMetrics.Len())
}

func TestConverterMeter(t *testing.T) {
	logger := zap.NewNop()
	converter := NewConverter(logger)

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Meter: MeterRealtimeData{
			"0": {
				Details: &MeterDetails{
					Manufacturer: "Fronius",
					Model:        "Smart Meter",
					Serial:       "12345",
				},
				Current_AC_Phase_1:  ptrFloat(10.5),
				Current_AC_Phase_2:  ptrFloat(11.2),
				Current_AC_Phase_3:  ptrFloat(9.8),
				Voltage_AC_Phase_1:  ptrFloat(230.0),
				PowerReal_P_Phase_1: ptrFloat(2415.0),
			},
		},
	}

	ctx := context.Background()
	metrics := converter.ConvertToMetrics(ctx, scraped)

	// Check that metrics were created
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.MetricCount(), 0)
}

func TestConverterStorage(t *testing.T) {
	logger := zap.NewNop()
	converter := NewConverter(logger)

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Storage: StorageRealtimeData{
			"0": {
				Controller: StorageController{
					Details: StorageDetails{
						Manufacturer: "BYD",
						Model:        "HV",
						Serial:       "67890",
					},
					StateOfCharge_Relative: 75.0,
					Voltage_DC:             400.0,
					Current_DC:             5.0,
					Temperature_Cell:       20.0,
					Capacity_Maximum:       10000.0,
					Status_BatteryCell:     0.0,
				},
				Modules: []interface{}{},
			},
		},
	}

	ctx := context.Background()
	metrics := converter.ConvertToMetrics(ctx, scraped)

	// Check that metrics were created
	assert.NotNil(t, metrics)
	assert.Greater(t, metrics.MetricCount(), 0)
}

// Helper function to create pointer to float64
func ptrFloat(f float64) *float64 {
	return &f
}
