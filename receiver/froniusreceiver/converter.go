package froniusreceiver

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Converter konvertiert Fronius API Daten zu OTEL pmetric.Metrics.
type Converter struct {
	logger *zap.Logger
}

// NewConverter erstellt einen neuen Converter.
func NewConverter(logger *zap.Logger) *Converter {
	return &Converter{logger: logger}
}

// ConvertToMetrics konvertiert ScrapedMetrics zu pmetric.Metrics.
func (c *Converter) ConvertToMetrics(ctx context.Context, scraped *ScrapedMetrics) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()

	now := pcommon.NewTimestampFromTime(scraped.Timestamp)

	// Conversion für jeden Datentyp
	if scraped.PowerFlow != nil {
		c.convertPowerFlow(scopeMetrics.Metrics(), scraped.PowerFlow, now)
	}
	if scraped.Inverter != nil {
		c.convertInverter(scopeMetrics.Metrics(), scraped.Inverter, now)
	}
	if scraped.Meter != nil && len(scraped.Meter) > 0 {
		c.convertMeter(scopeMetrics.Metrics(), scraped.Meter, now)
	}
	if scraped.Storage != nil && len(scraped.Storage) > 0 {
		c.convertStorage(scopeMetrics.Metrics(), scraped.Storage, now)
	}

	return metrics
}

// ======================== PowerFlow Conversion ========================

func (c *Converter) convertPowerFlow(metrics pmetric.MetricSlice, pf *PowerFlowRealtimeData, now pcommon.Timestamp) {
	if pf == nil {
		return
	}

	site := pf.Site

	// fronius_site_pv_power_watts (Gauge)
	c.addGaugeMetric(metrics, "fronius_site_pv_power_watts", "PV power in watts",
		site.P_PV, nil, now)

	// fronius_site_grid_power_watts (Gauge) — negative = import
	c.addGaugeMetric(metrics, "fronius_site_grid_power_watts", "Grid power in watts (negative=import)",
		site.P_Grid, nil, now)

	// fronius_site_load_power_watts (Gauge)
	c.addGaugeMetric(metrics, "fronius_site_load_power_watts", "Load power in watts",
		site.P_Load, nil, now)

	// fronius_site_battery_power_watts (Gauge)
	c.addGaugeMetric(metrics, "fronius_site_battery_power_watts", "Battery power in watts (negative=charging)",
		site.P_Akku, nil, now)

	// fronius_site_energy_total_wh (Sum, monotonic)
	c.addSumMetric(metrics, "fronius_site_energy_total_wh", "Total energy produced in watt-hours",
		site.E_Total, nil, now, true)

	// fronius_site_energy_year_wh (Sum, daily reset)
	c.addSumMetric(metrics, "fronius_site_energy_year_wh", "Energy produced this year in watt-hours",
		site.E_Year, nil, now, true)

	// fronius_site_energy_day_wh (Sum, daily reset)
	c.addSumMetric(metrics, "fronius_site_energy_day_wh", "Energy produced today in watt-hours",
		site.E_Day, nil, now, true)

	// fronius_site_autonomy_ratio (Gauge)
	c.addGaugeMetric(metrics, "fronius_site_autonomy_ratio", "Autonomy ratio (0-100)",
		site.Rel_Autonomy, nil, now)

	// fronius_site_selfconsumption_ratio (Gauge)
	c.addGaugeMetric(metrics, "fronius_site_selfconsumption_ratio", "Self-consumption ratio (0-100)",
		site.Rel_SelfConsumption, nil, now)

	// Inverter-level metrics aus PowerFlow (pro Inverter)
	for invID, inv := range pf.Inverters {
		labels := map[string]string{"inverter_id": invID}

		// fronius_inverter_ac_power_watts (Gauge)
		c.addGaugeMetric(metrics, "fronius_inverter_ac_power_watts", "Inverter AC power in watts",
			inv.P, labels, now)

		// fronius_inverter_soc_percent (Gauge) — aus PowerFlow
		if inv.SOC > 0 {
			c.addGaugeMetric(metrics, "fronius_inverter_soc_percent", "State of charge in percent",
				inv.SOC, labels, now)
		}

		// fronius_inverter_energy_total_wh (Sum, monotonic)
		c.addSumMetric(metrics, "fronius_inverter_energy_total_wh", "Inverter total energy in watt-hours",
			inv.E_Total, labels, now, true)

		// fronius_inverter_energy_year_wh (Sum)
		c.addSumMetric(metrics, "fronius_inverter_energy_year_wh", "Inverter energy this year in watt-hours",
			inv.E_Year, labels, now, true)

		// fronius_inverter_energy_day_wh (Sum)
		c.addSumMetric(metrics, "fronius_inverter_energy_day_wh", "Inverter energy today in watt-hours",
			inv.E_Day, labels, now, true)
	}
}

// ======================== Inverter Realtime Conversion ========================

func (c *Converter) convertInverter(metrics pmetric.MetricSlice, inv *InverterRealtimeData, now pcommon.Timestamp) {
	if inv == nil {
		return
	}

	labels := map[string]string{"inverter_id": "1"} // Default Device ID 1

	// AC Power
	if inv.PAC != nil {
		c.addGaugeMetric(metrics, "fronius_inverter_ac_power_watts", "Inverter AC power in watts",
			inv.PAC.Value, labels, now)
	}

	// AC Voltage
	if inv.UAC != nil {
		c.addGaugeMetric(metrics, "fronius_inverter_ac_voltage_volts", "Inverter AC voltage in volts",
			inv.UAC.Value, labels, now)
	}

	// AC Current
	if inv.IAC != nil {
		c.addGaugeMetric(metrics, "fronius_inverter_ac_current_amps", "Inverter AC current in amps",
			inv.IAC.Value, labels, now)
	}

	// AC Frequency
	if inv.FAC != nil {
		c.addGaugeMetric(metrics, "fronius_inverter_ac_frequency_hz", "Inverter AC frequency in Hz",
			inv.FAC.Value, labels, now)
	}

	// DC Voltages (pro MPPT)
	if inv.UDC != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "1"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_voltage_volts", "Inverter DC voltage MPPT 1 in volts",
			inv.UDC.Value, mpptLabels, now)
	}
	if inv.UDC_2 != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "2"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_voltage_volts", "Inverter DC voltage MPPT 2 in volts",
			inv.UDC_2.Value, mpptLabels, now)
	}
	if inv.UDC_3 != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "3"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_voltage_volts", "Inverter DC voltage MPPT 3 in volts",
			inv.UDC_3.Value, mpptLabels, now)
	}

	// DC Currents (pro MPPT)
	if inv.IDC != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "1"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_current_amps", "Inverter DC current MPPT 1 in amps",
			inv.IDC.Value, mpptLabels, now)
	}
	if inv.IDC_2 != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "2"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_current_amps", "Inverter DC current MPPT 2 in amps",
			inv.IDC_2.Value, mpptLabels, now)
	}
	if inv.IDC_3 != nil {
		mpptLabels := map[string]string{"inverter_id": "1", "mppt": "3"}
		c.addGaugeMetric(metrics, "fronius_inverter_dc_current_amps", "Inverter DC current MPPT 3 in amps",
			inv.IDC_3.Value, mpptLabels, now)
	}

	// Energies
	if inv.TOTAL_ENERGY != nil {
		c.addSumMetric(metrics, "fronius_inverter_energy_total_wh", "Inverter total energy in watt-hours",
			inv.TOTAL_ENERGY.Value, labels, now, true)
	}
	if inv.YEAR_ENERGY != nil {
		c.addSumMetric(metrics, "fronius_inverter_energy_year_wh", "Inverter energy this year in watt-hours",
			inv.YEAR_ENERGY.Value, labels, now, true)
	}
	if inv.DAY_ENERGY != nil {
		c.addSumMetric(metrics, "fronius_inverter_energy_day_wh", "Inverter energy today in watt-hours",
			inv.DAY_ENERGY.Value, labels, now, true)
	}

	// Status Code
	if inv.DeviceStatus != nil {
		c.addGaugeMetric(metrics, "fronius_inverter_status_code", "Inverter status code",
			float64(inv.DeviceStatus.StatusCode), labels, now)
		if inv.DeviceStatus.ErrorCode > 0 {
			c.addGaugeMetric(metrics, "fronius_inverter_error_code", "Inverter error code",
				float64(inv.DeviceStatus.ErrorCode), labels, now)
		}
	}
}

// ======================== Meter Conversion ========================

func (c *Converter) convertMeter(metrics pmetric.MetricSlice, meterData MeterRealtimeData, now pcommon.Timestamp) {
	for deviceID, meter := range meterData {
		// Phase 1
		if meter.Current_AC_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_current_amps", "Meter current phase 1 in amps",
				*meter.Current_AC_Phase_1, labels, now)
		}
		if meter.Voltage_AC_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_voltage_volts", "Meter voltage phase 1 in volts",
				*meter.Voltage_AC_Phase_1, labels, now)
		}
		if meter.PowerReal_P_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_power_real_watts", "Meter real power phase 1 in watts",
				*meter.PowerReal_P_Phase_1, labels, now)
		}
		if meter.PowerReactive_Q_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_power_reactive_var", "Meter reactive power phase 1 in VAR",
				*meter.PowerReactive_Q_Phase_1, labels, now)
		}
		if meter.PowerApparent_S_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_power_apparent_va", "Meter apparent power phase 1 in VA",
				*meter.PowerApparent_S_Phase_1, labels, now)
		}
		if meter.PowerFactor_Phase_1 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L1"}
			c.addGaugeMetric(metrics, "fronius_meter_power_factor", "Meter power factor phase 1",
				*meter.PowerFactor_Phase_1, labels, now)
		}

		// Phase 2
		if meter.Current_AC_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_current_amps", "Meter current phase 2 in amps",
				*meter.Current_AC_Phase_2, labels, now)
		}
		if meter.Voltage_AC_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_voltage_volts", "Meter voltage phase 2 in volts",
				*meter.Voltage_AC_Phase_2, labels, now)
		}
		if meter.PowerReal_P_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_power_real_watts", "Meter real power phase 2 in watts",
				*meter.PowerReal_P_Phase_2, labels, now)
		}
		if meter.PowerReactive_Q_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_power_reactive_var", "Meter reactive power phase 2 in VAR",
				*meter.PowerReactive_Q_Phase_2, labels, now)
		}
		if meter.PowerApparent_S_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_power_apparent_va", "Meter apparent power phase 2 in VA",
				*meter.PowerApparent_S_Phase_2, labels, now)
		}
		if meter.PowerFactor_Phase_2 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L2"}
			c.addGaugeMetric(metrics, "fronius_meter_power_factor", "Meter power factor phase 2",
				*meter.PowerFactor_Phase_2, labels, now)
		}

		// Phase 3
		if meter.Current_AC_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_current_amps", "Meter current phase 3 in amps",
				*meter.Current_AC_Phase_3, labels, now)
		}
		if meter.Voltage_AC_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_voltage_volts", "Meter voltage phase 3 in volts",
				*meter.Voltage_AC_Phase_3, labels, now)
		}
		if meter.PowerReal_P_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_power_real_watts", "Meter real power phase 3 in watts",
				*meter.PowerReal_P_Phase_3, labels, now)
		}
		if meter.PowerReactive_Q_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_power_reactive_var", "Meter reactive power phase 3 in VAR",
				*meter.PowerReactive_Q_Phase_3, labels, now)
		}
		if meter.PowerApparent_S_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_power_apparent_va", "Meter apparent power phase 3 in VA",
				*meter.PowerApparent_S_Phase_3, labels, now)
		}
		if meter.PowerFactor_Phase_3 != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "L3"}
			c.addGaugeMetric(metrics, "fronius_meter_power_factor", "Meter power factor phase 3",
				*meter.PowerFactor_Phase_3, labels, now)
		}

		// Sum
		if meter.Current_AC_Sum != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "Sum"}
			c.addGaugeMetric(metrics, "fronius_meter_current_amps", "Meter current sum in amps",
				*meter.Current_AC_Sum, labels, now)
		}
		if meter.PowerReal_P_Sum != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "Sum"}
			c.addGaugeMetric(metrics, "fronius_meter_power_real_watts", "Meter real power sum in watts",
				*meter.PowerReal_P_Sum, labels, now)
		}
		if meter.PowerReactive_Q_Sum != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "Sum"}
			c.addGaugeMetric(metrics, "fronius_meter_power_reactive_var", "Meter reactive power sum in VAR",
				*meter.PowerReactive_Q_Sum, labels, now)
		}
		if meter.PowerApparent_S_Sum != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "Sum"}
			c.addGaugeMetric(metrics, "fronius_meter_power_apparent_va", "Meter apparent power sum in VA",
				*meter.PowerApparent_S_Sum, labels, now)
		}
		if meter.PowerFactor_Sum != nil {
			labels := map[string]string{"device_id": deviceID, "phase": "Sum"}
			c.addGaugeMetric(metrics, "fronius_meter_power_factor", "Meter power factor sum",
				*meter.PowerFactor_Sum, labels, now)
		}

		// Energy
		if meter.EnergyReal_WAC_Sum_Consumed != nil {
			labels := map[string]string{"device_id": deviceID}
			c.addSumMetric(metrics, "fronius_meter_energy_consumed_wh", "Meter energy consumed in watt-hours",
				*meter.EnergyReal_WAC_Sum_Consumed, labels, now, true)
		}
		if meter.EnergyReal_WAC_Sum_Produced != nil {
			labels := map[string]string{"device_id": deviceID}
			c.addSumMetric(metrics, "fronius_meter_energy_produced_wh", "Meter energy produced in watt-hours",
				*meter.EnergyReal_WAC_Sum_Produced, labels, now, true)
		}

		// Frequency
		if meter.Frequency_Phase_Average != nil {
			labels := map[string]string{"device_id": deviceID}
			c.addGaugeMetric(metrics, "fronius_meter_frequency_hz", "Meter frequency in Hz",
				*meter.Frequency_Phase_Average, labels, now)
		}
	}
}

// ======================== Storage/Battery Conversion ========================

func (c *Converter) convertStorage(metrics pmetric.MetricSlice, storageData StorageRealtimeData, now pcommon.Timestamp) {
	for deviceID, storage := range storageData {
		ctrl := storage.Controller
		labels := map[string]string{"device_id": deviceID}

		// SOC
		c.addGaugeMetric(metrics, "fronius_battery_soc_percent", "Battery state of charge in percent",
			ctrl.StateOfCharge_Relative, labels, now)

		// Voltage
		c.addGaugeMetric(metrics, "fronius_battery_voltage_dc_volts", "Battery DC voltage in volts",
			ctrl.Voltage_DC, labels, now)

		// Current
		c.addGaugeMetric(metrics, "fronius_battery_current_dc_amps", "Battery DC current in amps",
			ctrl.Current_DC, labels, now)

		// Temperature
		c.addGaugeMetric(metrics, "fronius_battery_temperature_celsius", "Battery cell temperature in celsius",
			ctrl.Temperature_Cell, labels, now)

		// Capacity
		c.addGaugeMetric(metrics, "fronius_battery_capacity_max_wh", "Battery maximum capacity in watt-hours",
			ctrl.Capacity_Maximum, labels, now)

		// Status Code
		c.addGaugeMetric(metrics, "fronius_battery_status_code", "Battery status code",
			ctrl.Status_BatteryCell, labels, now)
	}
}

// ======================== Helper Functions ========================

// addGaugeMetric erstellt oder aktualisiert eine Gauge-Metrik.
func (c *Converter) addGaugeMetric(
	metrics pmetric.MetricSlice,
	name string,
	description string,
	value float64,
	labels map[string]string,
	now pcommon.Timestamp,
) {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit("")

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(now)

	if labels != nil {
		for k, v := range labels {
			dp.Attributes().PutStr(k, v)
		}
	}

	metric.CopyTo(metrics.AppendEmpty())
}

// addSumMetric erstellt oder aktualisiert eine Sum-Metrik (kumulativ).
func (c *Converter) addSumMetric(
	metrics pmetric.MetricSlice,
	name string,
	description string,
	value float64,
	labels map[string]string,
	now pcommon.Timestamp,
	monotonic bool,
) {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	metric.SetDescription(description)
	metric.SetUnit("")

	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(monotonic)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(now)

	if labels != nil {
		for k, v := range labels {
			dp.Attributes().PutStr(k, v)
		}
	}

	metric.CopyTo(metrics.AppendEmpty())
}
