package knxreceiver

import (
	"testing"
)

func TestConfigValidate(t *testing.T) {
	validTunnel := func() *Config {
		return &Config{
			Connection: ConnectionConfig{
				Type:     ConnectionTypeTunnel,
				Endpoint: "192.168.1.10:3671",
			},
			AddressConfigs: map[string]*AddressConfig{
				"1/1/1": {Name: "foo", DPT: "9.001", Export: true, MetricType: MetricTypeGauge},
			},
		}
	}

	validRouter := func() *Config {
		return &Config{
			Connection: ConnectionConfig{
				Type:             ConnectionTypeRouter,
				MulticastAddress: "224.0.23.12:3671",
			},
			AddressConfigs: map[string]*AddressConfig{
				"1/1/1": {Name: "foo", DPT: "9.001", Export: true, MetricType: MetricTypeGauge},
			},
		}
	}

	t.Run("valid tunnel config", func(t *testing.T) {
		if err := validTunnel().Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid router config", func(t *testing.T) {
		if err := validRouter().Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("tunnel missing endpoint", func(t *testing.T) {
		c := validTunnel()
		c.Connection.Endpoint = ""
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("router missing multicast address", func(t *testing.T) {
		c := validRouter()
		c.Connection.MulticastAddress = ""
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unknown connection type", func(t *testing.T) {
		c := validTunnel()
		c.Connection.Type = "unicast"
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("no address configs", func(t *testing.T) {
		c := validTunnel()
		c.AddressConfigs = map[string]*AddressConfig{}
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("address missing name", func(t *testing.T) {
		c := validTunnel()
		c.AddressConfigs["1/1/1"].Name = ""
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("address missing dpt", func(t *testing.T) {
		c := validTunnel()
		c.AddressConfigs["1/1/1"].DPT = ""
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid metric type", func(t *testing.T) {
		c := validTunnel()
		c.AddressConfigs["1/1/1"].MetricType = "counter"
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
