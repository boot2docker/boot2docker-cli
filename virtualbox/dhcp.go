package virtualbox

import (
	"bufio"
	"net"
	"strings"

	"github.com/boot2docker/boot2docker-cli/driver"
)

func addDHCP(kind, name string, d driver.DHCP) error {
	command := "modify"

	// On some platforms (OSX), creating a hostonlyinterface adds a default dhcpserver
	// While on others (Windows?) it does not.
	dhcps, err := DHCPs()
	if err != nil {
		return err
	}

	if _, ok := dhcps[name]; !ok {
		command = "add"
	}

	args := []string{"dhcpserver", command,
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

// AddInternalDHCP adds a DHCP server to an internal network.
func AddInternalDHCP(netname string, d driver.DHCP) error {
	return addDHCP("--netname", netname, d)
}

// AddHostonlyDHCP adds a DHCP server to a host-only network.
func AddHostonlyDHCP(ifname string, d driver.DHCP) error {
	return addDHCP("--netname", "HostInterfaceNetworking-"+ifname, d)
}

// DHCPs gets all DHCP server settings in a map keyed by DHCP.NetworkName.
func DHCPs() (map[string]*driver.DHCP, error) {
	out, err := vbmOut("list", "dhcpservers")
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(strings.NewReader(out))
	m := map[string]*driver.DHCP{}
	dhcp := &driver.DHCP{}
	for s.Scan() {
		line := s.Text()
		if line == "" {
			m[dhcp.NetworkName] = dhcp
			dhcp = &driver.DHCP{}
			continue
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
			dhcp.Enabled = (val == "Yes")
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
