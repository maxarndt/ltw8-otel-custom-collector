package knxreceiver

import (
	"context"
	"sync"
	"time"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type knxReceiver struct {
	cfg          *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	cancel       context.CancelFunc
	done         chan struct{}
	startTime    pcommon.Timestamp
	receiverID   string
}

func newKNXReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) *knxReceiver {
	return &knxReceiver{
		cfg:          cfg,
		logger:       set.Logger,
		nextConsumer: next,
		done:         make(chan struct{}),
		receiverID:   set.ID.String(),
	}
}

// Start launches the KNX receiver goroutine and returns immediately.
func (r *knxReceiver) Start(_ context.Context, _ component.Host) error {
	r.startTime = pcommon.NewTimestampFromTime(time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.run(ctx)
	return nil
}

// Shutdown signals the receiver to stop and waits for it to finish.
func (r *knxReceiver) Shutdown(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the main loop: connect → readStartup (async) → listen → reconnect on loss.
func (r *knxReceiver) run(ctx context.Context) {
	defer close(r.done)

	const (
		maxBackoff      = 60 * time.Second
		stableThreshold = 30 * time.Second
	)
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		client, err := NewKNXClient(r.cfg.Connection)
		if err != nil {
			r.logger.Error("KNX connect failed, retrying",
				zap.Error(err),
				zap.Duration("backoff", backoff))
			if !r.sleep(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		r.logger.Info("KNX connected", zap.String("type", string(r.cfg.Connection.Type)))
		connectedAt := time.Now()

		// readStartup runs concurrently with listen so the Inbound channel is drained
		// immediately. Otherwise responses to the GroupValueRead requests could be
		// dropped by the knx-go library while listen() has not started yet.
		connCtx, connCancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.readStartup(connCtx, client)
		}()

		// listen blocks until the inbound channel is closed or ctx is cancelled.
		r.listen(ctx, client)
		connCancel()
		// Wait for readStartup to finish before closing the client; otherwise a
		// concurrent Send on a closed connection could panic.
		wg.Wait()
		client.Close()

		select {
		case <-ctx.Done():
			return
		default:
		}

		// Only reset backoff if the connection was stable for a while; otherwise a
		// flapping connection would spin reconnects at the lowest backoff value.
		if time.Since(connectedAt) > stableThreshold {
			backoff = time.Second
		}

		r.logger.Warn("KNX connection lost, reconnecting...",
			zap.Duration("backoff", backoff))
		if !r.sleep(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

// nextBackoff doubles the backoff, clamped to max.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// readStartup sends a GroupValueRead for each address with read_startup: true.
func (r *knxReceiver) readStartup(ctx context.Context, client KNXClient) {
	for addr, ac := range r.cfg.AddressConfigs {
		if !ac.ReadStartup {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}

		ga, err := cemi.NewGroupAddrString(addr)
		if err != nil {
			r.logger.Error("invalid group address in config",
				zap.String("address", addr), zap.Error(err))
			continue
		}

		if err := client.Send(knx.GroupEvent{
			Command:     knx.GroupRead,
			Destination: ga,
		}); err != nil {
			r.logger.Error("ReadStartup send failed",
				zap.String("address", addr), zap.Error(err))
		} else {
			r.logger.Debug("ReadStartup sent", zap.String("address", addr))
		}

		r.sleep(ctx, r.cfg.ReadStartupInterval)
	}
}

// listen processes incoming KNX group events until the channel is closed or ctx is done.
func (r *knxReceiver) listen(ctx context.Context, client KNXClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-client.Inbound():
			if !ok {
				return // connection lost
			}
			r.handleEvent(ctx, event)
		}
	}
}

// handleEvent decodes a KNX group event and pushes it as an OTEL metric.
func (r *knxReceiver) handleEvent(ctx context.Context, event knx.GroupEvent) {
	// Only process Write and Response telegrams; skip Read requests.
	if event.Command == knx.GroupRead {
		return
	}

	addr := event.Destination.String()
	ac, ok := r.cfg.AddressConfigs[addr]
	if !ok {
		r.logger.Debug("unknown group address, skipping", zap.String("address", addr))
		return
	}
	if !ac.Export {
		return
	}

	value, err := DecodeDPT(ac.DPT, event.Data)
	if err != nil {
		r.logger.Error("DPT decode failed",
			zap.String("address", addr),
			zap.String("dpt", ac.DPT),
			zap.Error(err))
		return
	}

	metrics := ConvertToMetrics(addr, ac, value, event.Source.String(), r.startTime, r.receiverID)
	if err := r.nextConsumer.ConsumeMetrics(ctx, metrics); err != nil {
		r.logger.Error("ConsumeMetrics failed", zap.Error(err))
	}
}

// sleep waits for d or ctx cancellation. Returns false if ctx was cancelled.
func (r *knxReceiver) sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
