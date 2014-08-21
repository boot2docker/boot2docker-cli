package virtualbox

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boot2docker/boot2docker-cli/driver"
)

func init() {
}

var (
	reVMNameUUID      = regexp.MustCompile(`"(.+)" {([0-9a-f-]+)}`)
	reVMInfoLine      = regexp.MustCompile(`(?:"(.+)"|(.+))=(?:"(.*)"|(.*))`)
	reColonLine       = regexp.MustCompile(`(.+):\s+(.*)`)
	reMachineNotFound = regexp.MustCompile(`Could not find a registered machine named '(.+)'`)
)

var (
	ErrMachineExist    = errors.New("machine already exists")
	ErrMachineNotExist = errors.New("machine does not exist")
	ErrVBMNotFound     = errors.New("VBoxManage not found")
)

func vbm(args ...string) error {
	cmd := exec.Command(VBM, args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", VBM, strings.Join(args, " "))
	}
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.Error); ok && ee == exec.ErrNotFound {
			return ErrVBMNotFound
		}
		return err
	}
	return nil
}

func vbmOut(args ...string) (string, error) {
	cmd := exec.Command(VBM, args...)
	if verbose {
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", VBM, strings.Join(args, " "))
	}

	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee == exec.ErrNotFound {
			err = ErrVBMNotFound
		}
	}
	return string(b), err
}

func vbmOutErr(args ...string) (string, string, error) {
	cmd := exec.Command(VBM, args...)
	if verbose {
		log.Printf("executing: %v %v", VBM, strings.Join(args, " "))
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee == exec.ErrNotFound {
			err = ErrVBMNotFound
		}
	}
	return stdout.String(), stderr.String(), err
}

// Get or create the hostonly network interface
func getHostOnlyNetworkInterface(mc *driver.MachineConfig) (string, error) {
	// Check if the interface/dhcp exists.
	nets, err := HostonlyNets()
	if err != nil {
		return "", err
	}

	dhcps, err := DHCPs()
	if err != nil {
		return "", err
	}

	for _, n := range nets {
		if dhcp, ok := dhcps[n.NetworkName]; ok {
			if dhcp.IPv4.IP.Equal(mc.DHCPIP) &&
				dhcp.IPv4.Mask.String() == mc.NetMask.String() &&
				dhcp.LowerIP.Equal(mc.LowerIP) &&
				dhcp.UpperIP.Equal(mc.UpperIP) &&
				dhcp.Enabled == mc.DHCPEnabled {
				return n.Name, nil
			}
		}
	}

	// No existing host-only interface found. Create a new one.
	hostonlyNet, err := CreateHostonlyNet()
	if err != nil {
		return "", err
	}
	hostonlyNet.IPv4.IP = mc.HostIP
	hostonlyNet.IPv4.Mask = mc.NetMask
	if err := hostonlyNet.Config(); err != nil {
		return "", err
	}

	// Create and add a DHCP server to the host-only network
	dhcp := driver.DHCP{}
	dhcp.IPv4.IP = mc.DHCPIP
	dhcp.IPv4.Mask = mc.NetMask
	dhcp.LowerIP = mc.LowerIP
	dhcp.UpperIP = mc.UpperIP
	dhcp.Enabled = true
	if err := AddHostonlyDHCP(hostonlyNet.Name, dhcp); err != nil {
		return "", err
	}
	return hostonlyNet.Name, nil
}

// Copy disk image from given source path to destination
func copyDiskImage(dst, src string) (err error) {
	// Open source disk image
	srcImg, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if ee := srcImg.Close(); ee != nil {
			err = ee
		}
	}()
	dstImg, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if ee := dstImg.Close(); ee != nil {
			err = ee
		}
	}()
	_, err = io.Copy(dstImg, srcImg)
	return err
}

// Make a boot2docker VM disk image with the given size (in MB).
func makeDiskImage(dest string, size uint, initialBytes []byte) error {
	// Create the dest dir.
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	// Fill in the magic string so boot2docker VM will detect this and format
	// the disk upon first boot.
	raw := bytes.NewReader(initialBytes)
	return MakeDiskImage(dest, size, raw)
}
