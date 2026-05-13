package froniusreceiver

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Converter konvertiert Fronius API Daten zu OTEL pmetric.Metrics.
//
// Designprinzipien:
//   - Pro logischer Komponente (Site, Inverter, Meter, Storage, Ohmpilot) wird ein
//     eigenes ResourceMetrics erzeugt mit jeweiligen Resource-Attributen
//     (z.B. Serial, Model). Das ist OTEL-idiomatisch und ermöglicht saubere
//     Filterung in Backends.
//   - UCUM Units werden konsequent gesetzt (W, V, A, Hz, Wh, Cel, %, 1, var, VA).
//   - Metric-Namen tragen KEINE Unit-Suffixe (Unit ist eigenständiges Feld).
//   - Inverter-Energien kommen ausschließlich aus InverterRealtime (nicht doppelt
//     emittiert via PowerFlow-Inverter-Loop).
type Converter struct {
	logger   *zap.Logger
	endpoint string // für Resource-Attribut fronius.endpoint
}

// NewConverter erstellt einen neuen Converter.
func NewConverter(logger *zap.Logger) *Converter {
	return &Converter{logger: logger}
}

// NewConverterWithEndpoint erstellt einen Converter mit Endpoint-Resource-Attribut.
func NewConverterWithEndpoint(logger *zap.Logger, endpoint string) *Converter {
	return &Converter{logger: logger, endpoint: endpoint}
}

// ConvertToMetrics konvertiert ScrapedMetrics zu pmetric.Metrics.
func (c *Converter) ConvertToMetrics(_ context.Context, scraped *ScrapedMetrics) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	now := pcommon.NewTimestampFromTime(scraped.Timestamp)

	// Site-Level (PowerFlow Site-Daten)
	if scraped.PowerFlow != nil {
		c.convertSite(metrics, &scraped.PowerFlow.Site, now)
		// Pro-Inverter Übersichtsmetriken aus PowerFlow (P, SOC) — Energien NICHT
		// hier, die kommen aus InverterRealtime. AC-Power nur falls kein
		// InverterRealtime existiert (sonst Duplikat zu convertInverter).
		hasRealtime := make(map[string]bool, len(scraped.Inverters))
		for id, inv := range scraped.Inverters {
			if inv != nil {
				hasRealtime[id] = true
			}
		}
		c.convertPowerFlowInverters(metrics, scraped.PowerFlow.Inverters, scraped.Info, hasRealtime, now)
	}

	// Pro Inverter ein eigenes ResourceMetrics für Realtime-Daten
	for id, inv := range scraped.Inverters {
		if inv == nil {
			continue
		}
		c.convertInverter(metrics, id, inv, scraped.Info, now)
	}

	// Meter
	if len(scraped.Meter) > 0 {
		c.convertMeter(metrics, scraped.Meter, now)
	}

	// Storage
	if len(scraped.Storage) > 0 {
		c.convertStorage(metrics, scraped.Storage, now)
	}

	// Ohmpilot
	if len(scraped.Ohmpilot) > 0 {
		c.convertOhmpilot(metrics, scraped.Ohmpilot, now)
	}

	// Scrape Telemetry
	c.convertScrapeTelemetry(metrics, &scraped.Stats, now)

	return metrics
}

// ======================== Resource Builders ========================

func (c *Converter) newResource(metrics pmetric.Metrics, attrs map[string]string) pmetric.MetricSlice {
	rm := metrics.ResourceMetrics().AppendEmpty()
	if c.endpoint != "" {
		rm.Resource().Attributes().PutStr("fronius.endpoint", c.endpoint)
	}
	for k, v := range attrs {
		if v == "" {
			continue
		}
		rm.Resource().Attributes().PutStr(k, v)
	}
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("github.com/maxarndt/ltw8-otel-custom-collector/receiver/froniusreceiver")
	return sm.Metrics()
}

// ======================== Site Conversion ========================

func (c *Converter) convertSite(metrics pmetric.Metrics, site *PowerFlowSite, now pcommon.Timestamp) {
	ms := c.newResource(metrics, map[string]string{
		"fronius.component": "site",
	})

	c.addGauge(ms, "fronius_site_pv_power", "PV power", "W", site.P_PV, nil, now)
	c.addGauge(ms, "fronius_site_grid_power", "Grid power (negative=import)", "W", site.P_Grid, nil, now)
	c.addGauge(ms, "fronius_site_load_power", "Load power", "W", site.P_Load, nil, now)
	c.addGauge(ms, "fronius_site_battery_power", "Battery power (negative=charging)", "W", site.P_Akku, nil, now)
	c.addSum(ms, "fronius_site_energy_total", "Total energy produced", "Wh", site.E_Total, nil, now, true)
	c.addSum(ms, "fronius_site_energy_year", "Energy produced this year", "Wh", site.E_Year, nil, now, true)
	c.addSum(ms, "fronius_site_energy_day", "Energy produced today", "Wh", site.E_Day, nil, now, true)
	c.addGauge(ms, "fronius_site_autonomy_ratio", "Autonomy ratio (0-100)", "%", site.Rel_Autonomy, nil, now)
	c.addGauge(ms, "fronius_site_selfconsumption_ratio", "Self-consumption ratio (0-100)", "%", site.Rel_SelfConsumption, nil, now)
}

// ======================== PowerFlow per-Inverter (Übersicht) ========================

// convertPowerFlowInverters emittiert pro Inverter NUR die Werte, die
// einzigartig aus PowerFlow stammen (SOC). AC Power und Energien kommen aus
// InverterRealtime — wären hier Duplikate mit identischen Resource-Attributen.
//
// Sonderfall: Falls InverterRealtime deaktiviert ist (kein convertInverter-Call
// für diesen Inverter), wird zusätzlich AC Power aus PowerFlow geliefert,
// damit zumindest die wichtigste Kennzahl da ist.
func (c *Converter) convertPowerFlowInverters(
	metrics pmetric.Metrics,
	inverters map[string]PowerFlowInverter,
	info InverterInfoData,
	hasRealtime map[string]bool,
	now pcommon.Timestamp,
) {
	for invID, inv := range inverters {
		ms := c.newResource(metrics, c.inverterResourceAttrs(invID, info))

		// SOC immer (nur in PowerFlow verfügbar)
		c.addGauge(ms, "fronius_inverter_soc", "State of charge", "%", inv.SOC, nil, now)

		// AC Power nur falls InverterRealtime fehlt (sonst Duplikat)
		if !hasRealtime[invID] {
			c.addGauge(ms, "fronius_inverter_ac_power", "Inverter AC power", "W", inv.P, nil, now)
		}

		// HINWEIS: Energien (E_Day/Year/Total) kommen aus convertInverter
		// (InverterRealtime). Falls dieser Inverter dort fehlt, gehen die
		// Energien verloren — das ist ein bewusster Tradeoff zur
		// Duplikat-Vermeidung. Lösung: InverterRealtime aktivieren.
	}
}

// ======================== Inverter Realtime Conversion ========================

func (c *Converter) convertInverter(
	metrics pmetric.Metrics,
	invID string,
	inv *InverterRealtimeData,
	info InverterInfoData,
	now pcommon.Timestamp,
) {
	ms := c.newResource(metrics, c.inverterResourceAttrs(invID, info))

	// AC
	if inv.PAC != nil {
		c.addGauge(ms, "fronius_inverter_ac_power", "Inverter AC power", "W", inv.PAC.Value, nil, now)
	}
	if inv.SAC != nil {
		c.addGauge(ms, "fronius_inverter_ac_apparent_power", "Inverter AC apparent power", "VA", inv.SAC.Value, nil, now)
	}
	if inv.UAC != nil {
		c.addGauge(ms, "fronius_inverter_ac_voltage", "Inverter AC voltage", "V", inv.UAC.Value, nil, now)
	}
	if inv.IAC != nil {
		c.addGauge(ms, "fronius_inverter_ac_current", "Inverter AC current", "A", inv.IAC.Value, nil, now)
	}
	if inv.FAC != nil {
		c.addGauge(ms, "fronius_inverter_ac_frequency", "Inverter AC frequency", "Hz", inv.FAC.Value, nil, now)
	}

	// 3-phasige Werte (falls 3PInverterData ergänzend geliefert wird)
	for phase, dp := range map[string]*DataPoint{"L1": inv.IAC_L1, "L2": inv.IAC_L2, "L3": inv.IAC_L3} {
		if dp != nil {
			c.addGauge(ms, "fronius_inverter_ac_current_phase", "Inverter AC current per phase", "A",
				dp.Value, map[string]string{"phase": phase}, now)
		}
	}
	for phase, dp := range map[string]*DataPoint{"L1": inv.UAC_L1, "L2": inv.UAC_L2, "L3": inv.UAC_L3} {
		if dp != nil {
			c.addGauge(ms, "fronius_inverter_ac_voltage_phase", "Inverter AC voltage per phase", "V",
				dp.Value, map[string]string{"phase": phase}, now)
		}
	}
	if inv.T_AMBIENT != nil {
		c.addGauge(ms, "fronius_inverter_temperature_ambient", "Inverter ambient temperature", "Cel",
			inv.T_AMBIENT.Value, nil, now)
	}

	// DC pro MPPT
	for mppt, dp := range map[string]*DataPoint{"1": inv.UDC, "2": inv.UDC_2, "3": inv.UDC_3} {
		if dp != nil {
			c.addGauge(ms, "fronius_inverter_dc_voltage", "Inverter DC voltage", "V",
				dp.Value, map[string]string{"mppt": mppt}, now)
		}
	}
	for mppt, dp := range map[string]*DataPoint{"1": inv.IDC, "2": inv.IDC_2, "3": inv.IDC_3} {
		if dp != nil {
			c.addGauge(ms, "fronius_inverter_dc_current", "Inverter DC current", "A",
				dp.Value, map[string]string{"mppt": mppt}, now)
		}
	}

	// Energien (alleinige Quelle — nicht via PowerFlow doppelt)
	if inv.TOTAL_ENERGY != nil {
		c.addSum(ms, "fronius_inverter_energy_total", "Inverter total energy", "Wh",
			inv.TOTAL_ENERGY.Value, nil, now, true)
	}
	if inv.YEAR_ENERGY != nil {
		c.addSum(ms, "fronius_inverter_energy_year", "Inverter energy this year", "Wh",
			inv.YEAR_ENERGY.Value, nil, now, true)
	}
	if inv.DAY_ENERGY != nil {
		c.addSum(ms, "fronius_inverter_energy_day", "Inverter energy today", "Wh",
			inv.DAY_ENERGY.Value, nil, now, true)
	}

	// Status (immer emittieren)
	if inv.DeviceStatus != nil {
		c.addGauge(ms, "fronius_inverter_status_code", "Inverter status code", "1",
			float64(inv.DeviceStatus.StatusCode), nil, now)
		c.addGauge(ms, "fronius_inverter_error_code", "Inverter error code", "1",
			float64(inv.DeviceStatus.ErrorCode), nil, now)
	}
}

// inverterResourceAttrs baut die Resource-Attribute für einen Inverter.
func (c *Converter) inverterResourceAttrs(invID string, info InverterInfoData) map[string]string {
	attrs := map[string]string{
		"fronius.component":   "inverter",
		"fronius.inverter.id": invID,
	}
	if info != nil {
		if i, ok := info[invID]; ok {
			attrs["fronius.inverter.serial"] = i.UniqueID
			attrs["fronius.inverter.custom_name"] = i.CustomName
		}
	}
	return attrs
}

// ======================== Meter Conversion ========================

func (c *Converter) convertMeter(metrics pmetric.Metrics, meterData MeterRealtimeData, now pcommon.Timestamp) {
	for deviceID, meter := range meterData {
		attrs := map[string]string{
			"fronius.component": "meter",
			"fronius.meter.id":  deviceID,
		}
		if meter.Details != nil {
			attrs["fronius.meter.serial"] = meter.Details.Serial
			attrs["fronius.meter.model"] = meter.Details.Model
			attrs["fronius.meter.manufacturer"] = meter.Details.Manufacturer
		}
		ms := c.newResource(metrics, attrs)

		// Phase 1
		c.addPhaseGauge(ms, "fronius_meter_current", "Meter current per phase", "A", meter.Current_AC_Phase_1, "L1", now)
		c.addPhaseGauge(ms, "fronius_meter_voltage", "Meter voltage per phase", "V", meter.Voltage_AC_Phase_1, "L1", now)
		c.addPhaseGauge(ms, "fronius_meter_power_real", "Meter real power per phase", "W", meter.PowerReal_P_Phase_1, "L1", now)
		c.addPhaseGauge(ms, "fronius_meter_power_reactive", "Meter reactive power per phase", "var", meter.PowerReactive_Q_Phase_1, "L1", now)
		c.addPhaseGauge(ms, "fronius_meter_power_apparent", "Meter apparent power per phase", "VA", meter.PowerApparent_S_Phase_1, "L1", now)
		c.addPhaseGauge(ms, "fronius_meter_power_factor", "Meter power factor per phase", "1", meter.PowerFactor_Phase_1, "L1", now)

		// Phase 2
		c.addPhaseGauge(ms, "fronius_meter_current", "Meter current per phase", "A", meter.Current_AC_Phase_2, "L2", now)
		c.addPhaseGauge(ms, "fronius_meter_voltage", "Meter voltage per phase", "V", meter.Voltage_AC_Phase_2, "L2", now)
		c.addPhaseGauge(ms, "fronius_meter_power_real", "Meter real power per phase", "W", meter.PowerReal_P_Phase_2, "L2", now)
		c.addPhaseGauge(ms, "fronius_meter_power_reactive", "Meter reactive power per phase", "var", meter.PowerReactive_Q_Phase_2, "L2", now)
		c.addPhaseGauge(ms, "fronius_meter_power_apparent", "Meter apparent power per phase", "VA", meter.PowerApparent_S_Phase_2, "L2", now)
		c.addPhaseGauge(ms, "fronius_meter_power_factor", "Meter power factor per phase", "1", meter.PowerFactor_Phase_2, "L2", now)

		// Phase 3
		c.addPhaseGauge(ms, "fronius_meter_current", "Meter current per phase", "A", meter.Current_AC_Phase_3, "L3", now)
		c.addPhaseGauge(ms, "fronius_meter_voltage", "Meter voltage per phase", "V", meter.Voltage_AC_Phase_3, "L3", now)
		c.addPhaseGauge(ms, "fronius_meter_power_real", "Meter real power per phase", "W", meter.PowerReal_P_Phase_3, "L3", now)
		c.addPhaseGauge(ms, "fronius_meter_power_reactive", "Meter reactive power per phase", "var", meter.PowerReactive_Q_Phase_3, "L3", now)
		c.addPhaseGauge(ms, "fronius_meter_power_apparent", "Meter apparent power per phase", "VA", meter.PowerApparent_S_Phase_3, "L3", now)
		c.addPhaseGauge(ms, "fronius_meter_power_factor", "Meter power factor per phase", "1", meter.PowerFactor_Phase_3, "L3", now)

		// Sum
		if meter.Current_AC_Sum != nil {
			c.addGauge(ms, "fronius_meter_current_sum", "Meter current sum", "A", *meter.Current_AC_Sum, nil, now)
		}
		if meter.PowerReal_P_Sum != nil {
			c.addGauge(ms, "fronius_meter_power_real_sum", "Meter real power sum", "W", *meter.PowerReal_P_Sum, nil, now)
		}
		if meter.PowerReactive_Q_Sum != nil {
			c.addGauge(ms, "fronius_meter_power_reactive_sum", "Meter reactive power sum", "var", *meter.PowerReactive_Q_Sum, nil, now)
		}
		if meter.PowerApparent_S_Sum != nil {
			c.addGauge(ms, "fronius_meter_power_apparent_sum", "Meter apparent power sum", "VA", *meter.PowerApparent_S_Sum, nil, now)
		}
		if meter.PowerFactor_Sum != nil {
			c.addGauge(ms, "fronius_meter_power_factor_sum", "Meter power factor sum", "1", *meter.PowerFactor_Sum, nil, now)
		}

		// Energy (kumulativ, monoton)
		if meter.EnergyReal_WAC_Sum_Consumed != nil {
			c.addSum(ms, "fronius_meter_energy_consumed", "Meter energy consumed", "Wh",
				*meter.EnergyReal_WAC_Sum_Consumed, nil, now, true)
		}
		if meter.EnergyReal_WAC_Sum_Produced != nil {
			c.addSum(ms, "fronius_meter_energy_produced", "Meter energy produced", "Wh",
				*meter.EnergyReal_WAC_Sum_Produced, nil, now, true)
		}

		// Frequency
		if meter.Frequency_Phase_Average != nil {
			c.addGauge(ms, "fronius_meter_frequency", "Meter frequency", "Hz", *meter.Frequency_Phase_Average, nil, now)
		}
	}
}

// ======================== Storage/Battery Conversion ========================

func (c *Converter) convertStorage(metrics pmetric.Metrics, storageData StorageRealtimeData, now pcommon.Timestamp) {
	for deviceID, storage := range storageData {
		ctrl := storage.Controller
		attrs := map[string]string{
			"fronius.component":            "battery",
			"fronius.battery.id":           deviceID,
			"fronius.battery.serial":       ctrl.Details.Serial,
			"fronius.battery.model":        ctrl.Details.Model,
			"fronius.battery.manufacturer": ctrl.Details.Manufacturer,
		}
		ms := c.newResource(metrics, attrs)

		c.addGauge(ms, "fronius_battery_soc", "Battery state of charge", "%", ctrl.StateOfCharge_Relative, nil, now)
		c.addGauge(ms, "fronius_battery_voltage_dc", "Battery DC voltage", "V", ctrl.Voltage_DC, nil, now)
		c.addGauge(ms, "fronius_battery_current_dc", "Battery DC current", "A", ctrl.Current_DC, nil, now)
		c.addGauge(ms, "fronius_battery_temperature", "Battery cell temperature", "Cel", ctrl.Temperature_Cell, nil, now)
		c.addGauge(ms, "fronius_battery_capacity_max", "Battery maximum capacity", "Wh", ctrl.Capacity_Maximum, nil, now)
		c.addGauge(ms, "fronius_battery_status_code", "Battery status code", "1", ctrl.Status_BatteryCell, nil, now)
	}
}

// ======================== Ohmpilot Conversion ========================

func (c *Converter) convertOhmpilot(metrics pmetric.Metrics, data OhmpilotRealtimeData, now pcommon.Timestamp) {
	for deviceID, ohm := range data {
		attrs := map[string]string{
			"fronius.component":             "ohmpilot",
			"fronius.ohmpilot.id":           deviceID,
			"fronius.ohmpilot.serial":       ohm.Details.Serial,
			"fronius.ohmpilot.model":        ohm.Details.Model,
			"fronius.ohmpilot.manufacturer": ohm.Details.Manufacturer,
			"fronius.ohmpilot.hardware":     ohm.Details.Hardware,
			"fronius.ohmpilot.software":     ohm.Details.Software,
		}
		ms := c.newResource(metrics, attrs)

		c.addGauge(ms, "fronius_ohmpilot_power", "Ohmpilot real power", "W", ohm.PowerReal_PAC_Sum, nil, now)
		c.addSum(ms, "fronius_ohmpilot_energy_consumed", "Ohmpilot energy consumed", "Wh",
			ohm.EnergyReal_WAC_Sum_Consumed, nil, now, true)
		c.addGauge(ms, "fronius_ohmpilot_temperature", "Ohmpilot heating element temperature", "Cel",
			ohm.Temperature_Channel_1, nil, now)
		c.addGauge(ms, "fronius_ohmpilot_state_code", "Ohmpilot state code", "1",
			ohm.CodeOfState, nil, now)
	}
}

// ======================== Scrape Telemetry Conversion ========================

func (c *Converter) convertScrapeTelemetry(metrics pmetric.Metrics, stats *ScrapeStats, now pcommon.Timestamp) {
	ms := c.newResource(metrics, map[string]string{
		"fronius.component": "scraper",
	})

	c.addGauge(ms, "fronius_scrape_duration_seconds", "Scrape cycle duration", "s",
		stats.DurationSeconds, nil, now)
	c.addSumInt(ms, "fronius_scrape_errors_total", "Total scrape errors", "1",
		stats.Errors, nil, now, true)
	successVal := 0.0
	if stats.Success {
		successVal = 1.0
	}
	c.addGauge(ms, "fronius_scrape_success", "1 if last scrape had no errors", "1",
		successVal, nil, now)
}

// ======================== Helper Functions ========================

// addPhaseGauge ist eine Convenience-Wrapper für Phase-Metriken (mit Pointer-Wert).
func (c *Converter) addPhaseGauge(
	ms pmetric.MetricSlice,
	name, desc, unit string,
	value *float64,
	phase string,
	now pcommon.Timestamp,
) {
	if value == nil {
		return
	}
	c.addGauge(ms, name, desc, unit, *value, map[string]string{"phase": phase}, now)
}

// addGauge erstellt eine Gauge-Metrik.
func (c *Converter) addGauge(
	ms pmetric.MetricSlice,
	name, desc, unit string,
	value float64,
	labels map[string]string,
	now pcommon.Timestamp,
) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit(unit)

	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(now)
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}

// addSum erstellt eine kumulative Sum-Metrik (double).
func (c *Converter) addSum(
	ms pmetric.MetricSlice,
	name, desc, unit string,
	value float64,
	labels map[string]string,
	now pcommon.Timestamp,
	monotonic bool,
) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit(unit)

	sum := m.SetEmptySum()
	sum.SetIsMonotonic(monotonic)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(now)
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}

// addSumInt erstellt eine kumulative Sum-Metrik (int).
func (c *Converter) addSumInt(
	ms pmetric.MetricSlice,
	name, desc, unit string,
	value int64,
	labels map[string]string,
	now pcommon.Timestamp,
	monotonic bool,
) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDescription(desc)
	m.SetUnit(unit)

	sum := m.SetEmptySum()
	sum.SetIsMonotonic(monotonic)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(value)
	dp.SetTimestamp(now)
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}
