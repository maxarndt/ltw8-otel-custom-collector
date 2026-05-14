package knxreceiver

import (
	"math"
	"testing"

	"github.com/vapourismo/knx-go/knx/dpt"
)

func TestDecodeDPT(t *testing.T) {
	tests := []struct {
		name     string
		dptName  string
		data     []byte
		wantVal  float64
		wantErr  bool
	}{
		{
			name:    "1.001 false -> 0.0",
			dptName: "1.001",
			data:    dpt.DPT_1001(false).Pack(),
			wantVal: 0.0,
		},
		{
			name:    "1.001 true -> 1.0",
			dptName: "1.001",
			data:    dpt.DPT_1001(true).Pack(),
			wantVal: 1.0,
		},
		{
			name:    "9.001 temperature 21.5",
			dptName: "9.001",
			data:    dpt.DPT_9001(21.5).Pack(),
			wantVal: 21.5,
		},
		{
			name:    "9.001 temperature -5.0",
			dptName: "9.001",
			data:    dpt.DPT_9001(-5.0).Pack(),
			wantVal: -5.0,
		},
		{
			name:    "9.004 lux 500",
			dptName: "9.004",
			data:    dpt.DPT_9004(500).Pack(),
			wantVal: 500.0,
		},
		{
			name:    "9.007 humidity 65.0",
			dptName: "9.007",
			data:    dpt.DPT_9007(65.0).Pack(),
			wantVal: 65.0,
		},
		{
			name:    "13.010 energy 12345 Wh",
			dptName: "13.010",
			data:    dpt.DPT_13010(12345).Pack(),
			wantVal: 12345.0,
		},
		{
			name:    "13.010 energy negative (shouldn't happen but valid DPT)",
			dptName: "13.010",
			data:    dpt.DPT_13010(-1).Pack(),
			wantVal: -1.0,
		},
		{
			name:    "14.056 power 230.5 W",
			dptName: "14.056",
			data:    dpt.DPT_14056(230.5).Pack(),
			wantVal: 230.5,
		},
		{
			name:    "5.001 percent 50%",
			dptName: "5.001",
			data:    dpt.DPT_5001(50).Pack(),
			wantVal: 50.0,
		},
		{
			// knx-go wire format for DPT 5.x: [0, value] (2 bytes, APCI padding + value).
			name:    "5.010 wire-format value 127",
			dptName: "5.010",
			data:    []byte{0x00, 127},
			wantVal: 127.0,
		},
		{
			name:    "5.010 wire-format value 0",
			dptName: "5.010",
			data:    []byte{0x00, 0},
			wantVal: 0.0,
		},
		{
			name:    "5.010 wire-format value 255",
			dptName: "5.010",
			data:    []byte{0x00, 255},
			wantVal: 255.0,
		},
		{
			name:    "5.010 wrong length (1 byte) returns error",
			dptName: "5.010",
			data:    []byte{127},
			wantErr: true,
		},
		{
			name:    "5.010 wrong length (3 bytes) returns error",
			dptName: "5.010",
			data:    []byte{0, 1, 2},
			wantErr: true,
		},
		{
			name:    "5.010 empty data returns error",
			dptName: "5.010",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "unknown DPT returns error",
			dptName: "99.999",
			data:    []byte{0x00},
			wantErr: true,
		},
		{
			// DPT 232.600 (RGB) is registered in knx-go as a struct — should fall
			// through extractFloat64's default case with a clear error message.
			name:    "232.600 struct DPT returns clear error",
			dptName: "232.600",
			data:    []byte{0x00, 0xFF, 0x00, 0xFF},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeDPT(tt.dptName, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Use tolerance for float32-encoded DPTs (KNX 16-bit float has ~0.5% precision).
			const tol = 0.5
			if math.Abs(got-tt.wantVal) > tol {
				t.Errorf("got %v, want %v (tolerance %v)", got, tt.wantVal, tol)
			}
		})
	}
}

func TestDPTUnit(t *testing.T) {
	tests := map[string]string{
		"13.010":  "Wh",
		"14.056":  "W",
		"9.001":   "°C",
		"9.004":   "lux",
		"9.007":   "%",
		"7.012":   "mA",
		"5.001":   "%",
		"5.010":   "pulses", // custom decoder
		"99.999":  "",       // unknown -> empty
	}
	for dpt, want := range tests {
		if got := DPTUnit(dpt); got != want {
			t.Errorf("DPTUnit(%q) = %q, want %q", dpt, got, want)
		}
	}
}
