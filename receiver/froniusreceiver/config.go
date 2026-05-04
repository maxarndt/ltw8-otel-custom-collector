package froniusreceiver

import (
	"fmt"
	"time"
)

// MetricsConfig enthält Flags für die verschiedenen Fronius API-Endpoints.
type MetricsConfig struct {
	// PowerFlow: Site-Level Daten (P_PV, P_Grid, P_Load, P_Akku, Energien)
	PowerFlow bool `mapstructure:"power_flow"`
	// InverterRealtime: Inverter-spezifische RT-Daten (AC/DC Spannungen/Ströme, Leistung, Energie)
	InverterRealtime bool `mapstructure:"inverter_realtime"`
	// MeterRealtime: Smart Meter Daten (Phasen-Ströme, -Spannungen, Leistungen, Energien)
	MeterRealtime bool `mapstructure:"meter_realtime"`
	// StorageRealtime: Batterie/Storage-Daten (SOC, Spannung, Strom, Temperatur)
	StorageRealtime bool `mapstructure:"storage_realtime"`
	// OhmpilotRealtime: Ohmpilot-Daten (Heizstab-Energie, Leistung, Temperatur, State)
	OhmpilotRealtime bool `mapstructure:"ohmpilot_realtime"`
	// InverterInfo: Inverter-Metadaten (Serial, Modell, CustomName, etc.)
	InverterInfo bool `mapstructure:"inverter_info"`
}

// Config ist die Konfiguration des froniusreceivers.
type Config struct {
	// Endpoint ist die Basis-URL des Fronius Inverters, z.B. "http://192.168.1.10".
	Endpoint string `mapstructure:"endpoint"`
	// CollectionInterval legt fest, wie oft Metriken abgefragt werden (Default: 60s).
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// Timeout ist das HTTP-Timeout pro Anfrage (Default: 10s).
	Timeout time.Duration `mapstructure:"timeout"`
	// Metrics konfiguriert welche Fronius API-Endpoints abgefragt werden.
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// Validate prüft die Konfiguration auf Vollständigkeit.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint must not be empty")
	}
	if c.CollectionInterval <= 0 {
		return fmt.Errorf("collection_interval must be > 0, got %v", c.CollectionInterval)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be > 0, got %v", c.Timeout)
	}
	// Mindestens ein Endpoint sollte enabled sein (optional - Warning nur, kein Error)
	if !c.Metrics.PowerFlow && !c.Metrics.InverterRealtime && !c.Metrics.MeterRealtime &&
		!c.Metrics.StorageRealtime && !c.Metrics.OhmpilotRealtime && !c.Metrics.InverterInfo {
		return fmt.Errorf("at least one metrics endpoint must be enabled")
	}
	return nil
}
