package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	// keep 3rd-party imports separate from stdlib with an empty line
	"github.com/vaughan0/go-ini"
)

// boot2docker config.
var B2D struct {
	VBM        string // VirtualBox management utility
	SSH        string // SSH client executable
	VM         string // virtual machine name
	Dir        string // boot2docker directory
	ISO        string // boot2docker ISO image path
	Disk       string // VM disk image path
	DiskSize   int    // VM disk image size (MB)
	Memory     int    // VM memory size (MB)
	SSHPort    int    // host SSH port (forward to port 22 in VM)
	DockerPort int    // host Docker port (forward to port 4243 in VM)
	HostIP         string // Host only network IP address
	DHCPIP         string // Host only network DHCP address
	NetworkMask    string // Host only network
	LowerIPAddress string // Host only network
	UpperIPAddress string // Host only network
	DHCPEnabled    string // Host only network DHCP endabled
}

func getCfgDir(name string) (string, error) {
	if b2dDir := os.Getenv("BOOT2DOCKER_CFG_DIR"); b2dDir != "" {
		return b2dDir, nil
	}

	// Unix
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, name), nil
	}

	// Windows
	for _, env := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"USERPROFILE", // let's try USERPROFILE only as a very last resort
	} {
		if val := os.Getenv(env); val != "" {
			return filepath.Join(val, "boot2docker"), nil
		}
	}
	// ok, we've tried everything reasonable - now let's go for CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, name), nil
}

// Read configuration.
func config() (err error) {

	if B2D.Dir, err = getCfgDir(".boot2docker"); err != nil {
		return fmt.Errorf("failed to get current directory: %s", err)
	}
	cfgi, err := getConfigfile()

	if vboxPath := os.Getenv("VBOX_INSTALL_PATH") ; vboxPath != "" && runtime.GOOS == "windows" {
		B2D.VBM = cfgi.Get("", "VBM", filepath.Join(vboxPath, "VBoxManage.exe"))
	} else {
		B2D.VBM = cfgi.Get("", "VBM", "VBoxManage")
	}
	B2D.SSH = cfgi.Get("", "BOOT2DOCKER_SSH", "ssh")
	B2D.VM = cfgi.Get("", "VM_NAME", "boot2docker-vm")

	B2D.ISO = cfgi.Get("", "BOOT2DOCKER_ISO", filepath.Join(B2D.Dir, "boot2docker.iso"))
	B2D.Disk = cfgi.Get("", "VM_DISK", filepath.Join(B2D.Dir, "boot2docker.vmdk"))

	if B2D.DiskSize, err = strconv.Atoi(cfgi.Get("", "VM_DISK_SIZE", "20000")); err != nil {
		return fmt.Errorf("invalid VM_DISK_SIZE: %s", err)
	}
	if B2D.DiskSize <= 0 {
		return fmt.Errorf("VM_DISK_SIZE way too small")
	}
	if B2D.Memory, err = strconv.Atoi(cfgi.Get("", "VM_MEM", "1024")); err != nil {
		return fmt.Errorf("invalid VM_MEM: %s", err)
	}
	if B2D.Memory <= 0 {
		return fmt.Errorf("VM_MEM way too small")
	}
	if B2D.SSHPort, err = strconv.Atoi(cfgi.Get("", "SSH_HOST_PORT", "2022")); err != nil {
		return fmt.Errorf("invalid SSH_HOST_PORT: %s", err)
	}
	if B2D.SSHPort <= 0 {
		return fmt.Errorf("invalid SSH_HOST_PORT: must be in the range of 1--65535; got %d", B2D.SSHPort)
	}
	if B2D.DockerPort, err = strconv.Atoi(cfgi.Get("", "DOCKER_PORT", "4243")); err != nil {
		return fmt.Errorf("invalid DOCKER_PORT: %s", err)
	}
	if B2D.DockerPort <= 0 {
		return fmt.Errorf("invalid DOCKER_PORT: must be in the range of 1--65535; got %d", B2D.DockerPort)
	}
	// Host only networking settings
	B2D.HostIP = cfgi.Get("", "HOST_IP", "192.168.59.3")
	B2D.DHCPIP = cfgi.Get("", "DHCP_IP", "192.168.59.99")
	B2D.NetworkMask = cfgi.Get("", "NetworkMask", "255.255.255.0")
	B2D.LowerIPAddress = cfgi.Get("", "LowerIPAddress", "192.168.59.103")
	B2D.UpperIPAddress = cfgi.Get("", "UpperIPAddress", "192.168.59.254")
	B2D.DHCPEnabled = cfgi.Get("", "DHCP_Enabled", "Yes")

	// TODO maybe allow flags to override ENV vars?
	flag.Parse()
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}
	return
}

type cfgImport struct {
	cf ini.File
}

func (f cfgImport) Get(section, key, defaultstr string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value, ok := f.cf.Get(section, key); ok {
		return os.ExpandEnv(value)
	}
	return defaultstr
}

var readConfigfile = func(filename string) (string, error) {
	value, err := ioutil.ReadFile(filename)
	return string(value), err
}

var getConfigfile = func() (cfgImport, error) {
	var cfg cfgImport
	filename := os.Getenv("BOOT2DOCKER_PROFILE")
	if filename == "" {
		filename = filepath.Join(B2D.Dir, "profile")
	}

	cfgStr, err := readConfigfile(filename)
	if err != nil {
		return cfg, err
	}

	cfgini, err := ini.Load(strings.NewReader(cfgStr))
	if err != nil {
		log.Fatalf("Failed to parse %s: %s", filename, err)
		return cfg, err
	}
	cfg = cfgImport{cf: cfgini}

	return cfg, err
}
