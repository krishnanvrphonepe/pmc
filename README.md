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
$ mkdir /var/libvirt/hostdb # directory for hostdb. One json per host. Not configurable as of now ;)
$ ./pmc --master=<master:5050> --executor="/abs/path/to/exec.tgz" --logtostderr=true --address=<http_server> -q <beanstalkd_ip:port>
# The http server above is a server that hosts the exec. With this framework, it can be the IP address of the scheduler itself
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


#### Flow

scripts/create_host.pl -> Q -> scripts/update_dnsmasq -> Q -> mesos framework


#### Pending

* Identify duplicate requests for host creation and NOOP
* Make framework idempotent using hostdb, i.e., kill and restart framework as many times w/o causing adverse impact. 
* Handle deletes ( How to release mesos resource ? ) 
* Component type handling in baremetal host, i.e., "ct" attribute needs to be updated to reflect the instantaneous status, so as to get a better distribution. 

#### Open Questions

* How to apply attributes on the fly?
Egs: When i spin a vm say dev-nginx001 on a bremetal dev-baremetal001, I'd like to immediately tag dev-nginx001 as an attribute to dev-baremetal001. So, why do we need this  ? So when the framework dies, upon a restart, it'll fetch details of all existing hosts from a hostdb and accept offers for existing hosts from respective baremetals where there is an attribute match. This is to ensure the VM does not get recreated. 
Currently think of solving this with a perl program on the baremetal updating attributes and restarting mesos-slave as and when necessary. 

* How to kill a specific task ? 
If a duplicate request gets made for the same host, that task needs to be detected and a previous request for the same task should be killed. This ensures that it mesos does not reflect that VM as consuming twice the resources it actually does. 
Current thought is prevention of duplicate request is the best way. 
