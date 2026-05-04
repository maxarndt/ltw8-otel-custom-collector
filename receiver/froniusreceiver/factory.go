package froniusreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr                   = "fronius"
	defaultCollectionInterval = 60 * time.Second
	defaultTimeout            = 10 * time.Second
)

// NewFactory erstellt die froniusreceiver Factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:           "http://localhost",
		CollectionInterval: defaultCollectionInterval,
		Timeout:            defaultTimeout,
		Metrics: MetricsConfig{
			PowerFlow:        true,
			InverterRealtime: true,
			MeterRealtime:    true,
			StorageRealtime:  true,
			InverterInfo:     true,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	return newFroniusReceiver(set, c, next), nil
}
