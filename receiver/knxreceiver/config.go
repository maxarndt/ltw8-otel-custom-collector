package knxreceiver

import (
	"fmt"
	"time"
)

// ConnectionType discriminates Tunnel vs Router mode.
type ConnectionType string

const (
	ConnectionTypeTunnel ConnectionType = "tunnel"
	ConnectionTypeRouter ConnectionType = "router"
)

// ConnectionConfig holds KNX/IP connection parameters.
type ConnectionConfig struct {
	// Type selects the connection mode: "tunnel" (unicast) or "router" (multicast).
	Type ConnectionType `mapstructure:"type"`
	// Endpoint is the gateway address for tunnel mode, e.g. "192.168.1.10:3671".
	Endpoint string `mapstructure:"endpoint"`
	// MulticastAddress is the multicast group for router mode, e.g. "224.0.23.12:3671".
	MulticastAddress string `mapstructure:"multicast_address"`
}

// MetricType distinguishes OTEL metric kinds.
type MetricType string

const (
	MetricTypeGauge MetricType = "gauge"
	MetricTypeSum   MetricType = "sum"
)

// AddressConfig configures one KNX group address.
type AddressConfig struct {
	// Name is the OTEL metric name, e.g. "strom_eg_kueche_wh".
	Name string `mapstructure:"name"`
	// DPT is the KNX Data Point Type, e.g. "13.010".
	DPT string `mapstructure:"dpt"`
	// Export controls whether this address produces metrics.
	Export bool `mapstructure:"export"`
	// MetricType selects "gauge" or "sum".
	MetricType MetricType `mapstructure:"metric_type"`
	// ReadStartup sends a GroupValueRead at startup to get the initial value.
	ReadStartup bool `mapstructure:"read_startup"`
	// Unit overrides the OTEL metric unit. If empty, the unit is derived from
	// the DPT (e.g. "Wh" for 13.010, "mA" for 7.012). Set it when the device
	// transmits scaled or non-default values for the DPT.
	Unit string `mapstructure:"unit"`
	// Labels are static key/value pairs attached to each data point.
	Labels map[string]string `mapstructure:"labels"`
}

// Config is the knxreceiver configuration.
type Config struct {
	// Connection configures the KNX/IP connection.
	Connection ConnectionConfig `mapstructure:"connection"`
	// ReadStartupInterval is the delay between GroupValueRead requests at startup.
	ReadStartupInterval time.Duration `mapstructure:"read_startup_interval"`
	// AddressConfigs maps group addresses (e.g. "1/1/1") to their configuration.
	AddressConfigs map[string]*AddressConfig `mapstructure:"address_configs"`
}

// Validate checks that the config is self-consistent.
func (c *Config) Validate() error {
	switch c.Connection.Type {
	case ConnectionTypeTunnel:
		if c.Connection.Endpoint == "" {
			return fmt.Errorf("tunnel mode requires 'endpoint'")
		}
	case ConnectionTypeRouter:
		if c.Connection.MulticastAddress == "" {
			return fmt.Errorf("router mode requires 'multicast_address'")
		}
	default:
		return fmt.Errorf("unknown connection type: %q (must be \"tunnel\" or \"router\")", c.Connection.Type)
	}

	if len(c.AddressConfigs) == 0 {
		return fmt.Errorf("at least one address_config is required")
	}

	for addr, ac := range c.AddressConfigs {
		if ac.Name == "" {
			return fmt.Errorf("address %q: name is required", addr)
		}
		if ac.DPT == "" {
			return fmt.Errorf("address %q: dpt is required", addr)
		}
		if ac.MetricType != MetricTypeGauge && ac.MetricType != MetricTypeSum {
			return fmt.Errorf("address %q: metric_type must be \"gauge\" or \"sum\", got %q", addr, ac.MetricType)
		}
	}

	return nil
}
