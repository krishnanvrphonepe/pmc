# mesos-framework-kvm in Go

### 1) Goal
PhonePe proposes to have a homegeneous environment consisting of docker services on baremetal, services that require isolation on kvm. The mesos framework will be used to automatically allocate an appropriate baremetal to kvm. This is as opposed to having to look at the entire setup holistically before spinning the VM on a baremetal.

### 2) Current State - WIP

### 3) Sample run

#### Get the template code

```sh
$ mkdir -p $GOPATH/src/github.com/
$ cd $GOPATH/src/github.com/
$ git clone git@github.com:krishnanvrphonepe/pmc.git
$ go get ./...
$ cd pmc
$ go build
$ cd virt_executor ; go build
$ cd ..
$ cp virt_executor/virt_executor virtmesos
$ mkdir data
$ find data/
data/
data/cloud-init.goLang
data/PMCLibvirtTemplate.xml
data/trusty-server-cloudimg-amd64-vmlinuz-generic  # http://cloud-images.ubuntu.com/trusty/current/
data/trusty.ORIG.img  # http://cloud-images.ubuntu.com/trusty/current/ <- whatever suits you
$ tar czvf exec.tgz virtmesos data
$ ./pmc --master=<master_ip>:5050 --executor="$GOPATH/src/github.com/krishnanvrphonepe/pmc/exec.tgz" --logtostderr=true --address=<master_ip>
```

####  Notes
o Mesos slaves can be baremetal and kvm, however, kvm can only be spun on baremtals. On baremetals, add attribute like below:

```sh
$ cat /etc/mesos-slave/attributes 
vt_enabled:1,
```
o In order that VMs of a specific component type don't all land on the same baremetal, and that they get evenly distributed, a component type attribute is also used, like for egs:

```sh
$ cat /etc/mesos-slave/attributes 
vt_enabled:1;ct_test:4
```
which means, there exist 4 VMs of the type c_test on this mesos slave, which is a baremetal. 

