package driver

import (
	"errors"
	"fmt"
)

type InitFunc func(i *MachineConfig) (Machine, error)

type MachineState string

const (
	Poweroff = MachineState("poweroff")
	Running  = MachineState("running")
	Paused   = MachineState("paused")
	Saved    = MachineState("saved")
	Aborted  = MachineState("aborted")
)

// Machine represents a virtual machine instance
type Machine interface {
	Start() error
	Save() error
	Pause() error
	Stop() error
	Refresh() error
	Poweroff() error
	Restart() error
	Reset() error
	Delete() error
	Modify() error
	AddNATPF(n int, name string, rule PFRule) error
	DelNATPF(n int, name string) error
	SetNIC(n int, nic NIC) error
	AddStorageCtl(name string, ctl StorageController) error
	DelStorageCtl(name string) error
	AttachStorage(ctlName string, medium StorageMedium) error
	GetState() MachineState
	GetSerialFile() string
	GetDockerPort() uint
	GetSSHPort() uint
}

var (
	// All registred machines
	machines map[string]InitFunc

	ErrNotSupported    = errors.New("machine not supported")
	ErrMachineNotExist = errors.New("machine not exist")
	ErrPrerequisites   = errors.New("prerequisites for machine not satisfied (hypervisor installed?)")
)

func init() {
	machines = make(map[string]InitFunc)
}

func Register(driver string, initFunc InitFunc) error {
	if _, exists := machines[driver]; exists {
		return fmt.Errorf("Driver already registered %s", driver)
	}
	machines[driver] = initFunc

	return nil
}

func GetMachine(i *MachineConfig) (Machine, error) {
	if initFunc, exists := machines[i.Driver]; exists {
		return initFunc(i)
	}
	return nil, ErrNotSupported
}
