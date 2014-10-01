package vmware

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/boot2docker/boot2docker-cli/driver"
	"github.com/ogier/pflag"
)

var (
	verbose bool // Verbose mode (Local copy of B2D.Verbose).
	cfg     DriverCfg
)

type DriverCfg struct {
	VMRUN string // Path to VBoxManage utility.
	VMDK  string // base VMDK to use as persistent disk.
}

func init() {
	if err := driver.Register("fusion", InitFunc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver. Error : %s", err.Error())
		os.Exit(1)
	}
	if err := driver.RegisterConfig("fusion", ConfigFlags); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver config. Error : %s", err.Error())
		os.Exit(1)
	}
}

// Initialize the Machine.
func InitFunc(mc *driver.MachineConfig) (driver.Machine, error) {
	verbose = mc.Verbose

	m, err := GetMachine(getVMX(mc))
	if err != nil && mc.Init == true {
		return CreateMachine(mc)
	}
	return m, err
}

// Add cmdline params for this driver
func ConfigFlags(mc *driver.MachineConfig, flags *pflag.FlagSet) error {
	cfg.VMRUN = "/Applications/VMware Fusion.app/Contents/Library/vmrun"
	flags.StringVar(&cfg.VMRUN, "vmrun", cfg.VMRUN, "path to vmrun management utility.")

	return nil
}

// Machine information.
type Machine struct {
	Name       string
	State      driver.MachineState
	CPUs       uint
	Memory     uint // main memory (in MB)
	VRAM       uint // video memory (in MB)
	VMX        string
	OSType     string
	BootOrder  []string // max 4 slots, each in {none|floppy|dvd|disk|net}
	DockerPort uint
	SSHPort    uint
}

// Refresh reloads the machine information.
func (m *Machine) Refresh() error {
	mm, err := GetMachine(m.Name)
	mm.State = driver.Running
	if err != nil {
		return err
	}
	*m = *mm
	return nil
}

// Start starts the machine.
func (m *Machine) Start() error {
	m.State = driver.Running
	vmrun("start", m.VMX, "nogui")
	return nil
}

// Suspend suspends the machine and saves its state to disk.
func (m *Machine) Save() error {
	m.State = driver.Saved
	fmt.Printf("Save %s: %s\n", m.Name, m.State)
	return nil
}

// Pause pauses the execution of the machine.
func (m *Machine) Pause() error {
	m.State = driver.Paused
	fmt.Printf("Pause %s: %s\n", m.Name, m.State)
	return nil
}

// Stop gracefully stops the machine.
func (m *Machine) Stop() error {
	m.State = driver.Poweroff
	vmrun("stop", m.VMX, "nogui")
	return nil
}

// Poweroff forcefully stops the machine. State is lost and might corrupt the disk image.
func (m *Machine) Poweroff() error {
	m.State = driver.Poweroff
	fmt.Printf("Poweroff %s: %s\n", m.Name, m.State)
	return nil
}

// Restart gracefully restarts the machine.
func (m *Machine) Restart() error {
	m.State = driver.Running
	fmt.Printf("Restart %s: %s\n", m.Name, m.State)
	return nil
}

// Reset forcefully restarts the machine. State is lost and might corrupt the disk image.
func (m *Machine) Reset() error {
	m.State = driver.Running
	fmt.Printf("Reset %s: %s\n", m.Name, m.State)
	return nil
}

// Get vm name
func (m *Machine) GetName() string {
	return m.Name
}

// Get current state
func (m *Machine) GetState() driver.MachineState {
	return m.State
}

// Get serial file
func (m *Machine) GetSerialFile() string {
	return ""
}

// Get Docker port
func (m *Machine) GetDockerPort() uint {
	return m.DockerPort
}

// Get SSH port
func (m *Machine) GetSSHPort() uint {
	return m.SSHPort
}

// Delete deletes the machine and associated disk images.
func (m *Machine) Delete() error {
	fmt.Printf("Delete %s: %s\n", m.Name, m.State)
	return nil
}

// Modify changes the settings of the machine.
func (m *Machine) Modify() error {
	fmt.Printf("Modify %s: %s\n", m.Name, m.State)
	return m.Refresh()
}

// AddNATPF adds a NAT port forarding rule to the n-th NIC with the given name.
func (m *Machine) AddNATPF(n int, name string, rule driver.PFRule) error {
	fmt.Println("Add NAT PF")
	return nil
}

// DelNATPF deletes the NAT port forwarding rule with the given name from the n-th NIC.
func (m *Machine) DelNATPF(n int, name string) error {
	fmt.Println("Del NAT PF")
	return nil
}

// SetNIC set the n-th NIC.
func (m *Machine) SetNIC(n int, nic driver.NIC) error {
	fmt.Println("Set NIC")
	return nil
}

// AddStorageCtl adds a storage controller with the given name.
func (m *Machine) AddStorageCtl(name string, ctl driver.StorageController) error {
	fmt.Println("Add storage ctl")
	return nil
}

// DelStorageCtl deletes the storage controller with the given name.
func (m *Machine) DelStorageCtl(name string) error {
	fmt.Println("Del storage ctl")
	return nil
}

// AttachStorage attaches a storage medium to the named storage controller.
func (m *Machine) AttachStorage(ctlName string, medium driver.StorageMedium) error {
	fmt.Println("Attach storage")
	return nil
}

// GetMachine finds a machine.
func GetMachine(vmx string) (*Machine, error) {
	if _, err := os.Stat(vmx); os.IsNotExist(err) {
		return nil, ErrMachineNotExist
	}
	m := &Machine{VMX: vmx}
	return m, nil
}

// CreateMachine creates a new virtual machine.
func CreateMachine(mc *driver.MachineConfig) (*Machine, error) {
	if err := os.MkdirAll(getBaseFolder(mc), 0755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(getVMX(mc)); err == nil {
		return nil, ErrMachineExist
	}

	vmxt := template.Must(template.New("vmx").Parse(vmx))
	vmxfile, err := os.Create(getVMX(mc))
	if err != nil {
		return nil, err
	}
	vmxt.Execute(vmxfile, mc)
	return nil, nil
}

func getBaseFolder(mc *driver.MachineConfig) string {
	return filepath.Join(mc.Dir, mc.VM)
}
func getVMX(mc *driver.MachineConfig) string {
	return filepath.Join(getBaseFolder(mc), strings.Join([]string{mc.VM, "vmx"}, "."))
}
