// +build windows

package main

import (
	"os"
	"path/filepath"
)

func getCfgDir() (string, error) {
	if b2dDir := os.Getenv("BOOT2DOCKER_CFG_DIR"); b2dDir != "" {
		return b2dDir, nil
	}

	name := "boot2docker"
	
	for _, env := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"USERPROFILE", // let's try USERPROFILE only as a very last resort
	} {
		if val := os.Getenv(env); val != "" {
			return filepath.Join(val, name), nil
		}
	}
	
	// ok, we've tried everything reasonable - now let's go for CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, name), nil
}
