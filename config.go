package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	// keep 3rd-party imports separate from stdlib with an empty line
	flag "github.com/ogier/pflag"
	"github.com/vaughan0/go-ini"
)

// boot2docker config.
var B2D struct {
	// NOTE: separate sections with blank lines so gofmt doesn't change
	// indentation all the time.

	// basic config
	VBM      string // VirtualBox management utility
	SSH      string // SSH client executable
	VM       string // virtual machine name
	Dir      string // boot2docker directory
	ISO      string // boot2docker ISO image path
	DiskSize uint   // VM disk image size (MB)
	Memory   uint   // VM memory size (MB)

	// NAT network: port forwarding
	SSHPort    uint16 // host SSH port (forward to port 22 in VM)
	DockerPort uint16 // host Docker port (forward to port 4243 in VM)

	// host-only network
	HostIP         string
	DHCPIP         string
	NetworkMask    string
	LowerIPAddress string
	UpperIPAddress string
	DHCPEnabled    string
}

func getCfgDir(name string) (string, error) {
	if b2dDir := os.Getenv("BOOT2DOCKER_DIR"); b2dDir != "" {
		return b2dDir, nil
	}

	// *nix
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, name), nil
	}

	// Windows
	for _, env := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"USERPROFILE",
	} {
		if val := os.Getenv(env); val != "" {
			return filepath.Join(val, "boot2docker"), nil
		}
	}
	// Fallback to current working directory as a last resort
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, name), nil
}

// Read configuration from both profile and flags. Flags override profile.
func config() error {
	var err error
	if B2D.Dir, err = getCfgDir(".boot2docker"); err != nil {
		return fmt.Errorf("failed to get current directory: %s", err)
	}

	filename := os.Getenv("BOOT2DOCKER_PROFILE")
	if filename == "" {
		filename = filepath.Join(B2D.Dir, "profile")
	}
	profile, err := getProfile(filename)
	if err != nil && !os.IsNotExist(err) { // undefined/empty profile works
		return err
	}

	if p := os.Getenv("VBOX_INSTALL_PATH"); p != "" && runtime.GOOS == "windows" {
		B2D.VBM = profile.Get("", "vbm", filepath.Join(p, "VBoxManage.exe"))
	} else {
		B2D.VBM = profile.Get("", "vbm", "VBoxManage")
	}

	B2D.SSH = profile.Get("", "ssh", "ssh")
	B2D.VM = profile.Get("", "vm", "boot2docker-vm")
	B2D.ISO = profile.Get("", "iso", filepath.Join(B2D.Dir, "boot2docker.iso"))

	if diskSize, err := strconv.ParseUint(profile.Get("", "disksize", "20000"), 10, 32); err != nil {
		return fmt.Errorf("invalid disk image size: %s", err)
	} else {
		B2D.DiskSize = uint(diskSize)
	}

	if memory, err := strconv.ParseUint(profile.Get("", "memory", "1024"), 10, 32); err != nil {
		return fmt.Errorf("invalid memory size: %s", err)
	} else {
		B2D.Memory = uint(memory)
	}

	if sshPort, err := strconv.ParseUint(profile.Get("", "sshport", "2022"), 10, 16); err != nil {
		return fmt.Errorf("invalid SSH port: %s", err)
	} else {
		B2D.SSHPort = uint16(sshPort)
	}

	if dockerPort, err := strconv.ParseUint(profile.Get("", "dockerport", "4243"), 10, 16); err != nil {
		return fmt.Errorf("invalid DockerPort: %s", err)
	} else {
		B2D.DockerPort = uint16(dockerPort)
	}

	// Host only networking settings
	B2D.HostIP = profile.Get("", "hostiP", "192.168.59.3")
	B2D.DHCPIP = profile.Get("", "dhcpip", "192.168.59.99")
	B2D.NetworkMask = profile.Get("", "netmask", "255.255.255.0")
	B2D.LowerIPAddress = profile.Get("", "lowerip", "192.168.59.103")
	B2D.UpperIPAddress = profile.Get("", "upperip", "192.168.59.254")
	B2D.DHCPEnabled = profile.Get("", "dhcp", "Yes")

	// Commandline flags override profile settings.
	flag.StringVar(&B2D.VBM, "vbm", B2D.VBM, "Path to VirtualBox management utility")
	flag.StringVar(&B2D.SSH, "ssh", B2D.SSH, "Path to SSH client utility")
	flag.StringVarP(&B2D.Dir, "dir", "d", B2D.Dir, "boot2docker config directory")
	flag.StringVar(&B2D.ISO, "iso", B2D.ISO, "Path to boot2docker ISO image")
	flag.UintVarP(&B2D.DiskSize, "disksize", "s", B2D.DiskSize, "boot2docker disk image size (in MB)")
	flag.UintVarP(&B2D.Memory, "memory", "m", B2D.Memory, "Virtual machine memory size (in MB)")
	flag.Var(newUint16Value(B2D.SSHPort, &B2D.SSHPort), "sshport", "Host SSH port (forward to port 22 in VM)")
	flag.Var(newUint16Value(B2D.DockerPort, &B2D.DockerPort), "dockerport", "Host Docker port (forward to port 4243 in VM)")
	flag.StringVar(&B2D.HostIP, "hostip", B2D.HostIP, "VirtualBox host-only network IP address")
	flag.StringVar(&B2D.NetworkMask, "netmask", B2D.NetworkMask, "VirtualBox host-only network mask")
	flag.StringVar(&B2D.DHCPEnabled, "dhcp", B2D.DHCPEnabled, "Enable VirtualBox host-only network DHCP")
	flag.StringVar(&B2D.DHCPIP, "dhcpip", B2D.DHCPIP, "VirtualBox host-only network DHCP server address")
	flag.StringVar(&B2D.LowerIPAddress, "lowerip", B2D.LowerIPAddress, "VirtualBox host-only network DHCP lower bound")
	flag.StringVar(&B2D.UpperIPAddress, "upperip", B2D.UpperIPAddress, "VirtualBox host-only network DHCP upper bound")

	flag.Parse()

	// Name of VM is the second argument.
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}
	return nil
}

// boot2docker configuration profile.
type Profile struct {
	ini.File
}

func getProfile(filename string) (*Profile, error) {
	f, err := ini.LoadFile(filename)
	return &Profile{f}, err
}

func (f *Profile) Get(section, key, fallback string) string {
	if val, ok := f.File.Get(section, key); ok {
		return os.ExpandEnv(val)
	}
	return fallback
}

// The missing flag.Uint16Var value type.
type uint16Value uint16

func newUint16Value(val uint16, p *uint16) *uint16Value {
	*p = val
	return (*uint16Value)(p)
}
func (i *uint16Value) String() string { return fmt.Sprintf("%d", *i) }
func (i *uint16Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 10, 16)
	*i = uint16Value(v)
	return err
}
func (i *uint16Value) Get() interface{} {
	return uint16(*i)
}
