package virtualbox

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/boot2docker/boot2docker-cli/driver"
	flag "github.com/ogier/pflag"
)

type Flag int

// Flag names in lowercases to be consistent with VBoxManage options.
const (
	F_acpi Flag = 1 << iota
	F_ioapic
	F_rtcuseutc
	F_cpuhotplug
	F_pae
	F_longmode
	F_synthcpu
	F_hpet
	F_hwvirtex
	F_triplefaultreset
	F_nestedpaging
	F_largepages
	F_vtxvpid
	F_vtxux
	F_accelerate3d
)

type DriverCfg struct {
	VBM  string // Path to VBoxManage utility.
	VMDK string // base VMDK to use as persistent disk.

	shares shareSlice

	// see also func ConfigFlags later in this file
}

var shareDefault string // set in ConfigFlags - this is what gets filled in for "shares" if it's empty

var (
	verbose bool // Verbose mode (Local copy of B2D.Verbose).
	cfg     DriverCfg
)

func init() {
	if err := driver.Register("virtualbox", InitFunc); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver. Error : %s", err.Error())
		os.Exit(1)
	}
	if err := driver.RegisterConfig("virtualbox", ConfigFlags); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize driver config. Error : %s", err.Error())
		os.Exit(1)
	}
}

// Initialize the Machine.
func InitFunc(mc *driver.MachineConfig) (driver.Machine, error) {
	verbose = mc.Verbose

	m, err := GetMachine(mc.VM)
	if err != nil && mc.Init {
		return CreateMachine(mc)
	}
	return m, err
}

type shareSlice map[string]string

const shareSliceSep = "="

func (s shareSlice) String() string {
	var ret []string
	for name, dir := range s {
		ret = append(ret, fmt.Sprintf("%s%s%s", dir, shareSliceSep, name))
	}
	return fmt.Sprintf("[%s]", strings.Join(ret, " "))
}

func (s *shareSlice) Set(shareDir string) error {
	var shareName string
	if i := strings.Index(shareDir, shareSliceSep); i >= 0 {
		shareName = shareDir[i+1:]
		shareDir = shareDir[:i]
	}
	if shareName == "" {
		// parts of the VBox internal code are buggy with share names that start with "/"
		shareName = strings.TrimLeft(shareDir, "/")
		// TODO do some basic Windows -> MSYS path conversion
		// ie, s!^([a-z]+):[/\\]+!\1/!; s!\\!/!g
	}
	if *s == nil {
		*s = shareSlice{}
	}
	(*s)[shareName] = shareDir
	return nil
}

// Add cmdline params for this driver
func ConfigFlags(B2D *driver.MachineConfig, flags *flag.FlagSet) error {
	//B2D.DriverCfg["virtualbox"] = cfg

	flags.StringVar(&cfg.VMDK, "basevmdk", "", "Path to VMDK to use as base for persistent partition")

	cfg.VBM = "VBoxManage"
	if runtime.GOOS == "windows" {
		p := "C:\\Program Files\\Oracle\\VirtualBox"
		if t := os.Getenv("VBOX_INSTALL_PATH"); t != "" {
			p = t
		} else if t = os.Getenv("VBOX_MSI_INSTALL_PATH"); t != "" {
			p = t
		}
		cfg.VBM = filepath.Join(p, "VBoxManage.exe")
	}
	flags.StringVar(&cfg.VBM, "vbm", cfg.VBM, "path to VirtualBox management utility.")

	// TODO once boot2docker improves, replace this all with homeDir() from config.go so we only share the current user's HOME by default
	shareDefault = "disable"
	switch runtime.GOOS {
	case "darwin":
		shareDefault = "/Users" + shareSliceSep + "Users"
	case "windows":
		shareDefault = "C:\\Users" + shareSliceSep + "c/Users"
	}

	var defaultText string
	if shareDefault != "disable" {
		defaultText = "(defaults to '" + shareDefault + "' if no shares are specified; use 'disable' to explicitly prevent any shares from being created) "
	}
	flags.Var(&cfg.shares, "vbox-share", fmt.Sprintf("%sList of directories to share during 'up|start|boot' via VirtualBox Guest Additions, with optional labels", defaultText))

	return nil
}

// Convert bool to "on"/"off"
func bool2string(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// Test if flag is set. Return "on" or "off".
func (f Flag) Get(o Flag) string {
	return bool2string(f&o == o)
}

// Machine information.
type Machine struct {
	Name       string
	UUID       string
	Iso        string
	State      driver.MachineState
	CPUs       uint
	Memory     uint // main memory (in MB)
	VRAM       uint // video memory (in MB)
	CfgFile    string
	BaseFolder string
	OSType     string
	Flag       Flag
	BootOrder  []string // max 4 slots, each in {none|floppy|dvd|disk|net}
	DockerPort uint
	SSHPort    uint
	SerialFile string
}

// Refresh reloads the machine information.
func (m *Machine) Refresh() error {
	id := m.Name
	if id == "" {
		id = m.UUID
	}
	mm, err := GetMachine(id)
	if err != nil {
		return err
	}
	*m = *mm
	return nil
}

// Start starts the machine.
func (m *Machine) Start() error {
	switch m.State {
	case driver.Paused:
		return vbm("controlvm", m.Name, "resume")
	case driver.Poweroff, driver.Aborted:
		if err := m.setUpShares(); err != nil {
			return err
		}
		fallthrough
	case driver.Saved:
		return vbm("startvm", m.Name, "--type", "headless")
	}
	if err := m.Refresh(); err == nil {
		if m.State != driver.Running {
			return fmt.Errorf("Failed to start", m.Name)
		}
	}
	return nil
}

// Suspend suspends the machine and saves its state to disk.
func (m *Machine) Save() error {
	switch m.State {
	case driver.Paused:
		if err := m.Start(); err != nil {
			return err
		}
	case driver.Poweroff, driver.Aborted, driver.Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "savestate")
}

// Pause pauses the execution of the machine.
func (m *Machine) Pause() error {
	switch m.State {
	case driver.Paused, driver.Poweroff, driver.Aborted, driver.Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "pause")
}

// Stop gracefully stops the machine.
func (m *Machine) Stop() error {
	switch m.State {
	case driver.Poweroff, driver.Aborted, driver.Saved:
		return nil
	case driver.Paused:
		if err := m.Start(); err != nil {
			return err
		}
	}

	// busy wait until the machine is stopped
	for i := 0; i < 10; i++ {
		if err := vbm("controlvm", m.Name, "acpipowerbutton"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		if err := m.Refresh(); err != nil {
			return err
		}
		if m.State == driver.Poweroff {
			return nil
		}
	}

	return fmt.Errorf("timed out waiting for VM to stop")
}

// Poweroff forcefully stops the machine. State is lost and might corrupt the disk image.
func (m *Machine) Poweroff() error {
	switch m.State {
	case driver.Poweroff, driver.Aborted, driver.Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "poweroff")
}

// Restart gracefully restarts the machine.
func (m *Machine) Restart() error {
	switch m.State {
	case driver.Paused, driver.Saved:
		if err := m.Start(); err != nil {
			return err
		}
	}
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

// Reset forcefully restarts the machine. State is lost and might corrupt the disk image.
func (m *Machine) Reset() error {
	switch m.State {
	case driver.Paused, driver.Saved:
		if err := m.Start(); err != nil {
			return err
		}
	}
	return vbm("controlvm", m.Name, "reset")
}

// Delete deletes the machine and associated disk images.
func (m *Machine) Delete() error {
	if err := m.Poweroff(); err != nil {
		return err
	}
	return vbm("unregistervm", m.Name, "--delete")
}

// Get current state
func (m *Machine) GetName() string {
	return m.Name
}

// Get machine address
func (m *Machine) GetAddr() string {
	return "localhost"
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

// GetMachine finds a machine by its name or UUID.
func GetMachine(id string) (*Machine, error) {
	stdout, stderr, err := vbmOutErr("showvminfo", id, "--machinereadable")
	if err != nil {
		if reMachineNotFound.FindString(stderr) != "" {
			return nil, driver.ErrMachineNotExist
		}
		return nil, err
	}
	s := bufio.NewScanner(strings.NewReader(stdout))
	m := &Machine{}
	for s.Scan() {
		res := reVMInfoLine.FindStringSubmatch(s.Text())
		if res == nil {
			continue
		}
		key := res[1]
		if key == "" {
			key = res[2]
		}
		val := res[3]
		if val == "" {
			val = res[4]
		}

		switch key {
		case "name":
			m.Name = val
		case "UUID":
			m.UUID = val
		case "SATA-0-0":
			m.Iso = val
		case "VMState":
			m.State = driver.MachineState(val)
		case "memory":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.Memory = uint(n)
		case "cpus":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.CPUs = uint(n)
		case "vram":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.VRAM = uint(n)
		case "CfgFile":
			m.CfgFile = val
			m.BaseFolder = filepath.Dir(val)
		case "uartmode1":
			// uartmode1="server,/home/sven/.boot2docker/boot2docker-vm.sock"
			vals := strings.Split(val, ",")
			if len(vals) >= 2 {
				m.SerialFile = vals[1]
			}
		default:
			if strings.HasPrefix(key, "Forwarding(") {
				// "Forwarding(\d*)" are ordered by the name inside the val, not fixed order.
				// Forwarding(0)="docker,tcp,127.0.0.1,5555,,"
				// Forwarding(1)="ssh,tcp,127.0.0.1,2222,,22"
				vals := strings.Split(val, ",")
				n, err := strconv.ParseUint(vals[3], 10, 32)
				if err != nil {
					return nil, err
				}
				switch vals[0] {
				case "docker":
					m.DockerPort = uint(n)
				case "ssh":
					m.SSHPort = uint(n)
				}
			}
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// ListMachines lists all registered machines.
func ListMachines() ([]string, error) {
	out, err := vbmOut("list", "vms")
	if err != nil {
		return nil, err
	}
	ms := []string{}
	s := bufio.NewScanner(strings.NewReader(out))
	for s.Scan() {
		res := reVMNameUUID.FindStringSubmatch(s.Text())
		if res == nil {
			continue
		}
		ms = append(ms, res[1])
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ms, nil
}

// CreateMachine creates a new machine. If basefolder is empty, use default.
func CreateMachine(mc *driver.MachineConfig) (*Machine, error) {
	if mc.VM == "" {
		return nil, fmt.Errorf("machine name is empty")
	}

	// Check if a machine with the given name already exists.
	machineNames, err := ListMachines()
	if err != nil {
		return nil, err
	}
	for _, m := range machineNames {
		if m == mc.VM {
			return nil, driver.ErrMachineExist
		}
	}

	// Create and register the machine.
	args := []string{"createvm", "--name", mc.VM, "--register"}
	if err := vbm(args...); err != nil {
		return nil, err
	}

	m, err := GetMachine(mc.VM)
	if err != nil {
		return nil, err
	}

	// Configure VM for Boot2docker
	SetExtra(mc.VM, "VBoxInternal/CPUM/EnableHVP", "1")
	m.OSType = "Linux26_64"
	m.CPUs = uint(runtime.NumCPU())
	if m.CPUs > 32 {
		m.CPUs = 32
	}
	m.Memory = mc.Memory
	m.SerialFile = mc.SerialFile

	m.Flag |= F_pae
	m.Flag |= F_longmode // important: use x86-64 processor
	m.Flag |= F_rtcuseutc
	m.Flag |= F_acpi
	m.Flag |= F_ioapic
	m.Flag |= F_hpet
	m.Flag |= F_hwvirtex
	m.Flag |= F_vtxvpid
	m.Flag |= F_largepages
	m.Flag |= F_nestedpaging

	// Set VM boot order
	m.BootOrder = []string{"dvd"}
	if err := m.Modify(); err != nil {
		return m, err
	}

	// Set NIC #1 to use NAT
	m.SetNIC(1, driver.NIC{Network: driver.NICNetNAT, Hardware: driver.VirtIO})
	pfRules := map[string]driver.PFRule{
		"ssh": {Proto: driver.PFTCP, HostIP: net.ParseIP("127.0.0.1"), HostPort: mc.SSHPort, GuestPort: driver.SSHPort},
	}
	if mc.DockerPort > 0 {
		pfRules["docker"] = driver.PFRule{Proto: driver.PFTCP, HostIP: net.ParseIP("127.0.0.1"), HostPort: mc.DockerPort, GuestPort: driver.DockerPort}
	}

	for name, rule := range pfRules {
		if err := m.AddNATPF(1, name, rule); err != nil {
			return m, err
		}
	}

	hostIFName, err := getHostOnlyNetworkInterface(mc)
	if err != nil {
		return m, err
	}

	// Set NIC #2 to use host-only
	if err := m.SetNIC(2, driver.NIC{Network: driver.NICNetHostonly, Hardware: driver.VirtIO, HostonlyAdapter: hostIFName}); err != nil {
		return m, err
	}

	// Set VM storage
	if err := m.AddStorageCtl("SATA", driver.StorageController{SysBus: driver.SysBusSATA, HostIOCache: true, Bootable: true, Ports: 4}); err != nil {
		return m, err
	}

	// Attach ISO image
	if err := m.AttachStorage("SATA", driver.StorageMedium{Port: 0, Device: 0, DriveType: driver.DriveDVD, Medium: mc.ISO}); err != nil {
		return m, err
	}

	diskImg := filepath.Join(m.BaseFolder, fmt.Sprintf("%s.vmdk", mc.VM))
	if _, err := os.Stat(diskImg); err != nil {
		if !os.IsNotExist(err) {
			return m, err
		}

		if cfg.VMDK != "" {
			if err := copyDiskImage(diskImg, cfg.VMDK); err != nil {
				return m, err
			}
		} else {
			magicString := "boot2docker, please format-me"

			buf := new(bytes.Buffer)
			tw := tar.NewWriter(buf)

			// magicString first so the automount script knows to format the disk
			file := &tar.Header{Name: magicString, Size: int64(len(magicString))}
			if err := tw.WriteHeader(file); err != nil {
				return m, err
			}
			if _, err := tw.Write([]byte(magicString)); err != nil {
				return m, err
			}
			// .ssh/key.pub => authorized_keys
			file = &tar.Header{Name: ".ssh", Typeflag: tar.TypeDir, Mode: 0700}
			if err := tw.WriteHeader(file); err != nil {
				return m, err
			}
			pubKey, err := ioutil.ReadFile(mc.SSHKey + ".pub")
			if err != nil {
				return m, err
			}
			file = &tar.Header{Name: ".ssh/authorized_keys", Size: int64(len(pubKey)), Mode: 0644}
			if err := tw.WriteHeader(file); err != nil {
				return m, err
			}
			if _, err := tw.Write([]byte(pubKey)); err != nil {
				return m, err
			}
			file = &tar.Header{Name: ".ssh/authorized_keys2", Size: int64(len(pubKey)), Mode: 0644}
			if err := tw.WriteHeader(file); err != nil {
				return m, err
			}
			if _, err := tw.Write([]byte(pubKey)); err != nil {
				return m, err
			}
			if err := tw.Close(); err != nil {
				return m, err
			}

			if err := makeDiskImage(diskImg, mc.DiskSize, buf.Bytes()); err != nil {
				return m, err
			}
			if verbose {
				fmt.Println("Initializing disk with ssh keys")
				fmt.Printf("WRITING: %s\n-----\n", buf)
			}
		}
	}

	if err := m.AttachStorage("SATA", driver.StorageMedium{Port: 1, Device: 0, DriveType: driver.DriveHDD, Medium: diskImg}); err != nil {
		return m, err
	}

	return m, nil
}

func (m *Machine) setUpShares() error {
	// let VBoxService do nice magic automounting (when it's used)
	if err := vbm("guestproperty", "set", m.Name, "/VirtualBox/GuestAdd/SharedFolders/MountPrefix", "/"); err != nil {
		return err
	}
	if err := vbm("guestproperty", "set", m.Name, "/VirtualBox/GuestAdd/SharedFolders/MountDir", "/"); err != nil {
		return err
	}

	// set up some shared folders as appropriate
	if len(cfg.shares) == 0 {
		cfg.shares.Set(shareDefault)
	}
	for shareName, shareDir := range cfg.shares {
		if shareDir == "disable" {
			continue
		}
		if _, err := os.Stat(shareDir); err != nil {
			return err
		}

		// woo, shareDir exists!  let's carry on!
		if err := vbm("sharedfolder", "add", m.Name, "--name", shareName, "--hostpath", shareDir, "--automount"); err != nil {
			return err
		}

		// enable symlinks
		if err := vbm("setextradata", m.Name, "VBoxInternal2/SharedFoldersEnableSymlinksCreate/"+shareName, "1"); err != nil {
			return err
		}
	}
	return nil
}

// Modify changes the settings of the machine.
func (m *Machine) Modify() error {
	args := []string{"modifyvm", m.Name,
		"--firmware", "bios",
		"--bioslogofadein", "off",
		"--bioslogofadeout", "off",
		"--natdnshostresolver1", "on",
		"--bioslogodisplaytime", "0",
		"--biosbootmenu", "disabled",

		"--ostype", m.OSType,
		"--cpus", fmt.Sprintf("%d", m.CPUs),
		"--memory", fmt.Sprintf("%d", m.Memory),
		"--vram", fmt.Sprintf("%d", m.VRAM),

		"--acpi", m.Flag.Get(F_acpi),
		"--ioapic", m.Flag.Get(F_ioapic),
		"--rtcuseutc", m.Flag.Get(F_rtcuseutc),
		"--cpuhotplug", m.Flag.Get(F_cpuhotplug),
		"--pae", m.Flag.Get(F_pae),
		"--longmode", m.Flag.Get(F_longmode),
		"--synthcpu", m.Flag.Get(F_synthcpu),
		"--hpet", m.Flag.Get(F_hpet),
		"--hwvirtex", m.Flag.Get(F_hwvirtex),
		"--triplefaultreset", m.Flag.Get(F_triplefaultreset),
		"--nestedpaging", m.Flag.Get(F_nestedpaging),
		"--largepages", m.Flag.Get(F_largepages),
		"--vtxvpid", m.Flag.Get(F_vtxvpid),
		"--vtxux", m.Flag.Get(F_vtxux),
		"--accelerate3d", m.Flag.Get(F_accelerate3d),
	}

	//if runtime.GOOS != "windows" {
	args = append(args,
		"--uart1", "0x3F8", "4",
		"--uartmode1", "server", m.SerialFile,
	)
	//}

	for i, dev := range m.BootOrder {
		if i > 3 {
			break // Only four slots `--boot{1,2,3,4}`. Ignore the rest.
		}
		args = append(args, fmt.Sprintf("--boot%d", i+1), dev)
	}
	if err := vbm(args...); err != nil {
		return err
	}
	return m.Refresh()
}

// AddNATPF adds a NAT port forarding rule to the n-th NIC with the given name.
func (m *Machine) AddNATPF(n int, name string, rule driver.PFRule) error {
	return vbm("controlvm", m.Name, fmt.Sprintf("natpf%d", n),
		fmt.Sprintf("%s,%s", name, rule.Format()))
}

// DelNATPF deletes the NAT port forwarding rule with the given name from the n-th NIC.
func (m *Machine) DelNATPF(n int, name string) error {
	return vbm("controlvm", m.Name, fmt.Sprintf("natpf%d", n), "delete", name)
}

// SetNIC set the n-th NIC.
func (m *Machine) SetNIC(n int, nic driver.NIC) error {
	args := []string{"modifyvm", m.Name,
		fmt.Sprintf("--nic%d", n), string(nic.Network),
		fmt.Sprintf("--nictype%d", n), string(nic.Hardware),
		fmt.Sprintf("--cableconnected%d", n), "on",
	}

	if nic.Network == "hostonly" {
		args = append(args, fmt.Sprintf("--hostonlyadapter%d", n), nic.HostonlyAdapter)
	}
	return vbm(args...)
}

// AddStorageCtl adds a storage controller with the given name.
func (m *Machine) AddStorageCtl(name string, ctl driver.StorageController) error {
	args := []string{"storagectl", m.Name, "--name", name}
	if ctl.SysBus != "" {
		args = append(args, "--add", string(ctl.SysBus))
	}
	if ctl.Ports > 0 {
		args = append(args, "--portcount", fmt.Sprintf("%d", ctl.Ports))
	}
	if ctl.Chipset != "" {
		args = append(args, "--controller", string(ctl.Chipset))
	}
	args = append(args, "--hostiocache", bool2string(ctl.HostIOCache))
	args = append(args, "--bootable", bool2string(ctl.Bootable))
	return vbm(args...)
}

// DelStorageCtl deletes the storage controller with the given name.
func (m *Machine) DelStorageCtl(name string) error {
	return vbm("storagectl", m.Name, "--name", name, "--remove")
}

// AttachStorage attaches a storage medium to the named storage controller.
func (m *Machine) AttachStorage(ctlName string, medium driver.StorageMedium) error {
	return vbm("storageattach", m.Name, "--storagectl", ctlName,
		"--port", fmt.Sprintf("%d", medium.Port),
		"--device", fmt.Sprintf("%d", medium.Device),
		"--type", string(medium.DriveType),
		"--medium", medium.Medium,
	)
}
