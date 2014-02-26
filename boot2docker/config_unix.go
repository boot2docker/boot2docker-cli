// +build linux darwin freebsd

package main

import (
	"os"
	"path/filepath"
)

func getCfgDir() (string, error) {
	if b2dDir := os.Getenv("BOOT2DOCKER_CFG_DIR"); b2dDir != "" {
		return b2dDir, nil
	}

	name := ".boot2docker"
	
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, name), nil
	}

	// ok, we've tried everything reasonable - now let's go for CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, name), nil
}
