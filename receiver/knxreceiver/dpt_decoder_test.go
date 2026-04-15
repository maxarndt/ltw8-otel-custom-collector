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
			name:    "5.010 uint8 value 127",
			dptName: "5.010",
			data:    []byte{127},
			wantVal: 127.0,
		},
		{
			name:    "5.010 uint8 value 0",
			dptName: "5.010",
			data:    []byte{0},
			wantVal: 0.0,
		},
		{
			name:    "5.010 uint8 value 255",
			dptName: "5.010",
			data:    []byte{255},
			wantVal: 255.0,
		},
		{
			name:    "5.010 multi-byte uses last byte",
			dptName: "5.010",
			data:    []byte{0x00, 0xAB},
			wantVal: 0xAB,
		},
		{
			name:    "unknown DPT returns error",
			dptName: "99.999",
			data:    []byte{0x00},
			wantErr: true,
		},
		{
			name:    "5.010 empty data returns error",
			dptName: "5.010",
			data:    []byte{},
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
