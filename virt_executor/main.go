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
	"fmt"

	mesosexec "github.com/mesos/mesos-go/executor"
	mesos "github.com/mesos/mesos-go/mesosproto"
)
import "github.com/rgbkrk/libvirt-go"
import "flag"
import "io/ioutil"
import "strings"
import "strconv"
import "os/exec"
import "os"
import "bufio"
import "github.com/satori/go.uuid"

var (
	hostname                    = flag.String("h", "", "VM Hostname")
	mac                         = flag.String("mac", "52:54:00:a8:fe:69", "MAC Address")
	component_type              = flag.String("ct", "", "Component Type")
	fqdn                        = flag.String("f", "phonepe.int", "FQDN")
	cpu                         = flag.Int("c", 1, "VCPUs")
	mem                         = flag.Int("m", 1, "Mem")
	local_pmc_dir               = "/var/local/pmc"
	cloud_local_ds              = "/usr/bin/cloud-localds"
	host_image_location         = "/var/lib/libvirt/images"
	kernel_mesos                = "data/trusty-server-cloudimg-amd64-vmlinuz-generic"
	kernel                      = "/var/local/pmc/trusty-server-cloudimg-amd64-vmlinuz-generic"
	cloud_init_mesos            = "data/cloud-init.goLang"
	cloud_init                  = "/etc/default/cloud-init.goLang"
	original_source_image_mesos = "data/trusty.ORIG.img"
	original_source_image       = "/var/local/trusty.ORIG.img"
	virt_template_mesos         = "data/PMCLibvirtTemplate.xml"
	virt_template               = "/etc/default/PMCLibvirtTemplate.xml"
)

func resolvConfignImages() error {

	this_id := os.Getuid()
	fmt.Println("Running as id", this_id)
	kvs := map[string]string{
		kernel_mesos:                kernel,
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
		cmd := exec.Command("cp", config_mesosfile, config_virtfile)
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

func init() {
	flag.Parse()
}

type exampleExecutor struct {
	tasksLaunched int
}

func newExampleExecutor() *exampleExecutor {
	return &exampleExecutor{tasksLaunched: 0}
}

func (mesosexec *exampleExecutor) Registered(driver mesosexec.ExecutorDriver, execInfo *mesos.ExecutorInfo, fwinfo *mesos.FrameworkInfo, slaveInfo *mesos.SlaveInfo) {
	fmt.Println("Registered Executor on slave ", slaveInfo.GetHostname())
}

func (mesosexec *exampleExecutor) Reregistered(driver mesosexec.ExecutorDriver, slaveInfo *mesos.SlaveInfo) {
	fmt.Println("Re-registered Executor on slave ", slaveInfo.GetHostname())
}

func (mesosexec *exampleExecutor) Disconnected(mesosexec.ExecutorDriver) {
	fmt.Println("Executor disconnected.")
}

func (mesosexec *exampleExecutor) LaunchTask(driver mesosexec.ExecutorDriver, taskInfo *mesos.TaskInfo) {
	fmt.Printf("Launching task %v with data [%#x]\n", taskInfo.GetName(), taskInfo.Data)

	runStatus := &mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesos.TaskState_TASK_RUNNING.Enum(),
	}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		fmt.Println("Got error", err)
	}

	mesosexec.tasksLaunched++
	fmt.Println("Total tasks launched ", mesosexec.tasksLaunched)
	//
	// this is where one would perform the requested task
	fmt.Println("AM RUNNING THE TASK NOW WITH HOSTNAME=", *hostname)
	r_err := resolvConfignImages()
	if r_err != nil {
		fmt.Println("Config and image problem:", r_err)
		os.Exit(1)
	}
	xml := GenXMLForDom()
	domxml := GenDomXML(xml)
	fmt.Println("Printing the DOMXML after replace")
	fmt.Println(domxml)
	conn, err := libvirt.NewVirConnection("qemu:///system")
	if err != nil {
		fmt.Println("Connection Error:", err)
		os.Exit(1)
	}

	doms, err := conn.ListAllDomains(libvirt.VIR_CONNECT_LIST_DOMAINS_PERSISTENT)
	if err != nil {
		fmt.Println("List Domains:", err)
		os.Exit(0)
	}
	for _, dom := range doms {
		name, _ := dom.GetName()
		fmt.Println(name)
	}
	dom, err := conn.DomainDefineXML(domxml)
	if err != nil {
		panic(err)
	}
	if err := dom.Create(); err != nil {
		fmt.Printf("Failed to create domain")
	}
	//
	if err := updateattrib("/etc/mesos-slave/attributes", *component_type); err != nil {
		fmt.Printf("Failed to Update attribute for ", *component_type)
	}

	// finish task
	fmt.Println("Finishing task", taskInfo.GetName())
	finStatus := &mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesos.TaskState_TASK_FINISHED.Enum(),
	}
	_, err = driver.SendStatusUpdate(finStatus)
	if err != nil {
		fmt.Println("Got error", err)
	}
	fmt.Println("Task finished", taskInfo.GetName())
}

func (mesosexec *exampleExecutor) KillTask(mesosexec.ExecutorDriver, *mesos.TaskID) {
	fmt.Println("Kill task")
}

func (mesosexec *exampleExecutor) FrameworkMessage(driver mesosexec.ExecutorDriver, msg string) {
	fmt.Println("Got framework message: ", msg)
}

func (mesosexec *exampleExecutor) Shutdown(mesosexec.ExecutorDriver) {
	fmt.Println("Shutting down the executor")
}

func (mesosexec *exampleExecutor) Error(driver mesosexec.ExecutorDriver, err string) {
	fmt.Println("Got error message:", err)
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("Starting Example Executor (Go)")

	dconfig := mesosexec.DriverConfig{
		Executor: newExampleExecutor(),
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

func GenDomXML(xml string) string {
	cloud_init_img := GenCloudInitConfig()
	host_img := GenHostImg()
	fmt.Println(cloud_init_img)
	xml = strings.Replace(xml, "__PMC__HOSTNAME__", *hostname, 1)
	uuid := fmt.Sprintf("%s", uuid.NewV4())
	xml = strings.Replace(xml, "__PMC__UUID__", uuid, 1)
	mem := getMem()
	xml = strings.Replace(xml, "__PMC__MEMORY__", mem, 2)
	xml = strings.Replace(xml, "__PMC__VCPU__", strconv.Itoa(*cpu), 1)
	xml = strings.Replace(xml, "__PMC__KERNEL__", kernel, 1)
	//cloud_init_img := GenCloudInitConfig(*hostname)
	xml = strings.Replace(xml, "__PMC__CLOUDINITIMAGE__", cloud_init_img, 1)
	xml = strings.Replace(xml, "__PMC__HOSTIMAGE__", host_img, 1)
	xml = strings.Replace(xml, "__PMC__MAC__", *mac, 1)
	return xml

}

func GenHostImg() string {
	image_loc := fmt.Sprintf("%s/%s.img", host_image_location, *hostname)
	Removefile(image_loc)
	cmd := exec.Command("cp", original_source_image, image_loc)
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return image_loc
}

func GenCloudInitConfig() string {
	dat, err := ioutil.ReadFile(cloud_init)
	if err != nil {
		fmt.Println("Error Reading cloud init file", cloud_init, err)
		os.Exit(1)
	}
	cloud_init_yaml := string(dat)
	cloud_init_yaml = strings.Replace(cloud_init_yaml, "__HOSTNAME__", *hostname, 1)
	cloud_init_yaml = strings.Replace(cloud_init_yaml, "__FQDN__", *fqdn, 1)
	d1 := []byte(cloud_init_yaml)
	ci_input := fmt.Sprintf("%s/%s", local_pmc_dir, *hostname)
	ci_input_img := fmt.Sprintf("%s/%s.img", local_pmc_dir, *hostname)
	Removefile(ci_input)
	Removefile(ci_input_img)
	err = ioutil.WriteFile(ci_input, d1, 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cmd := exec.Command(cloud_local_ds, ci_input_img, ci_input)
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

func getMem() string {
	m := *mem * 1024 * 1024
	return strconv.Itoa(m)
}
func GenXMLForDom() string {
	dat, err := ioutil.ReadFile(virt_template)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	//fmt.Print(string(dat))
	xmlstr := string(dat)
	return xmlstr

}

func getfields(path string) map[string]string {
	inFile, _ := os.Open(path)
	defer inFile.Close()
	scanner := bufio.NewScanner(inFile)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		fmt.Println("GOT HERE")
		fmt.Println(scanner.Text())
		fields := strings.Split(scanner.Text(), ",")
		fmt.Println(fields)
		ch := make(map[string]string)
		for _, field := range fields {
			if len(field) < 1 {
				break
			}
			fmt.Println(field)
			kvs := strings.Split(field, ":")
			k := kvs[0]
			v := kvs[1]
			fmt.Println("0=", k, "1=", v)
			ch[k] = v
			fmt.Println(ch)
			fmt.Println("GOT HERE - BREAK")
		}
		return ch
	}
	return nil
}

func updateattrib(path string, attrib string) error {
	if attrib == "" {
		return nil
	}
	kvals := getfields(path)
	fmt.Println(kvals)
	kvalsn := updatefield(&kvals, attrib)
	fmt.Println(kvalsn, attrib)
	err := writefields(path, &kvalsn)
	return err
}

func writefields(f string, kvpw *map[string]string) error {
	kvp := *kvpw
	var writestr = ""
	for k, v := range kvp {
		s := fmt.Sprintf("%v:%v,", k, v)
		writestr += s
	}
	writestr += "\n"
	fmt.Println("WRITING STRING ", writestr)
	d1 := []byte(writestr)
	err := ioutil.WriteFile(f, d1, 0644)
	return err
}

func updatefield(kvpp *map[string]string, attrib string) map[string]string {
	kvp := *kvpp
	if len(kvp) == 0 {
		kvp = make(map[string]string)
	}
	i, ok := kvp[attrib]
	var attribval = 0
	if !ok {
		attribval = 0
	} else {
		attribval_i, err := strconv.Atoi(i)
		if err != nil {
			fmt.Println("Adding to attrib failed", attrib, err)
		}
		attribval = attribval_i

	}
	attribval += 1
	kvp[attrib] = strconv.Itoa(attribval)
	return kvp
}
