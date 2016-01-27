package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boot2docker/boot2docker-cli/driver"
)

var (
	// We're looking to get e.g. "1.2.0" from "Docker version 1.2.0, build fa7b24f"
	versionRe = regexp.MustCompile(`(\d+\.?){3}`)
)

const (
	SSHCommGetIp      = "ip addr show dev eth1"
	SSHCommGetTcp     = "grep tcp:// /proc/$(cat /var/run/docker.pid)/cmdline"
	SSHCommTarPems    = "tar c /home/docker/.docker/*.pem"
	SSHCommDaemonArgs = "xargs -0 <  /proc/$(cat /var/run/docker.pid)/cmdline"
)

// Try if addr tcp://addr is readable for n times at wait interval.
func read(addr string, n int, wait time.Duration) error {
	var lastErr error
	for i := 0; i < n; i++ {
		if B2D.Verbose {
			fmt.Printf("Connecting to tcp://%v (attempt #%d)\n", addr, i)
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
		// ".../tags" endpoints
		Name string `json:"name"`

		// ".../releases" endpoints
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
	}
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &t); err != nil {
		var e struct {
			Message          string
			DocumentationUrl string
		}
		if err := json.Unmarshal(body, &e); err != nil {
			return "", fmt.Errorf("Error decoding %s\nbody: %s", err, body)
		}
		return "", fmt.Errorf("Error getting releases: %s\n see %s", e.Message, e.DocumentationUrl)
	}
	if len(t) == 0 {
		return "", fmt.Errorf("no releases found at %q", url)
	}

	// Looking up by tag instead of release.
	// Github API call for docker releases yields nothing,
	// so we use tags API call in this case.
	if strings.Contains(url, "tags") {
		return t[0].Name, nil
	}

	for _, rel := range t {
		if rel.Prerelease {
			// skip "pre-releases" (RCs, etc) entirely
			continue
		}
		return rel.TagName, nil
	}

	return "", fmt.Errorf("no non-prerelease releases found at %q", url)
}

func getLocalClientVersion() (string, error) {
	versionOutput, err := exec.Command("docker", "-v").Output()
	if err != nil {
		return "", err
	}
	versionNumber := versionRe.FindString(string(versionOutput))

	return versionNumber, nil
}

func cmdInteractive(m driver.Machine, args ...string) error {
	cmd := getSSHCommandWithStd(m, args...)
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

type GetSSHCommandFunc func(m driver.Machine, args ...string) Outputer

var sshProvider GetSSHCommandFunc

func init() {

	sshProvider = WrapperGetSSHCommand
}

func getSSHCommandWithStd(m driver.Machine, args ...string) *exec.Cmd {
	cmd := DeafaultGetSSHCommand(m, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

type Outputer interface {
	Output() ([]byte, error)
}

func getSSHCommand(m driver.Machine, args ...string) Outputer {
	return sshProvider(m, args...)
}

func WrapperGetSSHCommand(m driver.Machine, args ...string) Outputer {
	return DeafaultGetSSHCommand(m, args...)
}

func DeafaultGetSSHCommand(m driver.Machine, args ...string) *exec.Cmd {

	DefaultSSHArgs := []string{
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts."
		"-p", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"docker@localhost",
	}

	sshArgs := append(DefaultSSHArgs, args...)
	cmd := exec.Command(B2D.SSH, sshArgs...)
	if B2D.Verbose {
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", cmd.Path, strings.Join(cmd.Args, " "))
	}

	return cmd
}

func RequestIPFromSSH(m driver.Machine) (string, error) {
	cmd := getSSHCommand(m, SSHCommGetIp)

	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	out := string(b)
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", out)
	}
	// parse to find: inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		vals := strings.Split(strings.TrimSpace(line), " ")
		if len(vals) >= 2 && vals[0] == "inet" {
			return vals[1][:strings.Index(vals[1], "/")], nil
		}
	}

	return "", fmt.Errorf("No IP address found %s", out)
}

func RequestSocketFromSSH(m driver.Machine) (string, error) {
	cmd := getSSHCommand(m, SSHCommGetTcp)

	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	out := string(b)
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", out)
	}
	// Lets only use the first one - its possible to specify more than one...
	lines := strings.Split(out, "\n")
	tcpRE := regexp.MustCompile(`^(tcp://)(0.0.0.0)(:.*)`)
	if s := tcpRE.FindStringSubmatch(lines[0]); s != nil {
		IP, err := RequestIPFromSSH(m)
		if err != nil {
			return "", err
		}
		return s[1] + IP + s[3], nil
	}
	if !strings.HasPrefix(lines[0], "tcp://") {
		return "", fmt.Errorf("Error requesting Docker Socket: %s", lines[0])
	}
	return lines[0], nil
}

// use the serial port socket to ask what the VM's host only IP is
func RequestIPFromSerialPort(socket string) (string, error) {
	c, err := net.Dial("unix", socket)

	if err != nil {
		return "", err
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
			return "", err
		}
		time.Sleep(1 * time.Second)
		buf := make([]byte, 1024)
		for {
			n, err := c.Read(buf[:])
			if err != nil {
				return "", err
			}
			line = line + string(buf[0:n])
			fullLog += string(buf[0:n])
			if strings.Contains(line, "\n") {
				//go looking for the string we want, and chomp line to after the \n
				if i := strings.IndexAny(line, "\n"); i != -1 {
					//     inet 10.180.1.3/16 brd 10.180.255.255 scope global wlan0
					inetRE := regexp.MustCompile(`^[\t ]*inet ([0-9.]*).*$`)
					if ip := inetRE.FindStringSubmatch(line[:i]); ip != nil {
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
		fmt.Printf(fullLog)
	}

	return IP, nil
}

// TODO: need to add or abstract to get a Serial coms version
// RequestCertsUsingSSH requests certs using SSH.
// The assumption is that if the certs are in b2d:/home/docker/.docker
// then the daemon is using TLS. We can't assume that because there are
// certs in the local host's user dir, that the server is using them, so
// for now, make sure things are updated from the server. (for `docker shellinit`)
func RequestCertsUsingSSH(m driver.Machine) (string, error) {
	cmd := getSSHCommand(m, SSHCommTarPems)

	certDir := ""

	b, err := cmd.Output()
	if err == nil {
		dir, err := cfgDir(".boot2docker")
		if err != nil {
			return "", err
		}

		certDir = filepath.Join(dir, "certs", m.GetName())

		// Open the tar archive for reading.
		r := bytes.NewReader(b)
		tr := tar.NewReader(r)

		// Iterate through the files in the archive.
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				// end of tar archive
				break
			}
			if err != nil {
				return "", err
			}
			filename := filepath.Base(hdr.Name)
			if err := os.MkdirAll(certDir, 0755); err != nil {
				return "", err
			}
			certFile := filepath.Join(certDir, filename)
			fmt.Fprintf(os.Stderr, "Writing %s\n", certFile)
			f, err := os.Create(certFile)
			if err != nil {
				return "", err
			}
			w := bufio.NewWriter(f)
			if _, err := io.Copy(w, tr); err != nil {
				return "", err
			}
			w.Flush()
		}
	}
	return certDir, nil
}

func getDaemonArgumentsUsingSSH(m driver.Machine) ([]string, error) {
	cmd := getSSHCommand(m, SSHCommDaemonArgs)

	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	out := string(b)
	if B2D.Verbose {
		fmt.Printf("SSH returned: %s\nEND SSH\n", out)
	}
	return strings.Split(out, " "), nil
}

func RequestTLSUsingSSH(m driver.Machine) (bool, error) {
	args, err := getDaemonArgumentsUsingSSH(m)
	if err != nil {
		return false, err
	}

	for _, a := range args {
		if a == "--tlsverify" || a == "--tls" {
			return true, nil
		}
	}
	return false, nil
}
