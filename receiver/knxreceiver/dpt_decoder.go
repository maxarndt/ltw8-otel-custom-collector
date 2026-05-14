package knxreceiver

import (
	"fmt"
	"reflect"

	"github.com/vapourismo/knx-go/knx/dpt"
)

// customDPTDecoders holds decoders for DPTs that are either missing from the
// knx-go registry or whose native representation cannot be flattened to float64
// by the reflection-based path. Add an entry here to support a new DPT.
var customDPTDecoders = map[string]func(data []byte) (float64, error){
	// DPT 5.010 (counter pulses, unsigned 8-bit) is not in the knx-go registry.
	"5.010": decodeDPT5010,
}

// DecodeDPT decodes raw KNX telegram data using the named DPT (e.g. "9.001").
// Returns the decoded value as float64.
// Resolution order:
//  1. Custom decoders in customDPTDecoders (overrides and additions to knx-go).
//  2. knx-go registry via dpt.Produce(), with extractFloat64() for scalar types.
//
// Unknown DPTs or non-scalar DPTs return an error — they are logged and skipped
// by the caller.
func DecodeDPT(dptName string, data []byte) (float64, error) {
	if dec, ok := customDPTDecoders[dptName]; ok {
		return dec(data)
	}

	d, ok := dpt.Produce(dptName)
	if !ok {
		return 0, fmt.Errorf("unsupported DPT %q (not in knx-go registry; register a handler in customDPTDecoders if needed)", dptName)
	}

	if err := d.Unpack(data); err != nil {
		return 0, fmt.Errorf("DPT %s unpack: %w", dptName, err)
	}

	return extractFloat64(dptName, d)
}

// extractFloat64 converts a dpt.DatapointValue to float64 via reflection.
// Handles bool, int*, uint*, float32/float64 underlying types. Non-scalar
// (struct) DPTs are rejected — register a handler in customDPTDecoders for them.
func extractFloat64(dptName string, d dpt.DatapointValue) (float64, error) {
	v := reflect.ValueOf(d)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return 1.0, nil
		}
		return 0.0, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return v.Float(), nil
	default:
		return 0, fmt.Errorf("DPT %s: non-scalar type %s cannot be mapped to float64; register a handler in customDPTDecoders", dptName, v.Type())
	}
}

// decodeDPT5010 handles DPT 5.010 (counter pulses, unsigned 8-bit, 0–255).
// Wire format matches knx-go's packU8/unpackU8: 2 bytes, with the first byte
// being APCI padding and the second byte the value.
func decodeDPT5010(data []byte) (float64, error) {
	if len(data) != 2 {
		return 0, fmt.Errorf("DPT 5.010: expected 2 bytes, got %d", len(data))
	}
	return float64(data[1]), nil
}
