package knxreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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
	r, err := newKNXReceiver(set, testConfig(), sink)
	if err != nil {
		t.Fatalf("newKNXReceiver: %v", err)
	}

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

// TestReceiver_ReadStartup_DoesNotBlockInbound exercises B2: when many ReadStartup
// requests are pending, an incoming GroupWrite must still be processed promptly —
// listen() runs in parallel with readStartup().
func TestReceiver_ReadStartup_DoesNotBlockInbound(t *testing.T) {
	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}

	// Custom config: 3 ReadStartup addresses with a 100ms gap = 300ms total — long
	// enough that the test would fail in the old sequential implementation.
	cfg := &Config{
		Connection: ConnectionConfig{
			Type:             ConnectionTypeRouter,
			MulticastAddress: "224.0.23.12:3671",
		},
		ReadStartupInterval: 100 * time.Millisecond,
		AddressConfigs: map[string]*AddressConfig{
			"1/1/1": {Name: "a", DPT: "13.010", Export: true, MetricType: MetricTypeSum, ReadStartup: true},
			"1/1/2": {Name: "b", DPT: "13.010", Export: true, MetricType: MetricTypeSum, ReadStartup: true},
			"1/1/3": {Name: "c", DPT: "13.010", Export: true, MetricType: MetricTypeSum, ReadStartup: true},
			"2/1/1": {Name: "temp", DPT: "9.001", Export: true, MetricType: MetricTypeGauge, ReadStartup: false},
		},
	}

	set := receivertest.NewNopSettings(receivertest.NopType)
	r, err := newKNXReceiver(set, cfg, sink)
	if err != nil {
		t.Fatalf("newKNXReceiver: %v", err)
	}
	orig := NewKNXClient
	t.Cleanup(func() { NewKNXClient = orig })
	NewKNXClient = func(_ ConnectionConfig) (KNXClient, error) { return mock, nil }

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Push an event while readStartup is still iterating (well before 300ms).
	ga, _ := cemi.NewGroupAddrString("2/1/1")
	src, _ := cemi.NewIndividualAddrString("1.1.5")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupWrite,
		Source:      src,
		Destination: ga,
		Data:        dpt.DPT_9001(22.5).Pack(),
	}

	// Give listen() a brief moment to consume the event — much shorter than the
	// 300ms readStartup would need to finish.
	time.Sleep(50 * time.Millisecond)

	// Snapshot the metric count before shutdown to assert prompt processing.
	got := len(sink.AllMetrics())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	if got == 0 {
		t.Fatal("expected the GroupWrite to be processed while readStartup was still running")
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

// TestReceiver_Telemetry_Counts verifies the self-telemetry counters and the
// connection_up up/down counter against a real SDK MeterProvider.
func TestReceiver_Telemetry_Counts(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })

	mock := newMockKNXClient()
	sink := &consumertest.MetricsSink{}

	set := receivertest.NewNopSettings(receivertest.NopType)
	set.TelemetrySettings = tel.NewTelemetrySettings()

	r, err := newKNXReceiver(set, testConfig(), sink)
	if err != nil {
		t.Fatalf("newKNXReceiver: %v", err)
	}
	orig := NewKNXClient
	t.Cleanup(func() { NewKNXClient = orig })
	NewKNXClient = func(_ ConnectionConfig) (KNXClient, error) { return mock, nil }

	if err := r.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Successful telegram on 1/1/2 (Gauge, DPT 14.056).
	ga, _ := cemi.NewGroupAddrString("1/1/2")
	src, _ := cemi.NewIndividualAddrString("1.1.5")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupWrite,
		Source:      src,
		Destination: ga,
		Data:        dpt.DPT_14056(100.0).Pack(),
	}

	// Decode error: send a 14.056 telegram with too-short data for address 1/1/1
	// (configured with DPT 13.010 expecting 6 bytes — passing 1 byte triggers
	// an unpack error).
	gaErr, _ := cemi.NewGroupAddrString("1/1/1")
	mock.inbound <- knx.GroupEvent{
		Command:     knx.GroupWrite,
		Source:      src,
		Destination: gaErr,
		Data:        []byte{0x00},
	}

	time.Sleep(80 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.Shutdown(ctx)

	if got := sumIntCounter(t, tel, "knxreceiver.telegrams_received"); got != 1 {
		t.Errorf("telegrams_received: got %d, want 1", got)
	}
	if got := sumIntCounter(t, tel, "knxreceiver.decode_errors"); got != 1 {
		t.Errorf("decode_errors: got %d, want 1", got)
	}
	// connection_up has been Add(+1) on connect, Add(-1) on shutdown → net 0.
	if got := sumIntCounter(t, tel, "knxreceiver.connection_up"); got != 0 {
		t.Errorf("connection_up: got %d, want 0", got)
	}
	// reconnects has not been incremented in a clean-shutdown path — the metric
	// may legitimately be absent from the reader output.
}

// sumIntCounter aggregates all data points of an int64 Sum metric.
func sumIntCounter(t *testing.T, tel *componenttest.Telemetry, name string) int64 {
	t.Helper()
	m, err := tel.GetMetric(name)
	if err != nil {
		t.Fatalf("GetMetric(%q): %v", name, err)
	}
	switch d := m.Data.(type) {
	case metricdata.Sum[int64]:
		var sum int64
		for _, dp := range d.DataPoints {
			sum += dp.Value
		}
		return sum
	default:
		t.Fatalf("metric %q has unexpected data type %T", name, m.Data)
		return 0
	}
}
