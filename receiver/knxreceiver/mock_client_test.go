package knxreceiver

import (
	"github.com/vapourismo/knx-go/knx"
)

// mockKNXClient is a test double for KNXClient.
type mockKNXClient struct {
	inbound chan knx.GroupEvent
	sent    []knx.GroupEvent
	sendErr error
}

func newMockKNXClient() *mockKNXClient {
	return &mockKNXClient{
		inbound: make(chan knx.GroupEvent, 32),
	}
}

func (m *mockKNXClient) Inbound() <-chan knx.GroupEvent { return m.inbound }

func (m *mockKNXClient) Send(e knx.GroupEvent) error {
	m.sent = append(m.sent, e)
	return m.sendErr
}

func (m *mockKNXClient) Close() {
	// Drain and close so the inbound range terminates.
	select {
	case <-m.inbound:
	default:
	}
	close(m.inbound)
}
