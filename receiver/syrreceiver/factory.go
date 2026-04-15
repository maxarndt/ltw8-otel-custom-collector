package syrreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr                    = "syr"
	defaultCollectionInterval  = 2 * time.Minute
	defaultTimeout             = 10 * time.Second
)

// NewFactory erstellt die syrreceiver Factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	allEnabled := MetricConfig{Enabled: true}
	return &Config{
		CollectionInterval: defaultCollectionInterval,
		Timeout:            defaultTimeout,
		Metrics: MetricsConfig{
			FLO: allEnabled,
			VOL: allEnabled,
			RE1: allEnabled,
			IWH: allEnabled,
			OWH: allEnabled,
			SS1: allEnabled,
			SV1: allEnabled,
			ALA: allEnabled,
			RG1: allEnabled,
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
	return newSyrReceiver(set, c, next), nil
}
