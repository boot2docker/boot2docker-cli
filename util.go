package main

import (
	"encoding/json"
	"fmt"
	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// fmt.Printf to stdout. Convention is to outf info intended for scripting.
func outf(f string, v ...interface{}) {
	fmt.Printf(f, v...)
}

// fmt.Printf to stderr. Convention is to errf info intended for human.
func errf(f string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, f, v...)
}

// Verbose output for debugging.
func logf(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}

// Try if addr tcp://addr is readable for n times at wait interval.
func read(addr string, n int, wait time.Duration) error {
	var lastErr error
	for i := 0; i < n; i++ {
		if B2D.Verbose {
			logf("Connecting to tcp://%v (attempt #%d)", addr, i)
		}
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(wait)
			continue
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(1 * time.Second))
		if _, err = conn.Read(make([]byte, 1)); err != nil {
			lastErr = err
			time.Sleep(wait)
			continue
		}
		return nil
	}
	return lastErr
}

// Check if an addr can be successfully connected.
func ping(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// Download the url to the dest path.
func download(dest, url string) error {
	rsp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	// Create the dest dir.
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s.download", dest))
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if _, err := io.Copy(f, rsp.Body); err != nil {
		// TODO: display download progress?
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if _, err := os.Stat(dest); err == nil {
		backup_dest := dest + ".bak"
		os.Remove(backup_dest)
		if err := os.Rename(dest, backup_dest); err != nil {
			return err
		}
	}
	if err := os.Rename(f.Name(), dest); err != nil {
		return err
	}
	return nil
}

// Get latest release tag name (e.g. "v0.6.0") from a repo on GitHub.
func getLatestReleaseName(url string) (string, error) {
	rsp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	var t []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(rsp.Body).Decode(&t); err != nil {
		return "", err
	}
	if len(t) == 0 {
		return "", fmt.Errorf("no releases found")
	}
	return t[0].TagName, nil
}

// Convenient function to exec a command.
func cmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if B2D.Verbose {
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", name, strings.Join(args, " "))
	}

	b, err := cmd.Output()
	return string(b), err
}

func cmdInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if B2D.Verbose {
		logf("executing: %v %v", name, strings.Join(args, " "))
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

//swiped from dotcloud/docker/utils/utils.go
func CopyFile(src, dst string) (int64, error) {
	if src == dst {
		return 0, nil
	}
	sf, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer sf.Close()
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	df, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer df.Close()
	return io.Copy(df, sf)
}

func reader(r io.Reader) {
	buf := make([]byte, 1024)
	for {
		_, err := io.ReadAtLeast(r, buf[:], 20)
		if err != nil {
			return
		}
	}
}

// use the serial port socket to ask what the VM's host only IP is
func RequestIPFromSerialPort(socket string) string {
	c, err := net.Dial("unix", socket)

	if err != nil {
		return ""
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))

	line := ""
	_, err = c.Write([]byte("\r"))
	_, err = c.Write([]byte("docker\r"))

	IP := ""
	fullLog := ""

	for IP == "" {
		_, err := c.Write([]byte("ip addr show dev eth1\r"))
		if err != nil {
			println(err)
			break
		}
		time.Sleep(1 * time.Second)
		buf := make([]byte, 1024)
		for {
			n, err := c.Read(buf[:])
			if err != nil {
				return IP
			}
			line = line + string(buf[0:n])
			fullLog += string(buf[0:n])
			if strings.Contains(line, "\n") {
				//go looking for the string we want, and chomp line to after the \n
				if i := strings.IndexAny(line, "\n"); i != -1 {
					//     inet 10.180.1.3/16 brd 10.180.255.255 scope global wlan0
					inet := regexp.MustCompile(`^[\t ]*inet ([0-9.]*).*$`)
					if ip := inet.FindStringSubmatch(line[:i]); ip != nil {
						IP = ip[1]
						// clean up
						break
					} else {
						line = line[i+1:]
					}
				}
			}
		}

	}
	go reader(c)
	//give us time reader clean up
	time.Sleep(1 * time.Second)
	if IP == "" && B2D.Verbose {
		logf(fullLog)
	}

	return IP
}

func RequestIPFromSSH(m *vbx.Machine) string {
	// fall back to using the NAT port forwarded ssh
	out, err := cmd(B2D.SSH,
		"-v", // please leave in - this seems to improve the chance of success
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", fmt.Sprintf("%d", m.SSHPort),
		"-i", B2D.SSHKey,
		"docker@localhost",
		"ip addr show dev eth1",
	)
	IP := ""
	if err != nil {
		logf("%s", err)
	} else {
		if B2D.Verbose {
			logf("SSH returned: %s\nEND SSH\n", out)
		}
		// parse to find: inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			vals := strings.Split(strings.TrimSpace(line), " ")
			if len(vals) >= 2 && vals[0] == "inet" {
				IP = vals[1][:strings.Index(vals[1], "/")]
				break
			}
		}
	}
	return IP
}

func GetIPForMachine(m *vbx.Machine) string {
	/*
		Determine the IP address for the default host-only network on a
		machine. In the case of a dummy machine, return "192.0.2.1"
		(TEST-NET-1 from http://tools.ietf.org/html/rfc5737).
	*/
	IP := ""
	if m.UUID == "dummy" {
		return "192.0.2.1"
	}
	if B2D.Serial {
		for i := 1; i < 20; i++ {
			if runtime.GOOS != "windows" {
				if IP = RequestIPFromSerialPort(m.SerialFile); IP != "" {
					break
				}
			}
		}
	}

	if IP == "" {
		IP = RequestIPFromSSH(m)
	}

	return IP
}

func DockerHostExportCommand(m *vbx.Machine) string {
	/*
		Calculate the correct export command to set the DOCKER_HOST environment
		variable.
	*/
	IP := GetIPForMachine(m)
	port := m.DockerPort
	export := fmt.Sprintf("export DOCKER_HOST=tcp://%s:%d", IP, port)
	return export
}
