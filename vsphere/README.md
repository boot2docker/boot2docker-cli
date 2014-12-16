Boot2docker vSphere Driver
==========================

The vSphere driver is to support running vSphere environment.

vSphere Environment Requirement
---------------

The vSphere environment requires DHCP on the VM network the boot2docker VM is running on.


Configuration
---------------

The boot2docker reads the driver information from its profile, and a sample snippet configuration is provided below:

```ini
# boot2docker profile filename: /Users/my/.boot2docker/profile
......
Driver = "vsphere"
......

[DriverCfg.vsphere]
# path to the govc binary
Govc = "govc"

# vCenter IP address
VcenterIp = "10.150.100.200"

# vCenter Username (console should prompt for password)
VcenterUser = "root"

# target datacenter to deploy the boot2docker virtual machine
VcenterDatacenter = "Datacenter"

# target datastore to upload the boot2docker ISO and store the boot2docker virtual machine
VcenterDatastore = "datastore1"

# target network to add the boot2docker virtual machine (requires DHCP)
VcenterNetwork = "VM Network"

# (optional) required when user want to deploy to a specified host or multiple clusters/hosts exist in the environment
VcenterHostIp = "10.120.180.160"

# (optional) required when user wants to deploy to a specified cluster or multiple clusters/hosts exist in this environment
VcenterPool = "cluster"

# (optional) the default vm CPU number is 2
VmCPU = 4
```

