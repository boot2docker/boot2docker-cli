package driver

import "net"

// Machine config.
type MachineConfig struct {
	// Gereral flags.
	Verbose bool
	VBM     string
	Driver  string

	// basic config
	SSH      string // SSH client executable
	SSHGen   string // SSH keygen executable
	SSHKey   string // SSH key to send to the vm
	VM       string // virtual machine name
	Dir      string // boot2docker directory
	ISO      string // boot2docker ISO image path
	VMDK     string // base VMDK to use as persistent disk
	DiskSize uint   // VM disk image size (MB)
	Memory   uint   // VM memory size (MB)

	// NAT network: port forwarding
	SSHPort    uint16 // host SSH port (forward to port 22 in VM)
	DockerPort uint16 // host Docker port (forward to port 2375 in VM)

	// host-only network
	HostIP      net.IP
	DHCPIP      net.IP
	NetMask     net.IPMask
	LowerIP     net.IP
	UpperIP     net.IP
	DHCPEnabled bool

	// Serial console pipe/socket
	Serial     bool
	SerialFile string
}
