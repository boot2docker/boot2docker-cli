package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	toml "github.com/BurntSushi/toml"
	"github.com/boot2docker/boot2docker-cli/driver"
	flag "github.com/ogier/pflag"
)

var (
	// Pattern to parse a key=value line in config profile.
	reFlagLine = regexp.MustCompile(`^\s*(\w+)\s*=\s*([^#;]+)`)
	B2D        = driver.MachineConfig{}
)

func homeDir() (string, error) {
	dir := ""

	// *nix and MSYS Windows
	if dir = os.Getenv("HOME"); dir == "" {
		// Windows (if not running under MSYS)
		dir = os.Getenv("USERPROFILE")
	}
	if _, err := os.Stat(dir); err != nil {
		return "", err
	}

	return dir, nil
}

func cfgDir(name string) (string, error) {
	if name == ".boot2docker" {
		if b2dDir := os.Getenv("BOOT2DOCKER_DIR"); b2dDir != "" {
			return b2dDir, nil
		}
	}

	dir, err := homeDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func cfgFilename(dir string) string {
	filename := os.Getenv("BOOT2DOCKER_PROFILE")
	if filename == "" {
		filename = filepath.Join(dir, "profile")
	}
	return filename
}

// Write configuration set by the combination of profile and flags
//    Should result in a format that can be piped into a profile file
func printConfig() string {
	var buf bytes.Buffer
	e := toml.NewEncoder(&buf)
	err := e.Encode(B2D)
	if err != nil {
		return ""
	}
	return buf.String()
}

// Read configuration from both profile and flags. Flags override profile.
func config() (*flag.FlagSet, error) {
	dir, err := cfgDir(".boot2docker")
	if err != nil {
		return nil, fmt.Errorf("failed to get boot2docker directory: %s", err)
	}

	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.Usage = func() { usageLong(flags) }

	driver.ConfigFlags(&B2D, flags)

	// Add the generic flags

	flags.StringVar(&B2D.VM, "vm", "boot2docker-vm", "virtual machine name.")
	// removed for now, requires re-parsing a new config file which is too messy
	//flags.StringVarP(&B2D.Dir, "dir", "d", dir, "boot2docker config directory.")
	B2D.Dir = dir
	flags.StringVar(&B2D.ISO, "iso", filepath.Join(dir, "boot2docker.iso"), "path to boot2docker ISO image.")

	// Sven disabled this, as it is broken - if I user with a fresh computer downloads
	// just the boot2docker-cli, and then runs `boot2docker --init ip`, we create a vm
	// which cannot run, because it fails to have have the boot2docker.iso and the ssh keys
	B2D.Init = false
	//flags.BoolVarP(&B2D.Init, "init", "i", false, "auto initialize vm instance.")

	flags.BoolVarP(&B2D.Verbose, "verbose", "v", false, "display verbose command invocations.")
	flags.StringVar(&B2D.Driver, "driver", "virtualbox", "hypervisor driver.")
	flags.StringVar(&B2D.SSH, "ssh", "ssh", "path to SSH client utility.")
	flags.StringVar(&B2D.SSHGen, "ssh-keygen", "ssh-keygen", "path to ssh-keygen utility.")

	sshdir, _ := cfgDir(".ssh")
	flags.StringVar(&B2D.SSHKey, "sshkey", filepath.Join(sshdir, "id_boot2docker"), "path to SSH key to use.")
	flags.UintVarP(&B2D.DiskSize, "disksize", "s", 20000, "boot2docker disk image size (in MB).")
	flags.UintVarP(&B2D.Memory, "memory", "m", 2048, "virtual machine memory size (in MB).")
	flags.Uint16Var(&B2D.SSHPort, "sshport", 2022, "host SSH port (forward to port 22 in VM).")
	flags.Uint16Var(&B2D.DockerPort, "dockerport", 0, "host Docker port (forward to port 2375 in VM). (deprecated - use with care)")
	flags.IPVar(&B2D.HostIP, "hostip", net.ParseIP("192.168.59.3"), "VirtualBox host-only network IP address.")
	flags.IPMaskVar(&B2D.NetMask, "netmask", flag.ParseIPv4Mask("255.255.255.0"), "VirtualBox host-only network mask.")
	flags.BoolVar(&B2D.DHCPEnabled, "dhcp", true, "enable VirtualBox host-only network DHCP.")
	flags.IPVar(&B2D.DHCPIP, "dhcpip", net.ParseIP("192.168.59.99"), "VirtualBox host-only network DHCP server address.")
	flags.IPVar(&B2D.LowerIP, "lowerip", net.ParseIP("192.168.59.103"), "VirtualBox host-only network DHCP lower bound.")
	flags.IPVar(&B2D.UpperIP, "upperip", net.ParseIP("192.168.59.254"), "VirtualBox host-only network DHCP upper bound.")

	if runtime.GOOS != "windows" {
		//SerialFile ~~ filepath.Join(dir, B2D.vm+".sock")
		flags.StringVar(&B2D.SerialFile, "serialfile", "", "path to the serial socket/pipe.")
		flags.BoolVar(&B2D.Serial, "serial", false, "try serial console to get IP address (experimental)")
	} else {
		B2D.Serial = false
	}

	// Set the defaults
	if err := flags.Parse([]string{}); err != nil {
		return nil, err
	}
	// Over-ride from the profile file
	filename := cfgFilename(B2D.Dir)
	if _, err := os.Lstat(filename); err == nil {
		if _, err := toml.DecodeFile(filename, &B2D); err != nil {
			return nil, err
		}
	}

	if B2D.SerialFile == "" {
		if runtime.GOOS == "windows" {
			//SerialFile ~~ filepath.Join(dir, B2D.vm+".sock")
			B2D.SerialFile = `\\.\pipe\` + B2D.VM
		} else {
			B2D.SerialFile = filepath.Join(dir, B2D.VM+".sock")
		}
	}
	// for cmd==ssh only:
	// only pass the params up to and including the `ssh` command - after that,
	// there might be other -flags that are destined for the ssh cmd
	sshIdx := 1
	for sshIdx < len(os.Args) && os.Args[sshIdx-1] != "ssh" {
		sshIdx++
	}
	// Command-line overrides profile config.
	if err := flags.Parse(os.Args[1:sshIdx]); err != nil {
		return nil, err
	}

	return flags, nil
}

func usageShort() {
	fmt.Fprintf(os.Stderr, "Usage: %s [<options>] {help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|ip|socket|shellinit|delete|destroy|download|version|share} [<args>]\n", os.Args[0])
}

func usageLong(flags *flag.FlagSet) {
	// NOTE: the help message uses spaces, not tabs for indentation!
	fmt.Fprintf(os.Stderr, `Usage: %s [<options>] <command> [<args>]

Boot2Docker management utility.

Commands:
   init                Create a new Boot2Docker VM.
   up|start|boot       Start VM from any states.
   ssh [ssh-command]   Login to VM via SSH.
   save|suspend        Suspend VM and save state to disk.
   down|stop|halt      Gracefully shutdown the VM.
   restart             Gracefully reboot the VM.
   poweroff            Forcefully power off the VM (may corrupt disk image).
   reset               Forcefully power cycle the VM (may corrupt disk image).
   delete|destroy      Delete Boot2Docker VM and its disk image.
   config|cfg          Show selected profile file settings.
   info                Display detailed information of VM.
   ip                  Display the IP address of the VM's Host-only network.
   socket              Display the DOCKER_HOST socket to connect to.
   share               Share the current directory to the remote Docker host.
   shellinit           Display the shell command to set up the Docker client.
   status              Display current state of VM.
   download            Download Boot2Docker ISO image.
   upgrade             Upgrade the Boot2Docker ISO image (restart if running).
   version             Display version information.

Options:
`, os.Args[0])
	flags.PrintDefaults()
}
