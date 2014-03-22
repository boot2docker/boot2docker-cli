package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	flag "github.com/ogier/pflag"
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
	HostIP      net.IP
	DHCPIP      net.IP
	NetMask     net.IPMask
	LowerIP     net.IP
	UpperIP     net.IP
	DHCPEnabled bool
}

// General flags.
var (
	verbose = new(bool)   // verbose mode
	vbm     = new(string) // path to VBoxManage utility
)

var (
	reFlagLine = regexp.MustCompile(`(\w+)\s*=\s*(.+)`)
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
	flag.Uint16Var(&B2D.SSHPort, "sshport", 2022, "host SSH port (forward to port 22 in VM).")
	flag.Uint16Var(&B2D.DockerPort, "dockerport", 4243, "host Docker port (forward to port 4243 in VM).")
	flag.IPVar(&B2D.HostIP, "hostip", net.ParseIP("192.168.59.3"), "VirtualBox host-only network IP address.")
	flag.IPMaskVar(&B2D.NetMask, "netmask", flag.ParseIPv4Mask("255.255.255.0"), "VirtualBox host-only network mask.")
	flag.BoolVar(&B2D.DHCPEnabled, "dhcp", true, "enable VirtualBox host-only network DHCP.")
	flag.IPVar(&B2D.DHCPIP, "dhcpip", net.ParseIP("192.168.59.99"), "VirtualBox host-only network DHCP server address.")
	flag.IPVar(&B2D.LowerIP, "lowerip", net.ParseIP("192.168.59.103"), "VirtualBox host-only network DHCP lower bound.")
	flag.IPVar(&B2D.UpperIP, "upperip", net.ParseIP("192.168.59.254"), "VirtualBox host-only network DHCP upper bound.")
	flag.StringVar(&B2D.VM, "vm", "boot2docker-vm", "virtual machine name.")

	flag.StringVarP(&B2D.Dir, "dir", "d", dir, "boot2docker config directory.")
	flag.StringVar(&B2D.ISO, "iso", filepath.Join(dir, "boot2docker.iso"), "path to boot2docker ISO image.")

	osArgs := os.Args // save original os.Args
	// Insert profile args before command-line args so that command-line overrides profile.
	os.Args = append([]string{os.Args[0]}, append(profileArgs, os.Args[1:]...)...)
	flag.Parse()
	os.Args = osArgs // restore original os.Args

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
	ln := 0
	for s.Scan() {
		ln++
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			// Ignore comment lines starting with # or ;
			continue
		}
		res := reFlagLine.FindStringSubmatch(line)
		if res == nil {
			return nil, fmt.Errorf("failed to parse profile line %d: %q", ln, line)
		}
		args = append(args, fmt.Sprintf("--%v=%v", res[1], os.ExpandEnv(res[2])))
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return args, nil
}
