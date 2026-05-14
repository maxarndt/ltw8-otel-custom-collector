package knxreceiver

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/maxarndt/ltw8-otel-custom-collector/receiver/knxreceiver"

// telemetry holds the receiver's self-telemetry instruments. They are created
// from the collector's MeterProvider so they automatically flow into the service
// telemetry pipeline (e.g. the internal Prometheus endpoint on :8888).
type telemetry struct {
	telegrams     metric.Int64Counter
	decodeErrors  metric.Int64Counter
	consumeErrors metric.Int64Counter
	reconnects    metric.Int64Counter
	// connectionUp is +1 on connect and -1 on disconnect — it stays at 0 or 1.
	connectionUp metric.Int64UpDownCounter
}

func newTelemetry(mp metric.MeterProvider) (*telemetry, error) {
	m := mp.Meter(meterName)
	var t telemetry
	var err error

	if t.telegrams, err = m.Int64Counter(
		"knxreceiver.telegrams_received",
		metric.WithDescription("KNX group telegrams successfully decoded and forwarded"),
		metric.WithUnit("{telegram}"),
	); err != nil {
		return nil, fmt.Errorf("telegrams_received: %w", err)
	}

	if t.decodeErrors, err = m.Int64Counter(
		"knxreceiver.decode_errors",
		metric.WithDescription("Failures while decoding a KNX telegram via the configured DPT"),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, fmt.Errorf("decode_errors: %w", err)
	}

	if t.consumeErrors, err = m.Int64Counter(
		"knxreceiver.consume_errors",
		metric.WithDescription("Failures while forwarding metrics to the next consumer"),
		metric.WithUnit("{error}"),
	); err != nil {
		return nil, fmt.Errorf("consume_errors: %w", err)
	}

	if t.reconnects, err = m.Int64Counter(
		"knxreceiver.reconnects",
		metric.WithDescription("KNX connection (re)connection attempts after loss or initial failure"),
		metric.WithUnit("{reconnect}"),
	); err != nil {
		return nil, fmt.Errorf("reconnects: %w", err)
	}

	if t.connectionUp, err = m.Int64UpDownCounter(
		"knxreceiver.connection_up",
		metric.WithDescription("Current KNX connection state (0 = down, 1 = up)"),
	); err != nil {
		return nil, fmt.Errorf("connection_up: %w", err)
	}

	return &t, nil
}
