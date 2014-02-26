package main

import (
	"encoding/json"
	"fmt"
	"github.com/vaughan0/go-ini"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func logf(fmt string, v ...interface{}) {
	log.Printf(fmt, v...)
}

// Return the value of an ENV var, or the fallback value if the ENV var is empty/undefined.
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Check if the connection to tcp://addr is readable.
func read(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.Read(make([]byte, 1)); err != nil {
		return err
	}
	return nil
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
