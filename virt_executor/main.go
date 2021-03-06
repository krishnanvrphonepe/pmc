/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"fmt"
	mesosexec "github.com/mesos/mesos-go/executor"
	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/rgbkrk/libvirt-go"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
)

var (
	hostname                    = flag.String("h", "", "VM Hostname")
	mac                         = flag.String("mac", "52:54:00:a8:fe:69", "MAC Address")
	component_type              = flag.String("ct", "", "Component Type")
	fqdn                        = flag.String("f", "phonepe.int", "FQDN")
	osv                          = flag.String("o", "trusty", "trusty/xenial")
	cpu                         = flag.Uint("C", 1, "VCPUs")
	mem                         = flag.Uint64("M", 1024, "Mem")
	local_pmc_dir               = "/var/local/pmc"
	cloud_local_ds              = "/usr/bin/cloud-localds"
	host_image_location         = "/opt/var/lib/libvirt/images"
	cloud_init_mesos            = "data/cloud-init.goLang.__OS_VERSION__"
	cloud_init                  = "/etc/default/cloud-init.goLang.__OS_VERSION__"
	virt_template_mesos         = "data/PMCLibvirtTemplate.xml"
	virt_template               = "/etc/default/PMCLibvirtTemplate-__OS_VERSION__.xml"
	AttribSeparator             = ";"
	initrd_mesos                = "data/__OS_VERSION__-server-cloudimg-amd64-initrd-generic"
	initrd                      = "/opt/var/lib/libvirt/images/__OS_VERSION__-server-cloudimg-amd64-initrd-generic"
	kernel_mesos                = "data/__OS_VERSION__-server-cloudimg-amd64-vmlinuz-generic"
	kernel                      = "/opt/var/lib/libvirt/images/__OS_VERSION__-server-cloudimg-amd64-vmlinuz-generic"
	original_source_image_mesos = "data/__OS_VERSION__.ORIG.img"
	original_source_image       = "/opt/var/lib/libvirt/images/__OS_VERSION__-server-cloudimg-amd64.img"
)

type virtExecutorImpl struct {
	Hostname       string
	MACAddress     string
	ComponentType  string
	Fqdn           string
	Cpu            uint
	Mem            uint64
	Local_pmc_dir  string
	Cloud_local_ds string
	HostImgLoc     string
	KernelLoc      string
	InitrdLoc      string
	CloudInitLoc   string
	OriginalImg    string
	VirtTemplate   string
	virtconn       *libvirt.VirConnection
}

type virtExecutor struct {
	virtconn *libvirt.VirConnection
}

func newVirtExecutor() *virtExecutor {
	conn, err := libvirt.NewVirConnection("qemu:///system")
	if err != nil {
		fmt.Println("Connection Error:", err)
		os.Exit(1)
	}
	return &virtExecutor{
		virtconn: &conn,
	}
}

func init() {
	flag.Parse()

	// Make sure we get the right values based on the OS version ( trusty/xenial) 
	// These are pushed by salt pmc

	fmt.Println("Before",virt_template,initrd_mesos,initrd,kernel_mesos,kernel,original_source_image_mesos,original_source_image)
	virt_template = strings.Replace(virt_template, "__OS_VERSION__", *osv, 1)
	initrd_mesos = strings.Replace(initrd_mesos, "__OS_VERSION__", *osv, 1)
	initrd = strings.Replace(initrd, "__OS_VERSION__", *osv, 1)
	kernel_mesos = strings.Replace(kernel_mesos, "__OS_VERSION__", *osv, 1)
	kernel = strings.Replace(kernel, "__OS_VERSION__", *osv, 1)
	original_source_image_mesos = strings.Replace(original_source_image_mesos, "__OS_VERSION__", *osv, 1)
	original_source_image = strings.Replace(original_source_image, "__OS_VERSION__", *osv, 1)
	cloud_init = strings.Replace(cloud_init, "__OS_VERSION__", *osv, 1)
	cloud_init_mesos = strings.Replace(cloud_init_mesos, "__OS_VERSION__", *osv, 1)
	fmt.Println("After",virt_template,initrd_mesos,initrd,kernel_mesos,kernel,original_source_image_mesos,original_source_image,cloud_init,cloud_init_mesos)

	r_err := resolvConfignImages()
	if r_err != nil {
		fmt.Println("Config and image problem:", r_err)
		os.Exit(1)
	}
}

func (mesosexec *virtExecutor) Registered(driver mesosexec.ExecutorDriver, execInfo *mesos.ExecutorInfo, fwinfo *mesos.FrameworkInfo, slaveInfo *mesos.SlaveInfo) {
	fmt.Println("Registered Executor on slave ", slaveInfo.GetHostname())
}

func (mesosexec *virtExecutor) Reregistered(driver mesosexec.ExecutorDriver, slaveInfo *mesos.SlaveInfo) {
	fmt.Println("Re-registered Executor on slave ", slaveInfo.GetHostname())
}

func (mesosexec *virtExecutor) Disconnected(mesosexec.ExecutorDriver) {
	fmt.Println("Executor disconnected.")
	mesosexec.virtconn.CloseConnection()
}

func (mesosexec *virtExecutor) LaunchTask(driver mesosexec.ExecutorDriver, taskInfo *mesos.TaskInfo) {
	fmt.Printf("Launching task %v with data [%#x]\n", taskInfo.GetName(), taskInfo.Data)

	fmt.Println("Check if VM Already exists")
	vExec := newVirtExecutorImpl(mesosexec.virtconn)
	vmexists := vExec.CheckVMExists()
	if vmexists == nil {
		fmt.Println("VM Already Present ... NOOP")
		/*
			runStatus := &mesos.TaskStatus{TaskId: taskInfo.GetTaskId(), State: mesos.TaskState_TASK_FINISHED.Enum()}
			_, err := driver.SendStatusUpdate(runStatus) // This ensures the resource is not held forever
			if err != nil {
				fmt.Println("Got error", err)
			}
		*/
	} else {
		fmt.Println("Attempting to Create a new VM: ", *hostname)
		vExec.CreateVM()
		vmexists := vExec.CheckVMExists()
		if vmexists == nil {
			fmt.Println("VM has been created, exists now")
		}
	}

	runStatus := &mesos.TaskStatus{TaskId: taskInfo.GetTaskId(), State: mesos.TaskState_TASK_RUNNING.Enum()}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		fmt.Println("Got error", err)
	}
	fmt.Println("Task finished", taskInfo.GetName())
}

func (mesosexec *virtExecutor) KillTask(mesosexec.ExecutorDriver, *mesos.TaskID) {
	fmt.Println("Kill task")
}

func (mesosexec *virtExecutor) FrameworkMessage(driver mesosexec.ExecutorDriver, msg string) {
	fmt.Println("Got framework message: ", msg)
}

func (mesosexec *virtExecutor) Shutdown(mesosexec.ExecutorDriver) {
	fmt.Println("Shutting down the executor")
}

func (mesosexec *virtExecutor) Error(driver mesosexec.ExecutorDriver, err string) {
	fmt.Println("Got error message:", err)
}

func main() {
	fmt.Println("Starting Example Executor (Go)")

	dconfig := mesosexec.DriverConfig{
		Executor: newVirtExecutor(),
	}
	driver, err := mesosexec.NewMesosExecutorDriver(dconfig)

	if err != nil {
		fmt.Println("Unable to create a ExecutorDriver ", err.Error())
	}

	_, err = driver.Start()
	if err != nil {
		fmt.Println("Got error:", err)
		return
	}
	fmt.Println("Executor process has started and running.")
	driver.Join()
}

func newVirtExecutorImpl(c *libvirt.VirConnection) *virtExecutorImpl {

	_, err := os.Stat(local_pmc_dir)
	if os.IsNotExist(err) {
		os.Mkdir(local_pmc_dir, 0755)
	}


	return &virtExecutorImpl{
		Hostname:       *hostname,
		MACAddress:     *mac,
		ComponentType:  *component_type,
		Fqdn:           *fqdn,
		Cpu:            *cpu,
		Mem:            *mem,
		Local_pmc_dir:  local_pmc_dir,
		Cloud_local_ds: cloud_local_ds,
		HostImgLoc:     host_image_location,
		KernelLoc:      kernel,
		InitrdLoc:      initrd,
		CloudInitLoc:   cloud_init,
		OriginalImg:    original_source_image,
		VirtTemplate:   virt_template,
		virtconn:       c,
	}
}

func (vE *virtExecutorImpl) CheckVMExists() error {

	conn := vE.virtconn
	err_d := checkdomainexists(conn, vE.Hostname)
	if err_d == nil {
		return err_d
	}
	_, err := net.Dial("tcp", vE.Hostname+":22")
	if err == nil {
		fmt.Printf("FATAL: %v: Host is sshable - elsewhere", vE.Hostname)
		os.Exit(1)

	}
	return fmt.Errorf("NOT FOUND: %v", vE.Hostname)
}

func checkdomainexists(conn *libvirt.VirConnection, h string) error {
	doms, err := conn.ListAllDomains(libvirt.VIR_CONNECT_LIST_DOMAINS_PERSISTENT)
	if err != nil {
		fmt.Println("List Domains:", err)
		os.Exit(0)
	}
	for _, dom := range doms {
		name, _ := dom.GetName()
		fmt.Println(name)
		if name == h {
			if af, auto_err := dom.GetAutostart(); auto_err == nil {
				if af == false {
					if sauto_err := dom.SetAutostart(true); sauto_err != nil {
						fmt.Printf("Failed to set AUTOSTART\n")
					} else {
						fmt.Printf("SUCCESS: set AUTOSTART\n")
					}
				}
			} else {
				fmt.Printf("Failed to get AUTOSTART STATUS\n")
			}
			return nil
		}
	}
	return fmt.Errorf("NOT FOUND: %v", h)
}

func (vE *virtExecutorImpl) CreateVM() {
	domxml := vE.GenDomXML()
	conn := vE.virtconn
	dom, err := conn.DomainDefineXML(domxml)
	if err != nil {
		panic(err)
	}
	if err := dom.Create(); err != nil {
		fmt.Printf("Failed to create domain")
		os.Exit(1)
	}
	if auto_err := dom.SetAutostart(true); auto_err != nil {
		fmt.Printf("Failed to set AUTOSTART\n")
	}
}

func (vE *virtExecutorImpl) GenDomXML() string {
	xml := GenXMLForDom(vE.VirtTemplate)
	xml = strings.Replace(xml, "__PMC__HOSTNAME__", vE.Hostname, 1)
	uuid := fmt.Sprintf("%s", uuid.NewV4())
	xml = strings.Replace(xml, "__PMC__UUID__", uuid, 1)
	mem := getMem(vE.Mem)
	xml = strings.Replace(xml, "__PMC__MEMORY__", mem, 2)
	xml = strings.Replace(xml, "__PMC__VCPU__", fmt.Sprintf("%v", vE.Cpu), 1)
	xml = strings.Replace(xml, "__PMC__KERNEL__", vE.KernelLoc, 1)
	xml = strings.Replace(xml, "__PMC__INITRD__", vE.InitrdLoc, 1)

	cloud_init_img := vE.GenCloudInitConfig()
	xml = strings.Replace(xml, "__PMC__CLOUDINITIMAGE__", cloud_init_img, 1)

	host_img := vE.GenHostImg()
	xml = strings.Replace(xml, "__PMC__HOSTIMAGE__", host_img, 1)

	xml = strings.Replace(xml, "__PMC__MAC__", vE.MACAddress, 1)
	return xml

}

func (vE *virtExecutorImpl) GenHostImg() string {
	image_loc := fmt.Sprintf("%s/%s.img", vE.HostImgLoc, vE.Hostname)
	Removefile(image_loc)
	cmd := exec.Command("cp", vE.OriginalImg, image_loc)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return image_loc
}

func (vE *virtExecutorImpl) GenCloudInitConfig() string {
	dat, err := ioutil.ReadFile(vE.CloudInitLoc)
	if err != nil {
		fmt.Println("Error Reading cloud init file", vE.CloudInitLoc, err)
		os.Exit(1)
	}
	cloud_init_yaml := string(dat)
	cloud_init_yaml = strings.Replace(cloud_init_yaml, "__HOSTNAME__", vE.Hostname, 1)
	cloud_init_yaml = strings.Replace(cloud_init_yaml, "__FQDN__", vE.Fqdn, 1)
	d1 := []byte(cloud_init_yaml)
	ci_input := fmt.Sprintf("%s/%s", vE.Local_pmc_dir, vE.Hostname)
	ci_input_img := fmt.Sprintf("%s/%s.img", vE.Local_pmc_dir, vE.Hostname)
	Removefile(ci_input)
	Removefile(ci_input_img)
	err = ioutil.WriteFile(ci_input, d1, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cmd := exec.Command(vE.Cloud_local_ds, ci_input_img, ci_input)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return ci_input_img

}

func Removefile(f string) {
	if _, err := os.Stat(f); err == nil {
		err = os.Remove(f)
		if err != nil {
			fmt.Println("DELETION FAILED", f, err)
			os.Exit(1)
		}
	}
}

func getMem(mem uint64) string {
	m := mem * 1024
	return fmt.Sprintf("%v", m)
}
func GenXMLForDom(virt_template string) string {
	dat, err := ioutil.ReadFile(virt_template)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//fmt.Print(string(dat))
	xmlstr := string(dat)
	return xmlstr

}

func resolvConfignImages() error {

	this_id := os.Getuid()
	fmt.Println("Running as id", this_id)
	kvs := map[string]string{
		kernel_mesos:                kernel,
		initrd_mesos:                initrd,
		cloud_init_mesos:            cloud_init,
		original_source_image_mesos: original_source_image,
		virt_template_mesos:         virt_template}
	for k, v := range kvs {
		err := copyIfDNE(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func copyIfDNE(config_mesosfile string, config_virtfile string) error {
	//if a new config is to be passed, it will overwrite existing config
	if _, err := os.Stat(config_mesosfile); err == nil {
		cmd := exec.Command("mv", config_mesosfile, config_virtfile)
		fmt.Println(cmd)
		if err := cmd.Run(); err != nil {
			cp_err := fmt.Errorf("Got error with File %s -> %s, : %s", config_mesosfile, config_virtfile, os.Stderr)
			return cp_err
		}
	}
	if _, err := os.Stat(config_virtfile); os.IsNotExist(err) {
		return err
	}
	return nil
}
