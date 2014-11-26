package vsphere

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/boot2docker/boot2docker-cli/driver"
	"github.com/boot2docker/boot2docker-cli/vsphere/errors"
	flag "github.com/ogier/pflag"
)

type DriverCfg struct {
	Govc          string // Path to govc binary
	VcenterIp     string // vCenter URL
	VcenterUser   string // vCenter User
	VcenterDC     string // target vCenter Datacenter
	VcenterDS     string // target vCenter Datastore
	VcenterNet    string // vCenter VM Network
	VcenterPool   string // target vCenter Resource Pool
	VcenterHostIp string // target vCenter Host Ip
	Cpu           string // CPU number of the virtual machine
}

var (
	verbose bool // Verbose mode (Local copy of B2D.Verbose).
	cfg     DriverCfg
)

const (
	DATASTORE_DIR      = "boot2docker-iso"
	DATASTORE_ISO_NAME = "boot2docker.iso"
	DEFAULT_CPU_NUMBER = 2
)

func init() {
	if err := driver.Register("vsphere", InitFunc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver. Error : %s", err.Error())
		os.Exit(1)
	}
	if err := driver.RegisterConfig("vsphere", ConfigFlags); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver config. Error : %s", err.Error())
		os.Exit(1)
	}
}

// Initialize the Machine.
func InitFunc(mc *driver.MachineConfig) (driver.Machine, error) {
	verbose = mc.Verbose

	m, err := GetMachine(mc)
	if _, ok := err.(*errors.GovcNotFoundError); ok {
		return nil, err
	}

	if err != nil && mc.Init == true {
		return CreateMachine(mc)
	}
	return m, err
}

// Add cmdline params for this driver
func ConfigFlags(B2D *driver.MachineConfig, flags *flag.FlagSet) error {
	flags.StringVar(&cfg.Govc, "govc", "govc", "Path to GOVC Binary")
	flags.StringVar(&cfg.VcenterIp, "vcenter-ip", "", "vCenter URL")
	flags.StringVar(&cfg.VcenterUser, "vcenter-user", "", "vCenter User")
	flags.StringVar(&cfg.VcenterDC, "vcenter-datacenter", "", "vCenter Datacenter")
	flags.StringVar(&cfg.VcenterDS, "vcenter-datastore", "", "vCenter Datastore")
	flags.StringVar(&cfg.VcenterNet, "vcenter-vm-network", "", "vCenter VM Network")
	flags.StringVar(&cfg.VcenterPool, "vcenter-pool", "", "vCenter Target Resource Pool")
	flags.StringVar(&cfg.VcenterHostIp, "vcenter-host-ip", "", "vCenter Target Host IP")

	return nil
}

// GetMachine fetches the machine information from a vCenter
func GetMachine(mc *driver.MachineConfig) (*Machine, error) {
	err := GetDriverCfg(mc)
	if err != nil {
		return nil, err
	}

	if mc.Init == false {
		fmt.Fprintf(os.Stdout, "Connecting to vSphere environment %s...\n", cfg.VcenterIp)
	}

	vcConn := NewVcConn(&cfg)
	err = vcConn.Login()
	if err != nil {
		return nil, err
	}

	stdout, err := vcConn.VmInfo(mc.VM)
	if err != nil {
		return nil, err
	}

	m := &Machine{
		Name:        mc.VM,
		State:       driver.Poweroff,
		SshPubKey:   mc.SSHKey + ".pub",
		VcenterIp:   cfg.VcenterIp,
		VcenterUser: cfg.VcenterUser,
		Datacenter:  cfg.VcenterDC,
		Network:     cfg.VcenterNet,
	}

	ParseVmProperty(stdout, m)

	return m, nil
}

// create a new machine in vsphere includes the following steps:
// 1. create a directory in vsphere datastore to include the B2D ISO;
// 2. uploads the ISO to the corresponding datastore;
// 3. bootup the virtual machine with the ISO mounted;
func CreateMachine(mc *driver.MachineConfig) (*Machine, error) {
	err := GetDriverCfg(mc)
	if err != nil {
		return nil, err
	}

	vcConn := NewVcConn(&cfg)
	err = vcConn.DatastoreMkdir(DATASTORE_DIR)
	if err != nil {
		return nil, err
	}

	err = vcConn.DatastoreUpload(mc.ISO)
	if err != nil {
		return nil, err
	}

	memory := strconv.Itoa(int(mc.Memory))
	isoPath := fmt.Sprintf("%s/%s", DATASTORE_DIR, DATASTORE_ISO_NAME)
	err = vcConn.VmCreate(isoPath, memory, mc.VM)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stdout, "Configuring the virtual machine %s... ", mc.VM)
	diskSize := strconv.Itoa(int(mc.DiskSize))
	err = vcConn.VmDiskCreate(mc.VM, diskSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return nil, err
	}

	err = vcConn.VmAttachNetwork(mc.VM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return nil, err
	}

	fmt.Fprintf(os.Stdout, "ok!\n")
	cpu, err := strconv.ParseUint(cfg.Cpu, 10, 32)
	if err != nil {
		return nil, err
	}

	m := &Machine{
		Name:        mc.VM,
		State:       driver.Poweroff,
		CPUs:        uint(cpu),
		Memory:      mc.Memory,
		VcenterIp:   cfg.VcenterIp,
		VcenterUser: cfg.VcenterUser,
		Datacenter:  cfg.VcenterDC,
		Network:     cfg.VcenterNet,
		SshPubKey:   mc.SSHKey + ".pub",
	}
	return m, nil
}

func ParseVmProperty(stdout string, m *Machine) {
	currentCpu := strings.Trim(strings.Split(strings.Split(stdout, "CPU:")[1], "vCPU")[0], " ")
	if cpus, err := strconv.ParseUint(currentCpu, 10, 32); err == nil {
		m.CPUs = uint(cpus)
	}
	currentMem := strings.Trim(strings.Split(strings.Split(stdout, "Memory:")[1], "MB")[0], " ")
	if mem, err := strconv.ParseUint(currentMem, 10, 32); err == nil {
		m.Memory = uint(mem)
	}
	if strings.Contains(stdout, "poweredOn") {
		m.State = driver.Running
		m.VmIp = strings.Trim(strings.Trim(strings.Split(stdout, "IP address:")[1], " "), "\n")
	}
}

func GetDriverCfg(mc *driver.MachineConfig) error {
	vcenterIp := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterIp"]
	if vcenterIp == nil {
		if cfg.VcenterIp == "" {
			return errors.NewIncompleteVcConfigError("vCenter IP")
		}
	} else {
		cfg.VcenterIp = vcenterIp.(string)
	}
	vcenterUser := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterUser"]
	if vcenterUser == nil {
		if cfg.VcenterUser == "" {
			return errors.NewIncompleteVcConfigError("vCenter User")
		}
	} else {
		cfg.VcenterUser = vcenterUser.(string)
	}
	vcenterDC := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterDatacenter"]
	if vcenterDC == nil {
		if cfg.VcenterDC == "" {
			return errors.NewIncompleteVcConfigError("vCenter Datacenter")
		}
	} else {
		cfg.VcenterDC = vcenterDC.(string)
	}
	vcenterDS := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterDatastore"]
	if vcenterDS == nil {
		if cfg.VcenterDS == "" {
			return errors.NewIncompleteVcConfigError("vCenter Datastore")
		}
	} else {
		cfg.VcenterDS = vcenterDS.(string)
	}
	vcenterNet := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterNetwork"]
	if vcenterNet == nil {
		if cfg.VcenterNet == "" {
			return errors.NewIncompleteVcConfigError("vCenter Network")
		}
	} else {
		cfg.VcenterNet = vcenterNet.(string)
	}
	cpu := mc.DriverCfg["vsphere"].(map[string]interface{})["VmCPU"]
	if cpu == nil {
		if cfg.Cpu == "" {
			cfg.Cpu = strconv.Itoa(DEFAULT_CPU_NUMBER)
		}
	} else {
		cfg.Cpu = strconv.Itoa(int(cpu.(int64)))
	}

	// govc path information are optional as user may want to use the default
	govc := mc.DriverCfg["vsphere"].(map[string]interface{})["Govc"]
	if govc != nil {
		cfg.Govc = govc.(string)
	}

	// vcenter resource pool and host ip are nullable configurations
	pool := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterPool"]
	if pool != nil {
		cfg.VcenterPool = pool.(string)
	}
	hostIp := mc.DriverCfg["vsphere"].(map[string]interface{})["VcenterHostIp"]
	if hostIp != nil {
		cfg.VcenterHostIp = hostIp.(string)
	}
	return nil
}

// Machine information.
type Machine struct {
	Name        string
	State       driver.MachineState
	CPUs        uint
	Memory      uint
	VcenterIp   string // the vcenter the machine belongs to
	VcenterUser string // the vcenter user/admin to own the machine
	Datacenter  string // the datacenter the machine locates
	Network     string // the network the machine is using
	VmIp        string // the Ip address of the machine
	SshPubKey   string // pass SSH here so the vm knows the source of authorized_keys
}

// Refresh reloads the machine information.
func (m *Machine) Refresh() error {
	vcConn := NewVcConn(&cfg)
	stdout, err := vcConn.VmInfo(m.Name)
	if err != nil {
		return err
	}
	ParseVmProperty(stdout, m)
	return nil
}

// Start starts the machine.
// for vSphere driver, the start process includes the following changes
// 1. start the docker virtual machine;
// 2. fetch the ip address from the virtual machine (with open-vmtools);
// 3. upload the ssh key to the virtual machine;
func (m *Machine) Start() error {
	switch m.State {
	case driver.Running:
		msg := fmt.Sprintf("VM %s has already been started", m.Name)
		fmt.Println(msg)
		return nil
	case driver.Poweroff:
		// TODO add transactional or error handling in the following steps
		vcConn := NewVcConn(&cfg)
		err := vcConn.VmPowerOn(m.Name)
		if err != nil {
			return err
		}
		// this step waits for the vm to start and fetch its ip address;
		// this guarantees that the opem-vmtools has started working...
		_, err = vcConn.VmFetchIp(m.Name)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Configuring virtual machine %s... ", m.Name)
		err = vcConn.GuestMkdir("docker", "tcuser", m.Name, "/home/docker/.ssh")
		if err != nil {
			fmt.Fprintf(os.Stdout, "failed!\n")
			return err
		}
		err = vcConn.GuestUpload("docker", "tcuser", m.Name, m.SshPubKey,
			"/home/docker/.ssh/authorized_keys")
		if err != nil {
			fmt.Fprintf(os.Stdout, "failed!\n")
			return err
		}
		fmt.Fprintf(os.Stdout, "ok!\n")
	}
	return nil
}

// Suspend suspends the machine and saves its state to disk.
func (m *Machine) Save() error {
	return driver.ErrNotSupported
}

// Pause pauses the execution of the machine.
func (m *Machine) Pause() error {
	return driver.ErrNotSupported
}

// Currently make stop equivalent to poweroff as there is no shutdown guestOS API
// yet with current open-vmtools and govc
func (m *Machine) Stop() error {
	vcConn := NewVcConn(&cfg)
	err := vcConn.VmPowerOff(m.Name)
	if err != nil {
		return err
	}
	m.State = driver.Poweroff
	return err
}

// Poweroff forcefully stops the machine. State is lost and might corrupt the disk image.
func (m *Machine) Poweroff() error {
	vcConn := NewVcConn(&cfg)
	err := vcConn.VmPowerOff(m.Name)
	if err != nil {
		return err
	}
	m.State = driver.Poweroff
	return err
}

// Restart gracefully restarts the machine.
func (m *Machine) Restart() error {
	switch m.State {
	case driver.Running:
		if err := m.Stop(); err != nil {
			return err
		}
	case driver.Poweroff:
		fmt.Fprintf(os.Stdout, "Machine %s already stopped, starting it... \n", m.Name)
	}
	return m.Start()
}

// Reset forcefully restarts the machine. State is lost and might corrupt the disk image.
func (m *Machine) Reset() error {
	return m.Restart()
}

// Get current name
func (m *Machine) GetName() string {
	return m.Name
}

// Get machine address
func (m *Machine) GetAddr() string {
	return m.VmIp
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
	if m.State == driver.Running {
		msg := fmt.Sprintf("Please poweroff machine %s before delete", m.Name)
		fmt.Println(msg)
		return errors.NewInvalidStateError(m.Name)
	}
	vcConn := NewVcConn(&cfg)
	err := vcConn.VmDestroy(m.Name)
	if err != nil {
		return err
	}
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
