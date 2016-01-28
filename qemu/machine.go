package qemu

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/boot2docker/boot2docker-cli/driver"
)

func init() {
	driver.Register("qemu", InitFunc)
}

// Initialize the Machine.
func InitFunc(i *driver.MachineConfig) (driver.Machine, error) {
	fmt.Printf("Init qemu %s\n", i.VM)
	
	// qemu-system-x86_64 -net nic -net user -m 2048M -boot d -cdrom boot2docker.iso persist.raw
	
	startCmd := []string{
	    "-curses",
        "-net", "nic",
        "-net", "user",
        "-m", fmt.Sprintf("%dM", i.Memory), 
        "-boot", "d", 
        "-cdrom", i.ISO,
//        "persist.raw",
	}

	if  err := cmdInteractive("qemu-system-x86_64", startCmd...); err != nil {
		fmt.Printf("ERROR: %s", err)
		return nil, err
	}

	
	return &Machine{Name: i.VM, State: driver.Poweroff}, nil
}

// Duplicated from ../utils.go
// Convenient function to exec a command.
func cmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
//	if B2D.Verbose {
//		cmd.Stderr = os.Stderr
//		log.Printf("executing: %v %v", name, strings.Join(args, " "))
//	}

	b, err := cmd.Output()
	return string(b), err
}
func cmdInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
//	if B2D.Verbose {
//		logf("executing: %v %v", name, strings.Join(args, " "))
//	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}


// Machine information.
type Machine struct {
	Name       string
	UUID       string
	State      driver.MachineState
	CPUs       uint
	Memory     uint // main memory (in MB)
	VRAM       uint // video memory (in MB)
	CfgFile    string
	BaseFolder string
	OSType     string
	BootOrder  []string // max 4 slots, each in {none|floppy|dvd|disk|net}
	DockerPort uint
	SSHPort    uint
	SerialFile string
}

// Refresh reloads the machine information.
func (m *Machine) Refresh() error {
	fmt.Printf("Refresh %s: %s\n", m.Name, m.State)
	return nil
}

// Start starts the machine.
func (m *Machine) Start() error {
	m.State = driver.Running
	fmt.Printf("Start %s: %s\n", m.Name, m.State)
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
	fmt.Printf("Stop %s: %s\n", m.Name, m.State)
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

// Get current state
func (m *Machine) GetState() driver.MachineState {
	return m.State
}

// Get serial file
func (m *Machine) GetSerialFile() string {
	return m.SerialFile
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
