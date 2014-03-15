package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"

	flag "github.com/ogier/pflag"
	vbx "github.com/riobard/go-virtualbox"
)

// boot2docker config.
var B2D struct {
	// NOTE: separate sections with blank lines so gofmt doesn't change
	// indentation all the time.

	// basic config
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
	HostIP         net.IP
	DHCPIP         net.IP
	NetworkMask    net.IPMask
	LowerIPAddress net.IP
	UpperIPAddress net.IP
	DHCPEnabled    bool
}

// General flags.
var (
	verbose = new(bool)   // verbose mode
	vbm     = new(string) // path to VBoxManage utility
)

var (
	reFlagLine = regexp.MustCompile(`(\w+)=(.+)`)
)

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
	dir, err := getCfgDir(".boot2docker")
	if err != nil {
		return fmt.Errorf("failed to get boot2docker directory: %s", err)
	}

	filename := os.Getenv("BOOT2DOCKER_PROFILE")
	if filename == "" {
		filename = filepath.Join(dir, "profile")
	}

	profileArgs, err := readProfile(filename)
	if err != nil && !os.IsNotExist(err) { // undefined/empty profile works
		return err
	}

	if p := os.Getenv("VBOX_INSTALL_PATH"); p != "" && runtime.GOOS == "windows" {
		flag.StringVar(vbm, "vbm", filepath.Join(p, "VBoxManage.exe"), "path to VBoxManage utility")
	} else {
		flag.StringVar(vbm, "vbm", "VBoxManage", "path to VirtualBox management utility.")
	}
	flag.BoolVarP(verbose, "verbose", "v", false, "display verbose command invocations.")
	flag.StringVar(&B2D.SSH, "ssh", "ssh", "path to SSH client utility.")
	flag.UintVarP(&B2D.DiskSize, "disksize", "s", 20000, "boot2docker disk image size (in MB).")
	flag.UintVarP(&B2D.Memory, "memory", "m", 1024, "virtual machine memory size (in MB).")
	flag.Var(newUint16Value(2022, &B2D.SSHPort), "sshport", "host SSH port (forward to port 22 in VM).")
	flag.Var(newUint16Value(4243, &B2D.DockerPort), "dockerport", "host Docker port (forward to port 4243 in VM).")
	flag.Var(newIPValue(net.ParseIP("192.168.59.3"), &B2D.HostIP), "hostip", "VirtualBox host-only network IP address.")
	flag.Var(newIPMaskValue(vbx.ParseIPv4Mask("255.255.255.0"), &B2D.NetworkMask), "netmask", "VirtualBox host-only network mask.")
	flag.BoolVar(&B2D.DHCPEnabled, "dhcp", true, "enable VirtualBox host-only network DHCP.")
	flag.Var(newIPValue(net.ParseIP("192.168.59.99"), &B2D.DHCPIP), "dhcpip", "VirtualBox host-only network DHCP server address.")
	flag.Var(newIPValue(net.ParseIP("192.168.59.103"), &B2D.LowerIPAddress), "lowerip", "VirtualBox host-only network DHCP lower bound.")
	flag.Var(newIPValue(net.ParseIP("192.168.59.254"), &B2D.UpperIPAddress), "upperip", "VirtualBox host-only network DHCP upper bound.")
	flag.StringVar(&B2D.VM, "vm", "boot2docker-vm", "virtual machine name.")

	// The following options need special handling after parsing.
	flag.StringVarP(&B2D.Dir, "dir", "d", "", "boot2docker config directory.")
	flag.StringVar(&B2D.ISO, "iso", "", "path to boot2docker ISO image.")

	osArgs := os.Args // save original os.Args
	// Insert profile args before command-line args so that command-line overrides profile.
	os.Args = append([]string{os.Args[0]}, append(profileArgs, os.Args[1:]...)...)
	flag.Parse()
	os.Args = osArgs // restore original os.Args

	if B2D.Dir == "" {
		B2D.Dir = dir
	}

	if B2D.ISO == "" {
		B2D.ISO = filepath.Join(B2D.Dir, "boot2docker.iso")
	}

	// Name of VM is the second argument. Override the value set in flag.
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}
	return nil
}

// Read boot2docker configuration profile into string slice. Expanding
// $ENVVARS in the values field.
func readProfile(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	args := []string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		res := reFlagLine.FindStringSubmatch(string(s.Text()))
		if res == nil {
			continue
		}
		args = append(args, fmt.Sprintf("--%v=%v", res[1], os.ExpandEnv(res[2])))
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return args, nil
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

type ipValue net.IP

func newIPValue(val net.IP, p *net.IP) *ipValue {
	*p = val
	return (*ipValue)(p)
}

func (i *ipValue) String() string { return net.IP(*i).String() }
func (i *ipValue) Set(s string) error {
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("failed to parse IP: %q", s)
	}
	*i = ipValue(ip)
	return nil
}
func (i *ipValue) Get() interface{} {
	return net.IP(*i)
}

type ipMaskValue net.IPMask

func newIPMaskValue(val net.IPMask, p *net.IPMask) *ipMaskValue {
	*p = val
	return (*ipMaskValue)(p)
}

func (i *ipMaskValue) String() string { return net.IP(*i).String() }
func (i *ipMaskValue) Set(s string) error {
	ip := vbx.ParseIPv4Mask(s)
	if ip == nil {
		return fmt.Errorf("failed to parse IP mask: %q", s)
	}
	*i = ipMaskValue(ip)
	return nil
}
func (i *ipMaskValue) Get() interface{} {
	return net.IPMask(*i)
}
