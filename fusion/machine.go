package fusion

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/boot2docker/boot2docker-cli/driver"
	"github.com/ogier/pflag"
)

var (
	verbose bool // Verbose mode.
	cfg     DriverCfg
)

type DriverCfg struct {
	VMRUN    string // Path to vmrun utility.
	VDISKMAN string // Path to vdiskmanager utility.
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
	flags.StringVar(&cfg.VMRUN, "vmrun", cfg.VMRUN, "path to vmrun utility.")

	cfg.VDISKMAN = "/Applications/VMware Fusion.app/Contents/Library/vmware-vdiskmanager"
	flags.StringVar(&cfg.VDISKMAN, "vmdiskman", cfg.VDISKMAN, "path to vdiskmanager utility.")

	return nil
}

// Machine information.
type Machine struct {
	Name   string
	State  driver.MachineState
	CPUs   uint64
	Memory uint64 // main memory (in MB)
	VMX    string
	OSType string
}

// Refresh reloads the machine information.
func (m *Machine) Refresh() error {
	mm, err := GetMachine(m.VMX)
	if err != nil {
		return err
	}
	*m = *mm
	return nil
}

// Start starts the machine.
func (m *Machine) Start() error {
	vmrun("start", m.VMX, "nogui")
	return nil
}

// Suspend suspends the machine and saves its state to disk.
func (m *Machine) Save() error {
	vmrun("suspend", m.VMX, "nogui")
	return nil
}

// Pause pauses the execution of the machine.
func (m *Machine) Pause() error {
	vmrun("pause", m.VMX, "nogui")
	return nil
}

// Stop gracefully stops the machine.
func (m *Machine) Stop() error {
	vmrun("stop", m.VMX, "nogui")
	return nil
}

// Poweroff forcefully stops the machine. State is lost and might corrupt the disk image.
func (m *Machine) Poweroff() error {
	vmrun("stop", m.VMX, "nogui hard")
	return nil
}

// Restart gracefully restarts the machine.
func (m *Machine) Restart() error {
	vmrun("reset", m.VMX, "nogui")
	return nil
}

// Reset forcefully restarts the machine. State is lost and might corrupt the disk image.
func (m *Machine) Reset() error {
	vmrun("reset", m.VMX, "nogui")
	return nil
}

// Get vm name
func (m *Machine) GetName() string {
	return m.Name
}

// Get vm hostname
func (m *Machine) GetHostname() string {
	stdout, _, _ := vmrunOutErr("getGuestIPAddress", m.VMX)
	return strings.TrimSpace(stdout)
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
	return 2375
}

// Get SSH port
func (m *Machine) GetSSHPort() uint {
	return 22
}

// Delete deletes the machine and associated disk images.
func (m *Machine) Delete() error {
	vmrun("deleteVM", m.VMX, "nogui")
	return nil
}

// Modify changes the settings of the machine.
func (m *Machine) Modify() error {
	fmt.Printf("Hot modify not supported")
	return m.Refresh()
}

// AddNATPF adds a NAT port forwarding rule to the n-th NIC with the given name.
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
		return nil, driver.ErrMachineNotExist
	}

	m := &Machine{VMX: vmx, State: driver.Poweroff}

	// VMRUN only tells use if the vm is running or not
	if stdout, _, _ := vmrunOutErr("list"); strings.Contains(stdout, m.VMX) {
		m.State = driver.Running
	}

	// Parse the vmx file
	vmxfile, err := os.Open(vmx)
	if err != nil {
		return m, err
	}
	defer vmxfile.Close()

	vmxscan := bufio.NewScanner(vmxfile)
	for vmxscan.Scan() {
		if vmxtokens := strings.Split(vmxscan.Text(), " = "); len(vmxtokens) > 1 {
			vmxkey := strings.TrimSpace(vmxtokens[0])
			vmxvalue, _ := strconv.Unquote(vmxtokens[1])
			switch vmxkey {
			case "displayName":
				m.Name = vmxvalue
			case "guestOS":
				m.OSType = vmxvalue
			case "memsize":
				m.Memory, _ = strconv.ParseUint(vmxvalue, 10, 0)
			case "numvcpus":
				m.CPUs, _ = strconv.ParseUint(vmxvalue, 10, 0)
			}
		}
	}
	return m, nil
}

// CreateMachine creates a new virtual machine.
func CreateMachine(mc *driver.MachineConfig) (*Machine, error) {
	if err := os.MkdirAll(getBaseFolder(mc), 0755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(getVMX(mc)); err == nil {
		return nil, driver.ErrMachineExist
	}

	// Generate vmx config file from template
	vmxt := template.Must(template.New("vmx").Parse(vmx))
	vmxfile, err := os.Create(getVMX(mc))
	if err != nil {
		return nil, err
	}
	vmxt.Execute(vmxfile, mc)

	// Generate vmdk file
	diskImg := filepath.Join(getBaseFolder(mc), fmt.Sprintf("%s.vmdk", mc.VM))
	if _, err := os.Stat(diskImg); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		if err := vdiskmanager(diskImg, mc.DiskSize); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func getBaseFolder(mc *driver.MachineConfig) string {
	return filepath.Join(mc.Dir, mc.VM)
}
func getVMX(mc *driver.MachineConfig) string {
	return filepath.Join(getBaseFolder(mc), fmt.Sprintf("%s.vmx", mc.VM))
}
