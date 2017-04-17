## ToDos to replace the shell script:

### Configuration
- [X] modify standard values with enviroment variables
- [ ] modify standard values with a config file `~/profiles`
- [X] check if all required software exists e.g. virtualbox,..

### Commands
- [X] init            Create a new boot2docker VM.
- [X] up|start|boot   Start the VM from any state.
- [X] save|suspend    Suspend the VM (saving running state to disk).
- [X] down|stop|halt  Gracefully shutdown the VM.
- [X] restart         Gracefully reboot the VM.
- [X] poweroff        Forcefully shutdown the VM (might cause disk corruption).
- [X] reset           Forcefully reboot the VM (might cause disk corruption).
- [X] delete          Delete the boot2docker VM and its disk image.
- [X] download        Download the boot2docker ISO image.
- [X] info            Display the detailed information of the VM
- [X] status          Display the current state of the VM.

### Build
- [X] go get suport
- [X] build with Dockerfile
- [ ] Testcases
