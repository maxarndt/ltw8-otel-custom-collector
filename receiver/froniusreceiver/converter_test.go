package froniusreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// findMetric durchsucht alle ResourceMetrics nach einer Metrik mit gegebenem Namen.
// Liefert die erste gefundene Metrik oder ein Zero-Value falls nicht vorhanden.
func findMetric(metrics pmetric.Metrics, name string) (pmetric.Metric, bool) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				if m.Name() == name {
					return m, true
				}
			}
		}
	}
	return pmetric.Metric{}, false
}

// countMetric zählt alle Metriken mit gegebenem Namen über alle ResourceMetrics.
func countMetric(metrics pmetric.Metrics, name string) int {
	count := 0
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				if sm.Metrics().At(k).Name() == name {
					count++
				}
			}
		}
	}
	return count
}

func TestConverterSiteMetrics(t *testing.T) {
	converter := NewConverterWithEndpoint(zap.NewNop(), "http://test")

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
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_site_pv_power")
	assert.True(t, ok)
	assert.Equal(t, "W", m.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, m.Type())
	assert.Equal(t, 5000.0, m.Gauge().DataPoints().At(0).DoubleValue())

	m, ok = findMetric(metrics, "fronius_site_energy_total")
	assert.True(t, ok)
	assert.Equal(t, "Wh", m.Unit())
	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	assert.True(t, m.Sum().IsMonotonic())
}

func TestConverterInverterEnergyNotDuplicated(t *testing.T) {
	// Wenn sowohl PowerFlow als auch InverterRealtime aktiv sind, darf
	// fronius_inverter_energy_total NUR einmal pro Inverter emittiert werden
	// (aus InverterRealtime).
	converter := NewConverter(zap.NewNop())

	totalEnergy := 1000000.0
	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		PowerFlow: &PowerFlowRealtimeData{
			Inverters: map[string]PowerFlowInverter{
				"1": {P: 5000.0, E_Total: totalEnergy, E_Year: 50000, E_Day: 5000, SOC: 80},
			},
		},
		Inverters: InverterRealtimeMap{
			"1": {
				PAC:          &DataPoint{Value: 5000, Unit: "W"},
				TOTAL_ENERGY: &DataPoint{Value: totalEnergy, Unit: "Wh"},
				YEAR_ENERGY:  &DataPoint{Value: 50000, Unit: "Wh"},
				DAY_ENERGY:   &DataPoint{Value: 5000, Unit: "Wh"},
			},
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	// Inverter-Energy-Metriken: erwartet genau 1 Vorkommen (aus InverterRealtime)
	assert.Equal(t, 1, countMetric(metrics, "fronius_inverter_energy_total"),
		"fronius_inverter_energy_total muss genau einmal vorkommen, nicht doppelt")
	assert.Equal(t, 1, countMetric(metrics, "fronius_inverter_energy_year"))
	assert.Equal(t, 1, countMetric(metrics, "fronius_inverter_energy_day"))
}

func TestConverterInverterResourceAttributes(t *testing.T) {
	converter := NewConverterWithEndpoint(zap.NewNop(), "http://test")

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Info: InverterInfoData{
			"1": {
				CustomName: "Wechselrichter Süd",
				UniqueID:   "INV-SERIAL-12345",
				DT:         99,
			},
		},
		Inverters: InverterRealtimeMap{
			"1": {PAC: &DataPoint{Value: 1234, Unit: "W"}},
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	// Suche das ResourceMetrics für den Inverter
	found := false
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		comp, exists := rm.Resource().Attributes().Get("fronius.component")
		if !exists || comp.Str() != "inverter" {
			continue
		}
		serial, _ := rm.Resource().Attributes().Get("fronius.inverter.serial")
		assert.Equal(t, "INV-SERIAL-12345", serial.Str())
		name, _ := rm.Resource().Attributes().Get("fronius.inverter.custom_name")
		assert.Equal(t, "Wechselrichter Süd", name.Str())
		ep, _ := rm.Resource().Attributes().Get("fronius.endpoint")
		assert.Equal(t, "http://test", ep.Str())
		found = true
	}
	assert.True(t, found, "Es muss ein Inverter-ResourceMetrics existieren")
}

func TestConverterMeter(t *testing.T) {
	converter := NewConverter(zap.NewNop())

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

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_meter_current")
	assert.True(t, ok)
	assert.Equal(t, "A", m.Unit())
}

func TestConverterStorage(t *testing.T) {
	converter := NewConverter(zap.NewNop())

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

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_battery_soc")
	assert.True(t, ok)
	assert.Equal(t, "%", m.Unit())
	assert.Equal(t, 75.0, m.Gauge().DataPoints().At(0).DoubleValue())
}

func TestConverterOhmpilot(t *testing.T) {
	converter := NewConverter(zap.NewNop())

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Ohmpilot: OhmpilotRealtimeData{
			"0": {
				Details: OhmpilotDetails{
					Manufacturer: "Fronius",
					Model:        "Ohmpilot",
					Serial:       "OHM-1",
				},
				CodeOfState:                 0,
				EnergyReal_WAC_Sum_Consumed: 12345.0,
				PowerReal_PAC_Sum:           1500.0,
				Temperature_Channel_1:       55.0,
			},
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_ohmpilot_power")
	assert.True(t, ok)
	assert.Equal(t, "W", m.Unit())
	assert.Equal(t, 1500.0, m.Gauge().DataPoints().At(0).DoubleValue())

	m, ok = findMetric(metrics, "fronius_ohmpilot_energy_consumed")
	assert.True(t, ok)
	assert.Equal(t, "Wh", m.Unit())
	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	assert.True(t, m.Sum().IsMonotonic())

	m, ok = findMetric(metrics, "fronius_ohmpilot_temperature")
	assert.True(t, ok)
	assert.Equal(t, "Cel", m.Unit())
}

func TestConverterEmptyOhmpilot(t *testing.T) {
	// Wenn kein Ohmpilot installiert ist, liefert die API leeres Map.
	// Code muss das tolerieren ohne Panic.
	converter := NewConverter(zap.NewNop())

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Ohmpilot:  OhmpilotRealtimeData{},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)
	_, ok := findMetric(metrics, "fronius_ohmpilot_power")
	assert.False(t, ok, "Bei leerem Ohmpilot dürfen keine Ohmpilot-Metriken entstehen")
}

func TestConverterScrapeTelemetry(t *testing.T) {
	converter := NewConverter(zap.NewNop())

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		Stats: ScrapeStats{
			DurationSeconds: 1.234,
			Errors:          2,
			Success:         false,
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_scrape_duration_seconds")
	assert.True(t, ok)
	assert.Equal(t, "s", m.Unit())
	assert.Equal(t, 1.234, m.Gauge().DataPoints().At(0).DoubleValue())

	m, ok = findMetric(metrics, "fronius_scrape_errors_total")
	assert.True(t, ok)
	assert.Equal(t, pmetric.MetricTypeSum, m.Type())
	assert.Equal(t, int64(2), m.Sum().DataPoints().At(0).IntValue())

	m, ok = findMetric(metrics, "fronius_scrape_success")
	assert.True(t, ok)
	assert.Equal(t, 0.0, m.Gauge().DataPoints().At(0).DoubleValue())
}

func TestConverterSOCAlwaysEmitted(t *testing.T) {
	// SOC=0 ist legitime Information (Batterie leer), darf nicht weggefiltert werden.
	converter := NewConverter(zap.NewNop())

	scraped := &ScrapedMetrics{
		Timestamp: time.Now(),
		PowerFlow: &PowerFlowRealtimeData{
			Inverters: map[string]PowerFlowInverter{
				"1": {SOC: 0.0, P: 0.0},
			},
		},
	}

	metrics := converter.ConvertToMetrics(context.Background(), scraped)

	m, ok := findMetric(metrics, "fronius_inverter_soc")
	assert.True(t, ok, "SOC muss auch bei Wert 0 emittiert werden")
	assert.Equal(t, 0.0, m.Gauge().DataPoints().At(0).DoubleValue())
}

// ptrFloat is a helper for tests that need *float64 values.
func ptrFloat(f float64) *float64 {
	return &f
}
