package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/boot2docker/boot2docker-cli/driver"
)

func shareDir(pwd string, m driver.Machine) error {
	// Make the destination dir.
	cmd := getSSHCommand(m, "sudo mkdir -p '"+pwd+"' && sudo chown docker '"+pwd+"'")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", string(b))
	}
	// getHostIP should probably be a machine method.
	IP, err := RequestIPFromSSH(m)
	if err != nil {
		return err
	}

	if B2D.ShareDriver == "rsync" {
		return shareDirRsync(pwd, m, true, IP)
	} else if B2D.ShareDriver == "rsyncfrom" {
		return shareDirRsync(pwd, m, false, IP)
	} else if B2D.ShareDriver == "smb" {
		return shareDirOSXSMB(pwd, m, IP)

	} else if B2D.ShareDriver == "sshfs" {
		return shareDirSSHFS(pwd, m, IP)

	} else {
		return fmt.Errorf("boot2docker share driver %s not supported", B2D.ShareDriver)
	}
	return nil
}

func shareDirSSHFS(pwd string, m driver.Machine, IP string) error {
	cmd := getSSHCommand(m, "tce-load -wi sshfs-fuse")
	fmt.Println("Please be patient, downloading sshfs modules")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", string(b))
	}

	// send the ssh keys to the b2d host
	cmd = exec.Command("scp", "-p",
		"-P", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts.",
		"/Users/sven/.ssh/id_boot2docker", "docker@localhost:~/.ssh/")

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}
	b, err = cmd.Output()
	if err != nil {
		return err
	}
	cmd = exec.Command("scp", "-p",
		"-P", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts.",
		"/Users/sven/.ssh/id_boot2docker.pub", "docker@localhost:~/.ssh/")

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}
	b, err = cmd.Output()
	if err != nil {
		return err
	}
	if B2D.Verbose {
		fmt.Printf("scp returned: %s\nEND \n", string(b))
	}

	// need to add b2d:/etc/ssh_host_rsa_key.pub to local:~/.ssh/known_hosts first
	cmd = exec.Command("scp", "-p",
		"-P", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts.",
		"docker@localhost:/var/lib/boot2docker/ssh/ssh_host_rsa_key.pub", "/Users/sven/.ssh/b2d_host",
	)

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}
	b, err = cmd.Output()
	if err != nil {
		return err
	}
	// TODO: really do this only once.
	cmd = exec.Command("sh", "-c",
		"cat ~/.ssh/b2d_host >> ~/.ssh/known_hosts")

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}

	b, err = cmd.Output()
	if err != nil {
		//return err
		fmt.Printf("Error setting up share %u\n", err)
	}
	if B2D.Verbose {
		fmt.Printf("rsync returned: %s\nEND \n", string(b))
	}

	// Add the b2d key to the .ssh/authorized_keys file
	// TODO: really do this only once.
	cmd = exec.Command("sh", "-c",
		"cat ~/.ssh/id_boot2docker.pub >> ~/.ssh/authorized_keys")

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}

	b, err = cmd.Output()
	if err != nil {
		//return err
		fmt.Printf("Error setting up share %u\n", err)
	}
	if B2D.Verbose {
		fmt.Printf("rsync returned: %s\nEND \n", string(b))
	}

	// need allow_root so the docker daemon can have access?
	cmd = getSSHCommand(m, "sudo sh -c 'echo user_allow_other >> /etc/fuse.conf'")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}
	cmd = getSSHCommand(m, "sshfs -o IdentityFile=/home/docker/.ssh/id_boot2docker -o StrictHostKeyChecking=no -o allow_root -o idmap=user sven@"+B2D.HostIP.String()+":"+pwd+" "+pwd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func shareDirRsync(pwd string, m driver.Machine, toDockerHost bool, IP string) error {
	// Make sure there's an rsync on the tinymce end.
	cmd := getSSHCommand(m, "tce-load -wi rsync")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	out := string(b)
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", out)
	}

	sshkey := B2D.SSHKey
	if sshkey[:2] == "~/" {
		usr, _ := user.Current()
		dir := usr.HomeDir
		sshkey = strings.Replace(sshkey, "~/", dir, 1)
	}
	if toDockerHost {
		// And then push the files to the remote.
		cmd = exec.Command("sh", "-c",
			"rsync -avz --chmod=ugo=rwX -e 'ssh -vv -l docker -i "+
				sshkey+
				" -o StrictHostKeyChecking=no -o IdentitiesOnly=yes' "+
				"./ "+
				"docker@"+IP+":"+pwd)
	} else {
		// And then pull the files from the remote.
		cmd = exec.Command("sh", "-c",
			"rsync -avz --chmod=ugo=rwX -e 'ssh -vv -l docker -i "+
				sshkey+
				" -o StrictHostKeyChecking=no -o IdentitiesOnly=yes' "+
				"docker@"+IP+":"+pwd+
				"./ ")
	}

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}
	b, err = cmd.Output()
	if err != nil {
		return err
	}
	out = string(b)
	if B2D.Verbose {
		fmt.Printf("rsync returned: %s\nEND \n", out)
	}
	return nil
}

// DO NOT USE - not finished as its kinda pointlessly insecure
func shareDirOSXSMB(pwd string, m driver.Machine, IP string) error {
	//TODO: OSX! need to find the same cmd's for Linux and Windows
	cmd := exec.Command("sudo", "sharing",
		"-a", pwd,
		"-A", "boot2docker", //TODO: need to work out a reasonable hash..
		"-s", "100", // only enable smb sharing
		"-g", "100", // enable guest sharing - Don't do this.
	)

	if B2D.Verbose {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
	}

	b, err := cmd.Output()
	if err != nil {
		//return err
		fmt.Printf("Error setting up share %u\n", err)
	}
	if B2D.Verbose {
		fmt.Printf("rsync returned: %s\nEND \n", string(b))
	}

	//mount it
	//"+B2D.HostIP.String()+"
	cmd = getSSHCommand(m, "sudo mount -t cifs //10.10.10.14/boot2docker "+pwd+" -o username=guest,password=,nounix,sec=ntlmssp,noperm,rw")
	b, err = cmd.Output()
	if err != nil {
		return err
	}
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", string(b))
	}
	return nil
}
