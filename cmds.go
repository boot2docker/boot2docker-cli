package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/boot2docker/boot2docker-cli/driver"
	_ "github.com/boot2docker/boot2docker-cli/dummy"
	_ "github.com/boot2docker/boot2docker-cli/virtualbox"
)

func vmNotRunningError(vmName string) error {
	return fmt.Errorf("VM %q is not running. (Did you run `boot2docker up`?)", vmName)
}

// Initialize the boot2docker VM from scratch.
func cmdInit() error {
	B2D.Init = false
	_, err := driver.GetMachine(&B2D)
	if err == nil {
		fmt.Printf("Virtual machine %s already exists\n", B2D.VM)
		return nil
	}

	if _, err := os.Stat(B2D.ISO); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Failed to open ISO image %q: %s", B2D.ISO, err)
		}

		if err := cmdDownload(); err != nil {
			return err
		}
	}

	if _, err := os.Stat(B2D.SSHKey); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Something wrong with SSH Key file %q: %s", B2D.SSHKey, err)
		}

		cmd := exec.Command(B2D.SSHGen, "-t", "rsa", "-N", "", "-f", B2D.SSHKey)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if B2D.Verbose {
			cmd.Stderr = os.Stderr
			fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Error generating new SSH Key into %s: %s", B2D.SSHKey, err)
		}
	}
	//TODO: print a ~/.ssh/config entry for our b2d connection that the user can c&p

	B2D.Init = true
	_, err = driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to initialize machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Bring up the VM from all possible states.
func cmdUp() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Start(); err != nil {
		return fmt.Errorf("Failed to start machine %q: %s", B2D.VM, err)
	}

	if err := m.Refresh(); err != nil {
		return fmt.Errorf("Failed to start machine %q: %s", B2D.VM, err)
	}
	if m.GetState() != driver.Running {
		return fmt.Errorf("Failed to start machine %q (run again with -v for details)", B2D.VM)
	}

	fmt.Println("Waiting for VM and Docker daemon to start...")
	//give the VM a little time to start, so we don't kill the Serial Pipe/Socket
	time.Sleep(time.Duration(B2D.Waittime) * time.Millisecond)
	natSSH := fmt.Sprintf("localhost:%d", m.GetSSHPort())
	IP := ""
	for i := 1; i < B2D.Retries; i++ {
		print(".")
		if B2D.Serial && runtime.GOOS != "windows" {
			if IP, err = RequestIPFromSerialPort(m.GetSerialFile()); err == nil {
				break
			}
		}
		if err := read(natSSH, 1, time.Duration(B2D.Waittime)*time.Millisecond); err == nil {
			if IP, err = RequestIPFromSSH(m); err == nil {
				break
			}
		}
	}
	if B2D.Verbose {
		fmt.Printf("VM Host-only IP address: %s", IP)
		fmt.Printf("\nWaiting for Docker daemon to start...\n")
	}

	time.Sleep(time.Duration(B2D.Waittime) * time.Millisecond)
	socket := ""
	for i := 1; i < B2D.Retries; i++ {
		print("o")
		if socket, err = RequestSocketFromSSH(m); err == nil {
			break
		}
		if B2D.Verbose {
			fmt.Printf("Error requesting socket: %s\n", err)
		}
		time.Sleep(600 * time.Millisecond)
	}
	fmt.Printf("\nStarted.\n")

	if socket == "" {
		// lets try one more time
		time.Sleep(time.Duration(B2D.Waittime) * time.Millisecond)
		fmt.Printf("  Trying to get Docker socket one more time\n")

		if socket, err = RequestSocketFromSSH(m); err != nil {
			fmt.Printf("Error requesting socket: %s\n", err)
		}
	}
	// Copying the certs here - someone might have have written a Windows API client.
	certPath, err := RequestCertsUsingSSH(m)
	if err != nil {
		// These errors are not fatal
		fmt.Fprintf(os.Stderr, "Warning: error copying certificates: %s\n", err)
	}
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("Docker client does not run on Windows for now. Please use\n")
		fmt.Printf("    \"%s\" ssh\n", os.Args[0])
		fmt.Printf("to SSH into the VM instead.\n")
	default:
		if socket == "" {
			fmt.Fprintf(os.Stderr, "Auto detection of the VM's Docker socket failed.\n")
			fmt.Fprintf(os.Stderr, "Please run `boot2docker -v up` to diagnose.\n")
		} else {
			// Check if $DOCKER_* ENV vars are properly configured.
			if !checkEnvironment(socket, certPath) {
				fmt.Printf("\nTo connect the Docker client to the Docker daemon, please set:\n")
				printExport(socket, certPath)
			} else {
				fmt.Printf("Your environment variables are already set correctly.\n")
			}
		}
	}
	fmt.Printf("\n")
	return nil
}

// Give the user the exact command to run to set the env.
func cmdShellInit() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return vmNotRunningError(B2D.VM)
	}

	socket, err := RequestSocketFromSSH(m)
	if err != nil {
		return fmt.Errorf("Error requesting socket: %s\n", err)
	}

	certPath, err := RequestCertsUsingSSH(m)
	if err != nil {
		// These errors are not fatal
		fmt.Fprintf(os.Stderr, "Warning: error copying certificates: %s\n", err)
	}
	printExport(socket, certPath)

	return nil
}

func checkEnvironment(socket, certPath string) bool {
	for name, value := range exports(socket, certPath) {
		if os.Getenv(name) != value {
			return false
		}
	}

	return true
}

func printExport(socket, certPath string) {
	for name, value := range exports(socket, certPath) {
		switch filepath.Base(os.Getenv("SHELL")) {
		case "fish":
			if value == "" {
				fmt.Printf("    set -e %s\n", name)
			} else {
				fmt.Printf("    set -x %s %s\n", name, value)
			}
		default: // default command to export variables POSIX shells, like bash, zsh, etc.
			if value == "" {
				fmt.Printf("    unset %s\n", name)
			} else {
				fmt.Printf("    export %s=%s\n", name, value)
			}
		}
	}
}

func exports(socket, certPath string) map[string]string {
	out := make(map[string]string)

	out["DOCKER_HOST"] = socket
	out["DOCKER_CERT_PATH"] = certPath

	if certPath == "" {
		out["DOCKER_TLS_VERIFY"] = ""
	} else {
		out["DOCKER_TLS_VERIFY"] = "1"
	}

	//if a http_proxy is set, we need to make sure the boot2docker ip
	//is added to the NO_PROXY environment variable
	if os.Getenv("http_proxy") != "" || os.Getenv("HTTP_PROXY") != "" {
		//get the ip from socket/DOCKER_HOST
		re := regexp.MustCompile("tcp://([^:]+):")
		if matches := re.FindStringSubmatch(socket); len(matches) == 2 {
			ip := matches[1]

			//first check for an existing lower case no_proxy var
			no_proxy_var := "no_proxy"
			no_proxy_value := os.Getenv("no_proxy")
			//otherweise try allcaps HTTP_PROXY
			if no_proxy_value == "" {
				no_proxy_var = "NO_PROXY"
				no_proxy_value = os.Getenv("NO_PROXY")
			}

			switch {
			case no_proxy_value == "":
				out[no_proxy_var] = ip
			case strings.Contains(no_proxy_value, ip):
				out[no_proxy_var] = no_proxy_value
			default:
				out[no_proxy_var] = fmt.Sprintf("%s,%s", no_proxy_value, ip)
			}
		}
	}

	return out
}

// Tell the user the config (and later let them set it?)
func cmdConfig() error {
	dir, err := cfgDir(".boot2docker")
	if err != nil {
		return fmt.Errorf("Error working out Profile file location: %s\n", err)
	}
	filename := cfgFilename(dir)
	fmt.Printf("# boot2docker profile filename: %s\n", filename)
	fmt.Println(printConfig())
	return nil
}

// Suspend and save the current state of VM on disk.
func cmdSave() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s\n", B2D.VM, err)
	}
	if err := m.Save(); err != nil {
		return fmt.Errorf("Failed to save machine %q: %s\n", B2D.VM, err)
	}
	return nil
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Stop(); err != nil {
		return fmt.Errorf("Failed to stop machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Forcefully power off the VM (equivalent to unplug power). Might corrupt disk
// image.
func cmdPoweroff() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Poweroff(); err != nil {
		return fmt.Errorf("Failed to poweroff machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Upgrade the boot2docker ISO - preserving server state
func cmdUpgrade() error {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if B2D.Clobber {
			err := upgradeDockerClientBinary()
			if err != nil {
				return err
			}
		} else {
			fmt.Println("Skipping client binary download, use --clobber=true to enable...")
		}
	}
	if err := upgradeBoot2DockerBinary(); err != nil {
		return fmt.Errorf("Error upgrading boot2docker binary: %s", err)
	}
	m, err := driver.GetMachine(&B2D)
	if err == nil {
		if m.GetState() == driver.Running || m.GetState() == driver.Saved || m.GetState() == driver.Paused {
			// Windows won't let us move the ISO aside while it's in use
			if err = cmdStop(); err == nil {
				if err = cmdDownload(); err == nil {
					err = cmdUp()
				}
			}
			return err
		}
	}
	return cmdDownload()
}

func upgradeBoot2DockerBinary() error {
	var (
		goos, arch, ext string
	)
	latestVersion, err := getLatestReleaseName("https://api.github.com/repos/boot2docker/boot2docker-cli/releases")
	if err != nil {
		return fmt.Errorf("Error attempting to get the latest boot2docker-cli release: %s", err)
	}
	baseUrl := "https://github.com/boot2docker/boot2docker-cli/releases/download"

	ext = ""

	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	default:
		return fmt.Errorf("Architecture not supported")
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		goos = runtime.GOOS
	case "windows":
		goos = "windows"
		arch = "amd64"
		ext = ".exe"
	default:
		return fmt.Errorf("Operating system not supported")
	}
	binaryUrl := fmt.Sprintf("%s/%s/boot2docker-%s-%s-%s%s", baseUrl, latestVersion, latestVersion, goos, arch, ext)
	currentBoot2DockerVersion := Version
	if err := attemptUpgrade(binaryUrl, "boot2docker", latestVersion, currentBoot2DockerVersion); err != nil {
		return fmt.Errorf("Error attempting upgrade: %s", err)
	}
	return nil
}

func upgradeDockerClientBinary() error {
	var (
		clientOs, clientArch string
	)
	resp, err := http.Get("https://get.docker.com/latest")
	if err != nil {
		return fmt.Errorf("Error checking the latest version of Docker: %s", err)
	}
	defer resp.Body.Close()
	latestVersionBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body on latest version of Docker call: %s", err)
	}
	latestVersion := strings.TrimSpace(string(latestVersionBytes))
	localClientVersion, err := getLocalClientVersion()
	if err != nil {
		return fmt.Errorf("Error getting local Docker client version: %s", err)
	}
	switch runtime.GOARCH {
	case "amd64":
		clientArch = "x86_64"
	default:
		return fmt.Errorf("Architecture not supported")
	}

	switch runtime.GOOS {
	case "darwin":
		clientOs = "Darwin"
	case "linux":
		clientOs = "Linux"
	default:
		return fmt.Errorf("Operating system not supported")
	}
	binaryUrl := fmt.Sprintf("https://get.docker.com/builds/%s/%s/docker-latest", clientOs, clientArch)
	if err := attemptUpgrade(binaryUrl, "docker", latestVersion, localClientVersion); err != nil {
		return fmt.Errorf("Error attempting upgrade: %s", err)
	}
	return nil
}

func attemptUpgrade(binaryUrl, binaryName, latestVersion, localVersion string) error {
	if (latestVersion != localVersion && !strings.Contains(latestVersion, "rc")) || B2D.ForceUpgradeDownload {
		if err := backupAndDownload(binaryUrl, binaryName, localVersion); err != nil {
			return fmt.Errorf("Error attempting backup and download of Docker client binary: %s", err)
		}
	} else {
		fmt.Printf("%s is up to date (%s), skipping upgrade...\n", binaryName, localVersion)
	}
	return nil
}

func backupAndDownload(binaryUrl, binaryName, localVersion string) error {
	binaryPath, err := exec.LookPath(binaryName)
	if err != nil {
		return fmt.Errorf("Error attempting to locate local binary: %s", err)
	}
	path := strings.TrimSpace(string(binaryPath))

	fmt.Println("Backing up existing", binaryName, "binary...")
	if err := backupBinary(binaryName, localVersion, path); err != nil {
		return fmt.Errorf("Error backing up docker client: %s", err)
	}

	fmt.Println("Downloading new", binaryName, "client binary...")
	if err := download(path, binaryUrl); err != nil {
		return fmt.Errorf("Error attempting to download new client binary: %s", err)
	}
	if err := os.Chmod(path, 0755); err != nil {
		return err
	}
	fmt.Printf("Success: downloaded %s\n\tto %s\n\tThe old version is backed up to ~/.boot2docker.\n", binaryUrl, path)
	return nil
}

func backupBinary(binaryName, localVersion, path string) error {
	dir, err := cfgDir(".boot2docker")
	if err != nil {
		return fmt.Errorf("Error getting boot2docker config dir: %s", err)
	}
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error opening binary for reading at %s: %s", path, err)
	}
	backupName := fmt.Sprintf("%s-%s", binaryName, localVersion)
	if err := ioutil.WriteFile(filepath.Join(dir, backupName), buf, 0755); err != nil {
		return fmt.Errorf("Error creating backup file: %s", err)
	}
	return nil
}

// Gracefully stop and then start the VM.
func cmdRestart() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Restart(); err != nil {
		return fmt.Errorf("Failed to restart machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Forcefully reset (equivalent to cold boot) the VM. Might corrupt disk image.
func cmdReset() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Reset(); err != nil {
		return fmt.Errorf("Failed to reset machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Delete the VM and associated disk image.
func cmdDelete() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		if err == driver.ErrMachineNotExist {
			return fmt.Errorf("Machine %q does not exist.", B2D.VM)
		}
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Delete(); err != nil {
		return fmt.Errorf("Failed to delete machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Show detailed info of the VM.
func cmdInfo() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	b, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return fmt.Errorf("Failed to encode machine %q info: %s", B2D.VM, err)
	}

	os.Stdout.Write(b)

	return nil
}

// Show the current state of the VM.
func cmdStatus() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	fmt.Println(m.GetState())
	return nil
}

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return vmNotRunningError(B2D.VM)
	}

	// find the ssh cmd string and then pass any remaining strings to ssh
	// TODO: it's a shame to repeat the same code as in config.go, but I
	//       didn't find a way to share the unsharable without more rework
	i := 1
	for i < len(os.Args) && os.Args[i-1] != "ssh" {
		i++
	}

	if err := cmdInteractive(m, os.Args[i:]...); err != nil {
		return fmt.Errorf("%s", err)
	}
	return nil
}

func cmdIP() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return vmNotRunningError(B2D.VM)
	}

	IP := ""
	if B2D.Serial {
		if runtime.GOOS != "windows" {
			if IP, err = RequestIPFromSerialPort(m.GetSerialFile()); err != nil {
				if B2D.Verbose {
					fmt.Printf("Error getting IP via Serial: %s\n", err)
				}
			}
		}
	}

	if IP == "" {
		if IP, err = RequestIPFromSSH(m); err != nil {
			if B2D.Verbose {
				fmt.Printf("Error getting IP via SSH: %s\n", err)
			}
		}
	}
	if IP != "" {
		fmt.Println(IP)
	} else {
		fmt.Fprintf(os.Stderr, "\nFailed to get VM Host only IP address.\n")
		fmt.Fprintf(os.Stderr, "\tWas the VM initialized using boot2docker?\n")
	}
	return nil
}

// Download the boot2docker ISO image.
func cmdDownload() error {
	url := B2D.ISOURL

	// match github (enterprise) release urls:
	// https://api.github.com/repos/../../relases or
	// https://some.github.enterprise/api/v3/repos/../../relases
	re := regexp.MustCompile("https://([^/]+)(/api/v3)?/repos/([^/]+)/([^/]+)/releases")
	if matches := re.FindStringSubmatch(url); len(matches) == 5 {
		tag, err := getLatestReleaseName(url)
		if err != nil {
			return fmt.Errorf("Failed to get latest release: %s", err)
		}
		host := matches[1]
		org := matches[3]
		repo := matches[4]
		if host == "api.github.com" {
			host = "github.com"
		}
		fmt.Printf("Latest release for %s/%s/%s is %s\n", host, org, repo, tag)
		url = fmt.Sprintf("https://%s/%s/%s/releases/download/%s/boot2docker.iso", host, org, repo, tag)
	}

	fmt.Println("Downloading boot2docker ISO image...")
	if err := download(B2D.ISO, url); err != nil {
		return fmt.Errorf("Failed to download ISO image: %s", err)
	}
	fmt.Printf("Success: downloaded %s\n\tto %s\n", url, B2D.ISO)
	return nil
}
