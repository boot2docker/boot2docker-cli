package virtualbox

import (
	"fmt"
	"net"
)

// Port forwarding rule.
type PFRule struct {
	Proto     PFProto
	HostIP    net.IP // can be nil to match any host interface
	HostPort  uint16
	GuestIP   net.IP // can be nil if guest IP is leased from built-in DHCP
	GuestPort uint16
}

type PFProto string

const (
	PFTCP PFProto = "tcp"
	PFUDP         = "udp"
)

func (r PFRule) String() string {
	hostip := ""
	if r.HostIP != nil {
		hostip = r.HostIP.String()
	}
	guestip := ""
	if r.GuestIP != nil {
		guestip = r.GuestIP.String()
	}
	return fmt.Sprintf("%s://%s:%d --> %s:%d",
		r.Proto, hostip, r.HostPort,
		guestip, r.GuestPort)
}

// Format the rule as command-line argument
func (r PFRule) Format() string {
	hostip := ""
	if r.HostIP != nil {
		hostip = r.HostIP.String()
	}
	guestip := ""
	if r.GuestIP != nil {
		guestip = r.GuestIP.String()
	}
	return fmt.Sprintf("%s,%s,%d,%s,%d", r.Proto, hostip, r.HostPort, guestip, r.GuestPort)
}
