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

package scheduler

import (
	b64 "encoding/base64"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/kr/beanstalk"
	//"os"
	"strconv"
	"time"
	"io/ioutil"
	"encoding/json"
	log "github.com/golang/glog"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"
)


var (
	HostDBDir      = "/var/libvirt/hostdb" 
) 

type ExampleScheduler struct {
	tasksLaunched int
	tasksFinished int
	totalTasks    int
	q             string
	uri           string
	mid           uint64
	is_new_host	bool
	Vm_input *VMInput
}

func NewExampleScheduler(q string, uri string) *ExampleScheduler {
	return &ExampleScheduler{
		tasksLaunched: 0,
		tasksFinished: 0,
		q:             q,
		uri:           uri,
	}
}

type VMInput struct {
	hostname  string
	mac       string
	cpu       float64
	mem       float64
	executor  string
	comp_type string
}
type VMInputJSON struct {
	Hostname  string `json:"hostname"`
	Mac       string `json:"mac"`
	Cpu       string `json:"cpu"`
	Mem       string `json:"mem"`
	Executor  string `json:"executor"`
	Comp_type string `json:"comp_type"`
}

func (sched *ExampleScheduler)  DeleteFromQ() {
	conn, e := beanstalk.Dial("tcp", sched.q)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	e = conn.Delete(sched.mid)
}

func (sched *ExampleScheduler) UpdateHostDB() {

	if(sched.is_new_host == false) {
		fmt.Println(">>>> EXISTING HOST ... No Hostdb update" ) 
		return
	}
		
	cpuval := strconv.FormatFloat(sched.Vm_input.cpu, 'f',-1,32)
	memval := strconv.FormatFloat(sched.Vm_input.mem, 'f',-1,32)
	v := &VMInputJSON{
		Hostname: sched.Vm_input.hostname,
		Mac:       sched.Vm_input.mac,
		Cpu:       cpuval,
		Mem:       memval,
		Executor:  sched.Vm_input.executor,
		Comp_type: sched.Vm_input.comp_type,
	}

	d,_ := json.Marshal(v)
	fmt.Println("Printing the struct:",v) 
	fmt.Println("Writing out:",string(d)) 
	fname := HostDBDir+"/"+sched.Vm_input.hostname 
	err := ioutil.WriteFile(fname, d, 0644)
	if err != nil {
		fmt.Println(fname,": Updattion of HOSTDB FAILED !!!!!!!!!!!!!!!!!") 
	}
		
}
func (sched *ExampleScheduler) FetchFromQ() {

	conn, e := beanstalk.Dial("tcp", sched.q)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	tubeSet := beanstalk.NewTubeSet(conn, "mesos")
	id, body, err := tubeSet.Reserve(10 * time.Hour)
	sched.mid = id
	if err != nil {
		panic(err)
	}
	str, err := b64.StdEncoding.DecodeString(string(body))
	if err != nil {
		fmt.Println("GOT ERROR", err)
	}

	var x VMInputJSON
	_ = json.Unmarshal(str, &x)
	fmt.Printf("Printing THE JSON UNMARSHAL %+v\n", x)
	cpuval, err := strconv.ParseFloat(x.Cpu, 64)
	memval, err := strconv.ParseFloat(x.Mem, 64)
	sched.Vm_input = &VMInput {
		hostname: x.Hostname,
		mac: x.Mac,
		executor: x.Executor,
		comp_type: x.Comp_type,
		cpu: cpuval,
		mem: memval,

	}
	sched.is_new_host = false

	fmt.Printf("PRINTING THE STRUCT %+v", sched.Vm_input)

}
func (sched *ExampleScheduler)  PrepareExecutorInfo() *mesos.ExecutorInfo {
	executorUris := []*mesos.CommandInfo_URI{
		{
			Value: &sched.uri,
			//Executable: proto.Bool(true),
		},
	}
	virt_cmd := "./virtmesos -h " + sched.Vm_input.hostname + " -mac " + sched.Vm_input.mac + " -ct " + sched.Vm_input.comp_type
	fmt.Println("Command to be exec: ", virt_cmd)
	//id := strconv.Itoa(sched.tasksLaunched) 
	return &mesos.ExecutorInfo{
		ExecutorId: util.NewExecutorID(sched.Vm_input.hostname),
		Name:       proto.String("kvm"),
		Source:     proto.String("virt_executor"),
		Command: &mesos.CommandInfo{
			Value: proto.String(virt_cmd),
			Uris:  executorUris,
			//Arguments: args,
		},
	}
}

func (sched *ExampleScheduler) Registered(driver sched.SchedulerDriver, frameworkId *mesos.FrameworkID, masterInfo *mesos.MasterInfo) {
	log.Infoln("Scheduler Registered with Master ", masterInfo)
}

func (sched *ExampleScheduler) Reregistered(driver sched.SchedulerDriver, masterInfo *mesos.MasterInfo) {
	log.Infoln("Scheduler Re-Registered with Master ", masterInfo)
}

func (sched *ExampleScheduler) Disconnected(sched.SchedulerDriver) {
	log.Infoln("Scheduler Disconnected")
}

func (sched *ExampleScheduler) ResourceOffers(driver sched.SchedulerDriver, offers []*mesos.Offer) {
	logOffers(offers)
	sched.FetchFromQ()
	exec := sched.PrepareExecutorInfo()
	attrib_arbitary_high := 100
	var chosen_offer *mesos.Offer

	var tasks []*mesos.TaskInfo
	for _, offer := range offers {
		remainingCpus := getOfferCpu(offer)
		remainingMems := getOfferMem(offer)

		if sched.Vm_input.cpu <= remainingCpus && sched.Vm_input.mem <= remainingMems {
			get_attrib_for_offer,vm_on_host := GetAttribVal(offer, sched.Vm_input.comp_type, sched.Vm_input.hostname)
			if vm_on_host == true {
				chosen_offer = offer
				fmt.Println(">>>>>>>>> VM already present, got the chosen offer") 
				break // thats it, this is it
			}
			if get_attrib_for_offer < attrib_arbitary_high {
				attrib_arbitary_high = get_attrib_for_offer
				chosen_offer = offer
				sched.is_new_host = true
			}
		}
	}

	if chosen_offer == nil {
		fmt.Println("NO OFFER MATCHED REQUIREMENT, RETURNING")
		sched.is_new_host = false
		return
	}
	fmt.Println(">>>>>>>>>> CHOSEN OFFER:\n",chosen_offer,"\n<<<<<<<<<<<<<<< CHOSEN OFFER")
	fmt.Println("\n>>>>>>>>>>>>>>>>>>> PRINT SCHEDULER INFO >>>>>>>>>>>>>>>>>>>>>>>>") 
	fmt.Printf("%+v\n%+v\n",sched,sched.Vm_input) 
	fmt.Println("\n>>>>>>>>>>>>>>>>>>> PRINT SCHEDULER INFO >>>>>>>>>>>>>>>>>>>>>>>>") 
	taskId := &mesos.TaskID{
		Value: proto.String(strconv.Itoa(sched.tasksLaunched)),
	}

	task := &mesos.TaskInfo{
		Name:     proto.String("kvm-(" + sched.Vm_input.hostname + ")-" + taskId.GetValue()),
		TaskId:   taskId,
		SlaveId:  chosen_offer.SlaveId,
		Executor: exec,
		Resources: []*mesos.Resource{
			util.NewScalarResource("cpus", sched.Vm_input.cpu),
			util.NewScalarResource("mem", sched.Vm_input.mem),
		},
	}
	log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), chosen_offer.Id.GetValue())

	tasks = append(tasks, task)
	log.Infoln("Launching ", len(tasks), "tasks for offer", chosen_offer.Id.GetValue())
	driver.LaunchTasks([]*mesos.OfferID{chosen_offer.Id}, tasks, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
	sched.tasksLaunched++
	return
}

func (sched *ExampleScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())
	fmt.Printf("%+v\n",status)
	sched.DeleteFromQ()
	if "TASK_RUNNING" == status.State.Enum().String() {
		fmt.Println(sched.Vm_input.hostname," has been started Succesfully, exiting")
		sched.UpdateHostDB()
		//os.Exit(0)
	}

}

func (sched *ExampleScheduler) OfferRescinded(s sched.SchedulerDriver, id *mesos.OfferID) {
	log.Infof("Offer '%v' rescinded.\n", *id)
}

func (sched *ExampleScheduler) FrameworkMessage(s sched.SchedulerDriver, exId *mesos.ExecutorID, slvId *mesos.SlaveID, msg string) {
	log.Infof("Received framework message from executor '%v' on slave '%v': %s.\n", *exId, *slvId, msg)
}

func (sched *ExampleScheduler) SlaveLost(s sched.SchedulerDriver, id *mesos.SlaveID) {
	log.Infof("Slave '%v' lost.\n", *id)
}

func (sched *ExampleScheduler) ExecutorLost(s sched.SchedulerDriver, exId *mesos.ExecutorID, slvId *mesos.SlaveID, i int) {
	log.Infof("Executor '%v' lost on slave '%v' with exit code: %v.\n", *exId, *slvId, i)
}

func (sched *ExampleScheduler) Error(driver sched.SchedulerDriver, err string) {
	log.Infoln("Scheduler received error:", err)
}
