package knxreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr                    = "knx"
	defaultMulticastAddress    = "224.0.23.12:3671"
	defaultReadStartupInterval = 200 * time.Millisecond
)

// NewFactory creates the knxreceiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Connection: ConnectionConfig{
			Type:             ConnectionTypeRouter,
			MulticastAddress: defaultMulticastAddress,
		},
		ReadStartupInterval: defaultReadStartupInterval,
		AddressConfigs:      make(map[string]*AddressConfig),
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
	return newKNXReceiver(set, c, next), nil
}
