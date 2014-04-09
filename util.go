package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
func cmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if B2D.Verbose {
		logf("executing: %v %v", name, strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if name == B2D.SSH {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// Convenient function to exec a command.
func cmdInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if B2D.Verbose {
		logf("executing: %v %v", name, strings.Join(args, " "))
	}
	return cmd.Run()
}
