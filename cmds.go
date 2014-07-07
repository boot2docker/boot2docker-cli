package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
)

// Initialize the boot2docker VM from scratch.
func cmdInit() int {
	// TODO(@riobard) break up this command into multiple stages
	m, err := vbx.GetMachine(B2D.VM)
	if err == nil {
		logf("Virtual machine %s already exists", B2D.VM)
		return 1
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.DockerPort)) {
		logf("--dockerport=%d on localhost is occupied. Please choose another one.", B2D.DockerPort)
		return 1
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.SSHPort)) {
		logf("--sshport=%d on localhost is occupied. Please choose another one.", B2D.SSHPort)
		return 1
	}

	if _, err := os.Stat(B2D.ISO); err != nil {
		if !os.IsNotExist(err) {
			logf("Failed to open ISO image %q: %s", B2D.ISO, err)
			return 1
		}

		if exitcode := cmdDownload(); exitcode != 0 {
			return exitcode
		}
	}
	if _, err := os.Stat(B2D.SSHKey); err != nil {
		if !os.IsNotExist(err) {
			logf("Something wrong with SSH Key file %q: %s", B2D.SSHKey, err)
			return 1
		}
		if err := cmdInteractive(B2D.SSHGen, "-t", "rsa", "-N", "", "-f", B2D.SSHKey); err != nil {
			logf("Error generating new SSH Key into %s: %s", B2D.SSHKey, err)
			return 1
		}
	}
	//TODO: print a ~/.ssh/config entry for our b2d connection that the user can c&p

	logf("Creating VM %s...", B2D.VM)
	m, err = vbx.CreateMachine(B2D.VM, "")
	if err != nil {
		logf("FIRST: Failed to create VM %q: %s", B2D.VM, err)
		// double tap. VBox will sometimes (like on initial install, or reboot)
		// fail to start the service the first time.
		m, err = vbx.CreateMachine(B2D.VM, "")
		if err != nil {
			logf("Failed to create VM %q: %s", B2D.VM, err)
			return 1
		}
	}

	logf("Apply interim patch to VM %s (https://www.virtualbox.org/ticket/12748)", B2D.VM)
	if err := vbx.SetExtra(B2D.VM, "VBoxInternal/CPUM/EnableHVP", "1"); err != nil {
		logf("Failed to patch vm: %s", err)
		return 1
	}

	m.OSType = "Linux26_64"
	m.CPUs = uint(runtime.NumCPU())
	m.Memory = B2D.Memory
	m.SerialFile = B2D.SerialFile

	m.Flag |= vbx.F_pae
	m.Flag |= vbx.F_longmode // important: use x86-64 processor
	m.Flag |= vbx.F_rtcuseutc
	m.Flag |= vbx.F_acpi
	m.Flag |= vbx.F_ioapic
	m.Flag |= vbx.F_hpet
	m.Flag |= vbx.F_hwvirtex
	m.Flag |= vbx.F_vtxvpid
	m.Flag |= vbx.F_largepages
	m.Flag |= vbx.F_nestedpaging

	m.BootOrder = []string{"dvd"}
	if err := m.Modify(); err != nil {
		logf("Failed to modify VM %q: %s", B2D.VM, err)
		return 1
	}

	logf("Setting NIC #1 to use NAT network...")
	if err := m.SetNIC(1, vbx.NIC{Network: vbx.NICNetNAT, Hardware: vbx.VirtIO}); err != nil {
		logf("Failed to add network interface to VM %q: %s", B2D.VM, err)
		return 1
	}

	pfRules := map[string]vbx.PFRule{
		"ssh":    {Proto: vbx.PFTCP, HostIP: net.ParseIP("127.0.0.1"), HostPort: B2D.SSHPort, GuestPort: 22},
		"docker": {Proto: vbx.PFTCP, HostIP: net.ParseIP("127.0.0.1"), HostPort: B2D.DockerPort, GuestPort: 2375},
	}

	for name, rule := range pfRules {
		if err := m.AddNATPF(1, name, rule); err != nil {
			logf("Failed to add port forwarding to VM %q: %s", B2D.VM, err)
			return 1
		}
		logf("Port forwarding [%s] %s", name, rule)
	}

	hostIFName, err := getHostOnlyNetworkInterface()
	if err != nil {
		logf("Failed to create host-only network interface: %s", err)
		return 1
	}

	logf("Setting NIC #2 to use host-only network %q...", hostIFName)
	if err := m.SetNIC(2, vbx.NIC{Network: vbx.NICNetHostonly, Hardware: vbx.VirtIO, HostonlyAdapter: hostIFName}); err != nil {
		logf("Failed to add network interface to VM %q: %s", B2D.VM, err)
		return 1
	}

	logf("Setting VM storage...")
	if err := m.AddStorageCtl("SATA", vbx.StorageController{SysBus: vbx.SysBusSATA, HostIOCache: true, Bootable: true}); err != nil {
		logf("Failed to add storage controller to VM %q: %s", B2D.VM, err)
		return 1
	}

	if err := m.AttachStorage("SATA", vbx.StorageMedium{Port: 0, Device: 0, DriveType: vbx.DriveDVD, Medium: B2D.ISO}); err != nil {
		logf("Failed to attach ISO image %q: %s", B2D.ISO, err)
		return 1
	}

	diskImg := filepath.Join(m.BaseFolder, fmt.Sprintf("%s.vmdk", B2D.VM))

	if _, err := os.Stat(diskImg); err != nil {
		if !os.IsNotExist(err) {
			logf("Failed to open disk image %q: %s", diskImg, err)
			return 1
		}

		if B2D.VMDK != "" {
			logf("Using %v as base VMDK", B2D.VMDK)
			if err := copyDiskImage(diskImg, B2D.VMDK); err != nil {
				logf("Failed to copy disk image %v from %v: %s", diskImg, B2D.VMDK, err)
				return 1
			}
		} else {
			magicString := "boot2docker, please format-me"

			buf := new(bytes.Buffer)
			tw := tar.NewWriter(buf)

			// magicString first so the automount script knows to format the disk
			file := &tar.Header{Name: magicString, Size: int64(len(magicString))}
			if err := tw.WriteHeader(file); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			if _, err := tw.Write([]byte(magicString)); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			// .ssh/key.pub => authorized_keys
			file = &tar.Header{Name: ".ssh", Typeflag: tar.TypeDir, Mode: 0700}
			if err := tw.WriteHeader(file); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			pubKey, err := ioutil.ReadFile(B2D.SSHKey + ".pub")
			if err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			file = &tar.Header{Name: ".ssh/authorized_keys", Size: int64(len(pubKey)), Mode: 0644}
			if err := tw.WriteHeader(file); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			if _, err := tw.Write([]byte(pubKey)); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			file = &tar.Header{Name: ".ssh/authorized_keys2", Size: int64(len(pubKey)), Mode: 0644}
			if err := tw.WriteHeader(file); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			if _, err := tw.Write([]byte(pubKey)); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}
			if err := tw.Close(); err != nil {
				logf("Error making tarfile: %s", err)
				return 1
			}

			if err := makeDiskImage(diskImg, B2D.DiskSize, buf.Bytes()); err != nil {
				logf("Failed to create disk image %q: %s", diskImg, err)
				return 1
			}
		}
	}

	if err := m.AttachStorage("SATA", vbx.StorageMedium{Port: 1, Device: 0, DriveType: vbx.DriveHDD, Medium: diskImg}); err != nil {
		logf("Failed to attach disk image %q: %s", diskImg, err)
		return 1
	}

	logf("Done. Type `%s up` to start the VM.", os.Args[0])
	return 0
}

// Bring up the VM from all possible states.
func cmdUp() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Start(); err != nil {
		logf("Failed to start machine %q: %s", B2D.VM, err)
		return 1
	}

	if err := m.Refresh(); err != nil {
		logf("Failed to start machine %q: %s", B2D.VM, err)
		return 1
	}
	if m.State != vbx.Running {
		logf("Failed to start machine %q (run again with -v for details)", B2D.VM)
		return 1
	}

	logf("Waiting for VM to be started...")
	//give the VM a little time to start, so we don't kill the Serial Pipe/Socket
	time.Sleep(600 * time.Millisecond)
	natSSH := fmt.Sprintf("localhost:%d", B2D.SSHPort)
	IP := ""
	for i := 1; i < 30; i++ {
		if B2D.Serial && runtime.GOOS != "windows" {
			if IP = RequestIPFromSerialPort(m.SerialFile); IP != "" {
				break
			}
		}
		if err := read(natSSH, 1, 2*time.Second); err == nil {
			IP = RequestIPFromSSH(m)
			break
		}

		print(".")
	}
	print("\n")

	logf("Started.")

	if IP == "" {
		// lets try one more time
		time.Sleep(600 * time.Millisecond)
		logf("  Trying to get IP one more time")

		IP = RequestIPFromSSH(m)
	}
	switch runtime.GOOS {
	case "windows":
		logf("Docker client does not run on Windows for now. Please use")
		logf("    \"%s\" ssh", os.Args[0])
		logf("to SSH into the VM instead.")
	default:
		if IP == "" {
			logf("Auto detection of the VM's IP address failed.")
			logf("Please run `boot2docker -v up` to diagnose.")
		} else {
			// Check if $DOCKER_HOST ENV var is properly configured.
			if os.Getenv("DOCKER_HOST") != fmt.Sprintf("tcp://%s:%d", IP, m.DockerPort) {
				logf("To connect the Docker client to the Docker daemon, please set:")
				logf("    export DOCKER_HOST=tcp://%s:%d", IP, m.DockerPort)
			} else {
				logf("Your DOCKER_HOST env variable is already set correctly.")
			}
		}
	}
	return 0
}

// Tell the user the config (and later let them set it?)
func cmdConfig() int {
	dir, err := getCfgDir(".boot2docker")
	if err != nil {
		logf("Error working out Profile file location: %s", err)
		return 1
	}
	filename := getCfgFilename(dir)
	logf("boot2docker profile filename: %s", filename)
	fmt.Println(printConfig())
	return 0
}

// Suspend and save the current state of VM on disk.
func cmdSave() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Save(); err != nil {
		logf("Failed to save machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Stop(); err != nil {
		logf("Failed to stop machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Forcefully power off the VM (equivalent to unplug power). Might corrupt disk
// image.
func cmdPoweroff() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Poweroff(); err != nil {
		logf("Failed to poweroff machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Gracefully stop and then start the VM.
func cmdRestart() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Restart(); err != nil {
		logf("Failed to restart machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Forcefully reset (equivalent to cold boot) the VM. Might corrupt disk image.
func cmdReset() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Reset(); err != nil {
		logf("Failed to reset machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Delete the VM and associated disk image.
func cmdDelete() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		if err == vbx.ErrMachineNotExist {
			logf("Machine %q does not exist.", B2D.VM)
			return 0
		}
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Delete(); err != nil {
		logf("Failed to delete machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show detailed info of the VM.
func cmdInfo() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		logf("Failed to encode machine %q info: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show the current state of the VM.
func cmdStatus() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	fmt.Println(m.State)
	return 0
}

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}

	if m.State != vbx.Running {
		logf("VM %q is not running.", B2D.VM)
		return 1
	}

	// find the ssh cmd string and then pass any remaining strings to ssh
	// TODO: it's a shame to repeat the same code as in config.go, but I
	//       didn't find a way to share the unsharable without more rework
	i := 1
	for i < len(os.Args) && os.Args[i-1] != "ssh" {
		i++
	}

	sshArgs := append([]string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts."
		"-p", fmt.Sprintf("%d", m.SSHPort),
		"-i", B2D.SSHKey,
		"docker@localhost",
	}, os.Args[i:]...)

	if err := cmdInteractive(B2D.SSH, sshArgs...); err != nil {
		logf("%s", err)
		return 1
	}
	return 0
}

func cmdIP() int {
	m, err := vbx.GetMachine(B2D.VM)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}

	if m.State != vbx.Running {
		logf("VM %q is not running.", B2D.VM)
		return 1
	}

	IP := GetIPForMachine(m)

	if IP != "" {
		errf("\nThe VM's Host only interface IP address is: ")
		fmt.Printf("%s", IP)
		errf("\n\n")
	} else {
		errf("\nFailed to get VM Host only IP address.\n")
		errf("\tWas the VM initilized using boot2docker?\n")
	}
	return 0
}

// Download the boot2docker ISO image.
func cmdDownload() int {
	logf("Downloading boot2docker ISO image...")
	url := "https://api.github.com/repos/boot2docker/boot2docker/releases"
	tag, err := getLatestReleaseName(url)
	if err != nil {
		logf("Failed to get latest release: %s", err)
		return 1
	}
	logf("Latest release is %s", tag)

	url = fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag)
	if err := download(B2D.ISO, url); err != nil {
		logf("Failed to download ISO image: %s", err)
		return 1
	}
	logf("Success: downloaded %s\n\tto %s", url, B2D.ISO)
	return 0
}

func cmdShellSetup(m *vbx.Machine, out io.Writer) int {
	export := DockerHostExportCommand(m)
	fmt.Fprint(out, export, "\n")
	return 0
}
