package syrreceiver

import (
	"fmt"
	"time"
)

// MetricConfig aktiviert oder deaktiviert eine einzelne SYR-Metrik.
type MetricConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// MetricsConfig enthält alle bekannten Metriken der SYR NeoSoft 2500.
// Jede Metrik kann unabhängig aktiviert oder deaktiviert werden.
type MetricsConfig struct {
	// FLO: Aktueller Durchfluss (l/h)
	FLO MetricConfig `mapstructure:"FLO"`
	// VOL: Gesamtvolumen kumuliert (l) — monoton steigend
	VOL MetricConfig `mapstructure:"VOL"`
	// RE1: Restkapazität bis zur nächsten Regenerierung (l)
	RE1 MetricConfig `mapstructure:"RE1"`
	// IWH: Eingangswasserhärte (°dH)
	IWH MetricConfig `mapstructure:"IWH"`
	// OWH: Ausgangswasserhärte nach Enthärtung (°dH)
	OWH MetricConfig `mapstructure:"OWH"`
	// SS1: Geschätzter Salzvorrat in Wochen
	SS1 MetricConfig `mapstructure:"SS1"`
	// SV1: Salzvorrat in kg
	SV1 MetricConfig `mapstructure:"SV1"`
	// ALA: Alarmstatus (0 = kein Alarm)
	ALA MetricConfig `mapstructure:"ALA"`
	// RG1: Regenerierungsstatus
	RG1 MetricConfig `mapstructure:"RG1"`
}

// Config ist die Konfiguration des syrreceivers.
type Config struct {
	// Endpoint ist die Basis-URL des SYR-Geräts, z.B. "http://192.168.1.100:5333".
	Endpoint string `mapstructure:"endpoint"`
	// CollectionInterval legt fest, wie oft Metriken abgefragt werden (Default: 2m).
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// Timeout ist das HTTP-Timeout pro Anfrage (Default: 10s).
	Timeout time.Duration `mapstructure:"timeout"`
	// Metrics konfiguriert welche Metriken abgefragt werden.
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// Validate prüft die Konfiguration auf Vollständigkeit.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("endpoint is required (e.g. \"http://192.168.1.100:5333\")")
	}
	if c.CollectionInterval <= 0 {
		return fmt.Errorf("collection_interval must be positive")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}
