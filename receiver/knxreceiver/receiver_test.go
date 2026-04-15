package knxreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func testConfig() *Config {
	return &Config{
		Connection: ConnectionConfig{
			Type:             ConnectionTypeRouter,
			MulticastAddress: "224.0.23.12:3671",
		},
		ReadStartupInterval: 10 * time.Millisecond,
		AddressConfigs: map[string]*AddressConfig{
			"1/1/1": {
				Name:        "strom_wh",
				DPT:         "13.010",
				Export:      true,
				MetricType:  MetricTypeSum,
				ReadStartup: true,
				Labels:      map[string]string{"room": "kueche"},
			},
			"1/1/2": {
				Name:        "leistung_w",
				DPT:         "14.056",
				Export:      true,
				MetricType:  MetricTypeGauge,
				ReadStartup: false,
			},
			"1/1/3": {
				Name:        "not_exported",
				DPT:         "9.001",
				Export:      false,
				MetricType:  MetricTypeGauge,
				ReadStartup: false,
			},
		},
	}
}

func makeReceiver(t *testing.T, mock *mockKNXClient, sink *consumertest.MetricsSink) *knxReceiver {
	t.Helper()
	set := receivertest.NewNopSettings(receivertest.NopType)
	r := newKNXReceiver(set, testConfig(), sink)

	// Replace the global NewKNXClient with a factory that returns our mock.
	orig := NewKNXClient
	t.Cleanup(func() { NewKNXClient = orig })
	NewKNXClient = func(_ ConnectionConfig) (KNXClient, error) {
		return mock, nil
	}
	return r
}

func TestReceiver_StartShutdown(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}
	r := makeReceiver(t, mock, sink)

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give the goroutine a moment to reach the listen loop.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestReceiver_HandleGroupWrite(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}
	r := makeReceiver(t, mock, sink)

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for ReadStartup to finish (1 address × 10ms interval).
	time.Sleep(50 * time.Millisecond)

	// Send a GroupWrite for 1/1/1 (DPT 13.010, Wh counter = 500).
	ga, _ := cemi.NewGroupAddrString("1/1/1")
	src, _ := cemi.NewIndividualAddrString("1.1.5")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupWrite,
		Source:      src,
		Destination: ga,
		Data:        dpt.DPT_13010(500).Pack(),
	}

	// Give the receiver time to process the event.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	all := sink.AllMetrics()
	if len(all) == 0 {
		t.Fatal("expected at least one metric batch")
	}

	// Find the metric named "strom_wh".
	var found pmetric.NumberDataPoint
	var ok bool
	for _, md := range all {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					m := sm.Metrics().At(k)
					if m.Name() == "strom_wh" {
						found = m.Sum().DataPoints().At(0)
						ok = true
					}
				}
			}
		}
	}
	if !ok {
		t.Fatal("metric 'strom_wh' not found")
	}
	if found.DoubleValue() != 500.0 {
		t.Errorf("value: got %v, want 500.0", found.DoubleValue())
	}
}

func TestReceiver_SkipsGroupRead(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}
	r := makeReceiver(t, mock, sink)

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	// Send a GroupRead — should be silently ignored.
	ga, _ := cemi.NewGroupAddrString("1/1/1")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupRead,
		Destination: ga,
		Data:        nil,
	}
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	// No metrics should have been emitted for the Read command.
	for _, md := range sink.AllMetrics() {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					t.Errorf("unexpected metric emitted: %s", sm.Metrics().At(k).Name())
				}
			}
		}
	}
}

func TestReceiver_SkipsUnexportedAddress(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}
	r := makeReceiver(t, mock, sink)

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	// 1/1/3 has Export=false.
	ga, _ := cemi.NewGroupAddrString("1/1/3")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupWrite,
		Destination: ga,
		Data:        dpt.DPT_9001(21.5).Pack(),
	}
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	if len(sink.AllMetrics()) > 0 {
		t.Error("expected no metrics for unexported address")
	}
}

func TestReceiver_ReadStartup(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}
	r := makeReceiver(t, mock, sink)

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for ReadStartup to complete (1 address with read_startup:true × 10ms).
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	// Exactly 1 GroupRead should have been sent (for "1/1/1" only).
	readCount := 0
	for _, e := range mock.sent {
		if e.Command == knx.GroupRead {
			readCount++
		}
	}
	if readCount != 1 {
		t.Errorf("expected 1 GroupRead sent, got %d", readCount)
	}
}
