package driver

import "net"

// DHCP server info.
type DHCP struct {
	NetworkName string
	IPv4        net.IPNet
	LowerIP     net.IP
	UpperIP     net.IP
	Enabled     bool
}
