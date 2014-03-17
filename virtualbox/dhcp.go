package virtualbox

import (
	"bufio"
	"bytes"
	"net"
)

// DHCP server info.
type DHCP struct {
	NetworkName string
	IPv4        net.IPNet
	LowerIP     net.IP
	UpperIP     net.IP
	Enabled     bool
}

func addDHCP(kind, name string, d DHCP) error {
	args := []string{"dhcpserver", "add",
		kind, name,
		"--ip", d.IPv4.IP.String(),
		"--netmask", net.IP(d.IPv4.Mask).String(),
		"--lowerip", d.LowerIP.String(),
		"--upperip", d.UpperIP.String(),
	}
	if d.Enabled {
		args = append(args, "--enable")
	} else {
		args = append(args, "--disable")
	}
	return vbm(args...)
}

// Add the DHCP server to an internal network
func AddInternalDHCP(netname string, d DHCP) error {
	return addDHCP("--netname", netname, d)
}

// Add the DHCP server to a host-only network
func AddHostonlyDHCP(ifname string, d DHCP) error {
	return addDHCP("--ifname", ifname, d)
}

// Get all DHCP server settings. Map is keyed by DHCP.NetworkName.
func DHCPs() (map[string]*DHCP, error) {
	b, err := vbmOut("list", "dhcpservers")
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(bytes.NewReader(b))
	m := make(map[string]*DHCP)
	dhcp := &DHCP{}
	for s.Scan() {
		line := s.Text()
		if line == "" {
			m[dhcp.NetworkName] = dhcp
			dhcp = &DHCP{}
		}
		res := reColonLine.FindStringSubmatch(line)
		if res == nil {
			continue
		}
		switch key, val := res[1], res[2]; key {
		case "NetworkName":
			dhcp.NetworkName = val
		case "IP":
			dhcp.IPv4.IP = net.ParseIP(val)
		case "upperIPAddress":
			dhcp.UpperIP = net.ParseIP(val)
		case "lowerIPAddress":
			dhcp.LowerIP = net.ParseIP(val)
		case "NetworkMask":
			dhcp.IPv4.Mask = ParseIPv4Mask(val)
		case "Enabled":
			if val == "Yes" {
				dhcp.Enabled = true
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
