package syrreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type syrReceiver struct {
	cfg          *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	scraper      *syrScraper
	cancel       context.CancelFunc
	done         chan struct{}
}

func newSyrReceiver(set receiver.Settings, cfg *Config, next consumer.Metrics) *syrReceiver {
	return &syrReceiver{
		cfg:          cfg,
		logger:       set.Logger,
		nextConsumer: next,
		scraper:      newSyrScraper(cfg, set.Logger),
		done:         make(chan struct{}),
	}
}

// Start startet die Scraping-Goroutine und kehrt sofort zurück.
func (r *syrReceiver) Start(_ context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.run(ctx)
	return nil
}

// Shutdown stoppt die Scraping-Goroutine und wartet auf ihr Ende.
func (r *syrReceiver) Shutdown(ctx context.Context) error {
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

// run ist der Scraping-Loop: sofortiger erster Scrape, dann periodisch.
func (r *syrReceiver) run(ctx context.Context) {
	defer close(r.done)

	// Sofortiger erster Scrape — kein Warten auf den ersten Tick.
	r.collect(ctx)

	ticker := time.NewTicker(r.cfg.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.collect(ctx)
		}
	}
}

// collect ruft den Scraper auf und leitet die Metriken weiter.
func (r *syrReceiver) collect(ctx context.Context) {
	metrics, err := r.scraper.Scrape(ctx)
	if err != nil {
		r.logger.Error("SYR scrape failed", zap.Error(err))
		return
	}

	// Keine Metriken gesammelt (alle deaktiviert oder Fehler).
	if metrics.DataPointCount() == 0 {
		return
	}

	if err := r.nextConsumer.ConsumeMetrics(ctx, metrics); err != nil {
		r.logger.Error("ConsumeMetrics failed", zap.Error(err))
	}
}
