package vsphere

import (
	"fmt"
	"os"
	"strings"

	"github.com/boot2docker/boot2docker-cli/vsphere/errors"
	"github.com/howeyc/gopass"
)

type VcConn struct {
	cfg      *DriverCfg
	password string
}

func NewVcConn(cfg *DriverCfg) VcConn {
	return VcConn{
		cfg:      cfg,
		password: "",
	}
}

func (conn VcConn) Login() error {
	err := conn.queryAboutInfo()
	if err == nil {
		return nil
	}
	if _, ok := err.(*errors.GovcNotFoundError); ok {
		return err
	}

	fmt.Fprintf(os.Stdout, "Enter vCenter Password: ")
	password := gopass.GetPasswd()
	conn.password = string(password[:])

	err = conn.queryAboutInfo()
	if err == nil {
		return nil
	}
	return err
}

func (conn VcConn) DatastoreLs(path string) (string, error) {
	args := []string{"datastore.ls"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--ds=%s", conn.cfg.VcenterDS))
	args = append(args, path)
	stdout, stderr, err := govcOutErr(args...)
	if stderr == "" && err == nil {
		return stdout, nil
	}
	return "", errors.NewDatastoreError(conn.cfg.VcenterDC, "ls", stderr)
}

func (conn VcConn) DatastoreMkdir(dirName string) error {
	_, err := conn.DatastoreLs(dirName)
	if err == nil {
		return nil
	}

	fmt.Fprintf(os.Stdout, "Creating directory %s on datastore %s of vCenter %s... ",
		dirName, conn.cfg.VcenterDS, conn.cfg.VcenterIp)

	args := []string{"datastore.mkdir"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--ds=%s", conn.cfg.VcenterDS))
	args = append(args, dirName)
	_, stderr, err := govcOutErr(args...)
	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewDatastoreError(conn.cfg.VcenterDS, "mkdir", stderr)
	}
}

func (conn VcConn) DatastoreUpload(localPath string) error {
	stdout, err := conn.DatastoreLs(DATASTORE_DIR)
	if err == nil && strings.Contains(stdout, DATASTORE_ISO_NAME) {
		fmt.Fprintf(os.Stdout, "boot2docker ISO already uploaded, skipping upload... \n")
		return nil
	}

	fmt.Fprintf(os.Stdout, "Uploading %s to %s on datastore %s of vCenter %s... ",
		localPath, DATASTORE_DIR, conn.cfg.VcenterDS, conn.cfg.VcenterIp)

	dsPath := fmt.Sprintf("%s/%s", DATASTORE_DIR, DATASTORE_ISO_NAME)
	args := []string{"datastore.upload"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--ds=%s", conn.cfg.VcenterDS))
	args = append(args, localPath)
	args = append(args, dsPath)
	_, stderr, err := govcOutErr(args...)
	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewDatastoreError(conn.cfg.VcenterDC, "upload", stderr)
	}
}

func (conn VcConn) VmInfo(vmName string) (string, error) {
	args := []string{"vm.info"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--dc=%s", conn.cfg.VcenterDC))
	args = append(args, vmName)

	stdout, stderr, err := govcOutErr(args...)
	if strings.Contains(stdout, "Name") && stderr == "" && err == nil {
		return stdout, nil
	} else {
		return "", errors.NewVmError("find", vmName, "VM not found")
	}
}

func (conn VcConn) VmCreate(isoPath, memory, vmName string) error {
	fmt.Fprintf(os.Stdout, "Creating virtual machine %s of vCenter %s... ",
		vmName, conn.cfg.VcenterIp)

	args := []string{"vm.create"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--net=%s", conn.cfg.VcenterNet))
	args = append(args, fmt.Sprintf("--dc=%s", conn.cfg.VcenterDC))
	args = append(args, fmt.Sprintf("--ds=%s", conn.cfg.VcenterDS))
	args = append(args, fmt.Sprintf("--iso=%s", isoPath))
	args = append(args, fmt.Sprintf("--m=%s", memory))
	args = append(args, fmt.Sprintf("--c=%s", conn.cfg.Cpu))
	args = append(args, "--disk.controller=scsi")
	args = append(args, "--on=false")
	if conn.cfg.VcenterPool != "" {
		args = append(args, fmt.Sprintf("--pool=%s", conn.cfg.VcenterPool))
	}
	if conn.cfg.VcenterHostIp != "" {
		args = append(args, fmt.Sprintf("--host.ip=%s", conn.cfg.VcenterHostIp))
	}
	args = append(args, vmName)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewVmError("create", vmName, stderr)
	}
}

func (conn VcConn) VmPowerOn(vmName string) error {
	fmt.Fprintf(os.Stdout, "Powering on virtual machine %s of vCenter %s... ",
		vmName, conn.cfg.VcenterIp)

	args := []string{"vm.power"}
	args = conn.AppendConnectionString(args)
	args = append(args, "-on")
	args = append(args, vmName)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewVmError("power on", vmName, stderr)
	}
}

func (conn VcConn) VmPowerOff(vmName string) error {
	fmt.Fprintf(os.Stdout, "Powering off virtual machine %s of vCenter %s... ",
		vmName, conn.cfg.VcenterIp)

	args := []string{"vm.power"}
	args = conn.AppendConnectionString(args)
	args = append(args, "-off")
	args = append(args, vmName)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewVmError("power on", vmName, stderr)
	}
}

func (conn VcConn) VmDestroy(vmName string) error {
	fmt.Fprintf(os.Stdout, "Deleting virtual machine %s of vCenter %s... ",
		vmName, conn.cfg.VcenterIp)

	args := []string{"vm.destroy"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--dc=%s", conn.cfg.VcenterDC))
	args = append(args, vmName)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return errors.NewVmError("delete", vmName, stderr)
	}

}

func (conn VcConn) VmDiskCreate(vmName, diskSize string) error {
	args := []string{"vm.disk.create"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--vm=%s", vmName))
	args = append(args, fmt.Sprintf("--ds=%s", conn.cfg.VcenterDS))
	args = append(args, fmt.Sprintf("--name=%s", vmName))
	args = append(args, fmt.Sprintf("--size=%sMiB", diskSize))

	_, stderr, err := govcOutErr(args...)
	if stderr == "" && err == nil {
		return nil
	} else {
		return errors.NewVmError("add network", vmName, stderr)
	}
}

func (conn VcConn) VmAttachNetwork(vmName string) error {
	args := []string{"vm.network.add"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--vm=%s", vmName))
	args = append(args, fmt.Sprintf("--net=%s", conn.cfg.VcenterNet))

	_, stderr, err := govcOutErr(args...)
	if stderr == "" && err == nil {
		return nil
	} else {
		return errors.NewVmError("add network", vmName, stderr)
	}
}

func (conn VcConn) VmFetchIp(vmName string) (string, error) {
	fmt.Fprintf(os.Stdout, "Fetching IP on virtual machine %s of vCenter %s... ",
		vmName, conn.cfg.VcenterIp)

	args := []string{"vm.ip"}
	args = conn.AppendConnectionString(args)
	args = append(args, vmName)
	stdout, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		fmt.Fprintf(os.Stdout, "ok!\n")
		return stdout, nil
	} else {
		fmt.Fprintf(os.Stderr, "failed!\n")
		return "", errors.NewVmError("fetching IP", vmName, stderr)
	}
}

func (conn VcConn) GuestMkdir(guestUser, guestPass, vmName, dirName string) error {
	args := []string{"guest.mkdir"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--l=%s:%s", guestUser, guestPass))
	args = append(args, fmt.Sprintf("--vm=%s", vmName))
	args = append(args, "-p")
	args = append(args, dirName)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		return nil
	} else {
		return errors.NewGuestError("mkdir", vmName, stderr)
	}
}

func (conn VcConn) GuestUpload(guestUser, guestPass, vmName, localPath, remotePath string) error {
	args := []string{"guest.upload"}
	args = conn.AppendConnectionString(args)
	args = append(args, fmt.Sprintf("--l=%s:%s", guestUser, guestPass))
	args = append(args, fmt.Sprintf("--vm=%s", vmName))
	args = append(args, "-f")
	args = append(args, localPath)
	args = append(args, remotePath)
	_, stderr, err := govcOutErr(args...)

	if stderr == "" && err == nil {
		return nil
	} else {
		return errors.NewGuestError("upload", vmName, stderr)
	}
}

func (conn VcConn) AppendConnectionString(args []string) []string {
	if conn.password == "" {
		args = append(args, fmt.Sprintf("--u=%s@%s", conn.cfg.VcenterUser, cfg.VcenterIp))
	} else {
		args = append(args, fmt.Sprintf("--u=%s:%s@%s", conn.cfg.VcenterUser, conn.password, conn.cfg.VcenterIp))
	}
	args = append(args, "--k=true")
	return args
}

func (conn VcConn) queryAboutInfo() error {
	args := []string{"about"}
	args = conn.AppendConnectionString(args)
	stdout, _, err := govcOutErr(args...)
	if strings.Contains(stdout, "Name") {
		return nil
	}
	if _, ok := err.(*errors.GovcNotFoundError); ok {
		return err
	}
	return errors.NewInvalidLoginError()
}
