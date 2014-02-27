# boot2docker-cli

This is the Go port of boot2docker (https://github.com/boot2docker/boot2docker)
management script. It is intended to replace the shell script eventually. It is
currently usable but since it is under active development, frequent changes and
bugs are expected. USE AT YOUR OWN RISK. 

The Go port will produce a single binary without extra dependencies for the
following platforms:

- Linux/386
- Linux/amd64
- OS X (Darwin)/386
- OS X (Darwin)/amd64
- Windows/386
- Windows/amd64


## Installation

Once you have your `$GOPATH` properly setup, run

    go get github.com/boot2docker/boot2docker-cli/boot2docker


The binary will be available at `$GOPATH/bin/boot2docker`.

If you don't want to install golang you can use the `Dockerfile` to create the
binary for any supported target platform.

1. Build the image with the Go toolchain: `docker build -t boot2docker-golang .`
2. Choose the right settings for your plattform
  * Windows: `GOOS=windows GOARCH=amd64`
  * OS X: `GOOS=darwin GOARCH=amd64`
  * Linux: `GOOS=linux GOARCH=amd64`
3. Build the binaries
```
docker run -e GOOS=darwin -e GOARCH=amd64 --name boot2docker-cli boot2docker-golang
docker cp boot2docker-cli:/data/boot2docker-cli .
docker rm boot2docker-cli
```

The binary `boot2docker-cli` will be in your current folder.
Please do not forget to rename the binary on Windows to `boot2docker-cli.exe`


## Usage

To initialize a new boot2docker VM, run

    boot2docker-cli init

Then you can start the VM by

    boot2docker-cli up

To stop the VM, run

    boot2docker-cli down

And finally if you don't need the VM anymore, run

    boot2docker-cli delete

to remove it completely. 


## Configuration

The `boot2docker-cli` binary reads configuration from the environment and a configuration file. Currently you can configure the following options:

- `BOOT2DOCKER_CFG_DIR` path to the directory with all tool related files (custom config, ISO, persistent hard disk), default ${HOME}/.boot2docker
- `BOOT2DOCKER_PROFILE` path to custom config file, default ${HOME}/.boot2docker/profile
- `VBM` path to the VirtualBox management utility, default is
  `VBoxManage`.
- `BOOT2DOCKER_SSH` path to the `ssh` client utility, default is `ssh`.
- `VM_NAME` name of the boot2docker virtual machine in VirtualBox,
  default is `boot2docker-vm`.
- `BOOT2DOCKER_ISO` path to the boot2docker ISO image, default is
  `$BOOT2DOCKER_DIR/boot2docker.iso`.
- `VM_DISK` path to the boot2docker disk image, default is
  `$BOOT2DOCKER_CFG_DIR/boot2docker.vmdk`.
- `VM_DISK_SIZE` boot2docker disk image size in MB, default is `20000`.
- `VM_MEM` boot2docker VM memory size in MB, default is `1024`.
- `SSH_HOST_PORT` port on the host forwarding to port 22 in boot2docker
  VM, default is `2022`.
- `DOCKER_PORT` port on the host forwarding to port 4243 in
  boot2docker VM, default is `4243`.

You can create a custom config `${HOME}/.boot2docker/profile` or `%AllUsersProfile%\boot2docker\profile`:

    VBM=%ProgramFiles%\Oracle\VirtualBox\VBoxManage.exe
    BOOT2DOCKER_SSH=%ProgramFiles%\putty\putty.exe

You can put custom options into your shell enviroment which will overwrite the default values and the values from an existing config file:

Unix:
    export BOOT2DOCKER_CFG_DIR="${HOME}/Library/Application\ Support/boot2docker"
    export DOCKER_PORT=4244

Windows:
    set VBM=%ProgramFiles%\Oracle\VirtualBox\VBoxManage.exe
    set BOOT2DOCKER_SSH=%ProgramFiles%\putty\putty.exe


**What is the development process**

We are implementing the same process as [Docker merge approval](https://github.com/dotcloud/docker/blob/master/CONTRIBUTING.md#merge-approval), so all commits need to be done via pull requests, and will need 3 or more LGTMs.
