package virtualbox

// NIC represents a virtualized network interface card.
type NIC struct {
	Network         NICNetwork
	Hardware        NICHardware
	HostonlyAdapter string
}

// NICNetwork represents the type of NIC networks.
type NICNetwork string

const (
	NICNetAbsent       NICNetwork = "none"
	NICNetDisconnected            = "null"
	NICNetNAT                     = "nat"
	NICNetBridged                 = "bridged"
	NICNetInternal                = "intnet"
	NICNetHostonly                = "hostonly"
	NICNetGeneric                 = "generic"
)

// NICHardware represents the type of NIC hardware.
type NICHardware string

const (
	AMDPCNetPCIII         NICHardware = "Am79C970A"
	AMDPCNetFASTIII                   = "Am79C973"
	IntelPro1000MTDesktop             = "82540EM"
	IntelPro1000TServer               = "82543GC"
	IntelPro1000MTServer              = "82545EM"
	VirtIO                            = "virtio"
)
