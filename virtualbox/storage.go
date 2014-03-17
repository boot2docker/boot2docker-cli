package virtualbox

type StorageController struct {
	SysBus      string // ide/sata/scsi/floppy
	Ports       uint   // SATA port count 1--30
	Chipset     string // LSILogic|LSILogicSAS|BusLogic|IntelAHCI|PIIX3|PIIX4|ICH6|I82078
	HostIOCache bool
	Bootable    bool
}

type StorageMedium struct {
	Port      uint
	Device    uint
	DriveType string // dvddrive|hdd|fdd
	Medium    string // none|emptydrive|<uuid>|<filename|host:<drive>|iscsi
}
