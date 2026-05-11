package froniusreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Endpoint:           "http://192.168.1.10",
				CollectionInterval: 60 * time.Second,
				Timeout:            10 * time.Second,
				Metrics: MetricsConfig{
					PowerFlow: true,
				},
			},
			wantErr: false,
		},
		{
			name: "empty endpoint",
			config: &Config{
				Endpoint:           "",
				CollectionInterval: 60 * time.Second,
				Timeout:            10 * time.Second,
				Metrics: MetricsConfig{
					PowerFlow: true,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid collection interval",
			config: &Config{
				Endpoint:           "http://192.168.1.10",
				CollectionInterval: -1 * time.Second,
				Timeout:            10 * time.Second,
				Metrics: MetricsConfig{
					PowerFlow: true,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			config: &Config{
				Endpoint:           "http://192.168.1.10",
				CollectionInterval: 60 * time.Second,
				Timeout:            0 * time.Second,
				Metrics: MetricsConfig{
					PowerFlow: true,
				},
			},
			wantErr: true,
		},
		{
			name: "no metrics enabled",
			config: &Config{
				Endpoint:           "http://192.168.1.10",
				CollectionInterval: 60 * time.Second,
				Timeout:            10 * time.Second,
				Metrics:            MetricsConfig{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
