package virtualbox

// Network interface card.
type NIC struct {
	Network         NICNetwork
	Hardware        NICHardware
	HostonlyAdapter string
}

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

type NICHardware string

const (
	AMDPCNetPCIII         NICHardware = "Am79C970A"
	AMDPCNetFASTIII                   = "Am79C973"
	IntelPro1000MTDesktop             = "82540EM"
	IntelPro1000TServer               = "82543GC"
	IntelPro1000MTServer              = "82545EM"
	VirtIO                            = "virtio"
)
