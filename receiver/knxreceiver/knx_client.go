package knxreceiver

import (
	"fmt"

	"github.com/vapourismo/knx-go/knx"
)

// KNXClient abstracts knx.GroupTunnel and knx.GroupRouter behind a single interface.
// Using an interface allows tests to inject a mock without a real KNX bus.
type KNXClient interface {
	// Inbound returns the channel for incoming KNX group events.
	Inbound() <-chan knx.GroupEvent
	// Send transmits a group event onto the KNX bus.
	Send(event knx.GroupEvent) error
	// Close terminates the connection and releases resources.
	Close()
}

// knxTunnelClient wraps knx.GroupTunnel.
type knxTunnelClient struct {
	tunnel knx.GroupTunnel
}

func (c *knxTunnelClient) Inbound() <-chan knx.GroupEvent { return c.tunnel.Inbound() }
func (c *knxTunnelClient) Send(e knx.GroupEvent) error    { return c.tunnel.Send(e) }
func (c *knxTunnelClient) Close()                         { c.tunnel.Close() }

// knxRouterClient wraps knx.GroupRouter.
type knxRouterClient struct {
	router knx.GroupRouter
}

func (c *knxRouterClient) Inbound() <-chan knx.GroupEvent { return c.router.Inbound() }
func (c *knxRouterClient) Send(e knx.GroupEvent) error    { return c.router.Send(e) }
func (c *knxRouterClient) Close()                         { c.router.Close() }

// NewKNXClient is a variable so tests can replace it with a mock factory.
var NewKNXClient = newKNXClient

func newKNXClient(cfg ConnectionConfig) (KNXClient, error) {
	switch cfg.Type {
	case ConnectionTypeTunnel:
		t, err := knx.NewGroupTunnel(cfg.Endpoint, knx.DefaultTunnelConfig)
		if err != nil {
			return nil, fmt.Errorf("KNX tunnel connect to %s: %w", cfg.Endpoint, err)
		}
		return &knxTunnelClient{tunnel: t}, nil

	case ConnectionTypeRouter:
		r, err := knx.NewGroupRouter(cfg.MulticastAddress, knx.DefaultRouterConfig)
		if err != nil {
			return nil, fmt.Errorf("KNX router connect to %s: %w", cfg.MulticastAddress, err)
		}
		return &knxRouterClient{router: r}, nil

	default:
		return nil, fmt.Errorf("unknown connection type: %q", cfg.Type)
	}
}
