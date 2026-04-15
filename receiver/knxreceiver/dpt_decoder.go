package knxreceiver

import (
	"fmt"
	"reflect"

	"github.com/vapourismo/knx-go/knx/dpt"
)

// DecodeDPT decodes raw KNX telegram data using the named DPT (e.g. "9.001").
// Returns the decoded value as float64.
// Unknown DPTs return an error — they are logged and skipped by the caller.
func DecodeDPT(dptName string, data []byte) (float64, error) {
	// DPT 5.010 (unsigned 8-bit counter value) is not in the knx-go registry.
	if dptName == "5.010" {
		return decodeDPT5010(data)
	}

	d, ok := dpt.Produce(dptName)
	if !ok {
		return 0, fmt.Errorf("unsupported DPT %q", dptName)
	}

	if err := d.Unpack(data); err != nil {
		return 0, fmt.Errorf("DPT %s unpack: %w", dptName, err)
	}

	return extractFloat64(d)
}

// extractFloat64 converts a dpt.DatapointValue to float64 via reflection.
// Handles bool, int*, uint*, float32/float64 underlying types.
func extractFloat64(d dpt.DatapointValue) (float64, error) {
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
		return 0, fmt.Errorf("cannot convert DPT value of type %s to float64", v.Type())
	}
}

// decodeDPT5010 handles DPT 5.010 (unsigned 8-bit value, 0–255).
// The value is stored in the last byte of the telegram data.
func decodeDPT5010(data []byte) (float64, error) {
	if len(data) < 1 {
		return 0, fmt.Errorf("DPT 5.010: expected at least 1 byte, got %d", len(data))
	}
	return float64(data[len(data)-1]), nil
}
