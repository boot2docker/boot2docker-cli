package vsphere

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/boot2docker/boot2docker-cli/vsphere/errors"
)

func init() {
}

func govc(args ...string) error {
	err := lookPath(cfg.Govc)
	if err != nil {
		return errors.NewGovcNotFoundError(cfg.Govc)
	}

	cmd := exec.Command(cfg.Govc, args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", cfg.Govc, strings.Join(args, " "))
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func govcOutErr(args ...string) (string, string, error) {
	err := lookPath(cfg.Govc)
	if err != nil {
		return "", "", errors.NewGovcNotFoundError(cfg.Govc)
	}

	cmd := exec.Command(cfg.Govc, args...)
	if verbose {
		log.Printf("executing: %v %v", cfg.Govc, strings.Join(args, " "))
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

func lookPath(file string) error {
	_, err := exec.LookPath(file)
	return err
}
