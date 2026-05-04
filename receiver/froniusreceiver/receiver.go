package froniusreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// froniusReceiver ist der Fronius-Metrics Receiver.
type froniusReceiver struct {
	config    *Config
	settings  receiver.Settings
	consumer  consumer.Metrics
	scraper   *FroniusScraper
	converter *Converter
	ticker    *time.Ticker
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
}

// newFroniusReceiver erstellt einen neuen Fronius Receiver.
func newFroniusReceiver(
	settings receiver.Settings,
	config *Config,
	consumer consumer.Metrics,
) *froniusReceiver {
	return &froniusReceiver{
		config:   config,
		settings: settings,
		consumer: consumer,
		done:     make(chan struct{}),
	}
}

// Start startet den Fronius Receiver.
func (r *froniusReceiver) Start(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting Fronius receiver", zap.String("endpoint", r.config.Endpoint))

	// Create Scraper
	scraper, err := NewFroniusScraper(
		r.config.Endpoint,
		r.config.Timeout,
		r.config.Metrics,
		r.settings.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create scraper: %w", err)
	}
	r.scraper = scraper

	// Create Converter (mit Endpoint-Resource-Attribut)
	r.converter = NewConverterWithEndpoint(r.settings.Logger, r.config.Endpoint)

	// Create context for goroutine
	r.ctx, r.cancel = context.WithCancel(ctx)

	// Start scraping goroutine
	go r.run()

	r.settings.Logger.Info("Fronius receiver started successfully")
	return nil
}

// Shutdown stoppt den Fronius Receiver.
func (r *froniusReceiver) Shutdown(ctx context.Context) error {
	r.settings.Logger.Info("Stopping Fronius receiver")

	if r.ticker != nil {
		r.ticker.Stop()
	}

	if r.cancel != nil {
		r.cancel()
	}

	// Wait for goroutine to finish with timeout
	select {
	case <-r.done:
		r.settings.Logger.Info("Fronius receiver stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("fronius receiver shutdown timeout: %w", ctx.Err())
	}
}

// run ist die Hauptschleife des Scrapers.
func (r *froniusReceiver) run() {
	defer close(r.done)

	// Erstelle Ticker für Collection Interval
	r.ticker = time.NewTicker(r.config.CollectionInterval)
	defer r.ticker.Stop()

	r.settings.Logger.Debug("Fronius scraping loop started",
		zap.Duration("collection_interval", r.config.CollectionInterval))

	// Führe erste Scrape sofort aus (nicht auf Tick warten)
	r.scrapeOnce()

	// Loop für weitere Scrapes
	for {
		select {
		case <-r.ctx.Done():
			r.settings.Logger.Debug("Fronius scraping loop stopped")
			return

		case <-r.ticker.C:
			r.scrapeOnce()
		}
	}
}

// scrapeOnce führt einen einzelnen Scrape-Zyklus durch.
func (r *froniusReceiver) scrapeOnce() {
	ctx, cancel := context.WithTimeout(r.ctx, r.config.Timeout)
	defer cancel()

	start := time.Now()

	// Scrape
	scraped, err := r.scraper.Scrape(ctx)
	if err != nil {
		r.settings.Logger.Warn("Scrape failed", zap.Error(err))
		return
	}

	// Convert to pmetrics
	metrics := r.converter.ConvertToMetrics(ctx, scraped)

	// Push to consumer
	if err := r.consumer.ConsumeMetrics(ctx, metrics); err != nil {
		r.settings.Logger.Error("Failed to consume metrics", zap.Error(err))
		return
	}

	duration := time.Since(start)
	r.settings.Logger.Debug("Scrape cycle completed",
		zap.Duration("duration", duration),
		zap.Int("metrics_count", metrics.MetricCount()),
	)
}
