package syrreceiver

import (
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	valid := func() *Config {
		return &Config{
			Endpoint:           "http://192.168.1.100:5333",
			CollectionInterval: 2 * time.Minute,
			Timeout:            10 * time.Second,
		}
	}

	t.Run("valid config", func(t *testing.T) {
		if err := valid().Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing endpoint", func(t *testing.T) {
		c := valid()
		c.Endpoint = ""
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("zero collection_interval", func(t *testing.T) {
		c := valid()
		c.CollectionInterval = 0
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		c := valid()
		c.Timeout = 0
		if err := c.Validate(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
