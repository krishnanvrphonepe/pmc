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
	"encoding/json"
	"fmt"
	"github.com/gogo/protobuf/proto"
	log "github.com/golang/glog"
	"github.com/kr/beanstalk"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"
	"io/ioutil"
	"net"
	"os"
	//"math"
	"strconv"
	"strings"
	"time"
)

var (
	HostDBDir = "/var/libvirt/hostdb"
)

type ExampleScheduler struct {
	tasksLaunched         int
	tasksFinished         int
	totalTasks            int
	beanstalk_tube        *beanstalk.TubeSet
	beanstalk_tube_finish *beanstalk.Tube
	q                     *beanstalk.Conn
	uri                   string
	mid                   uint64
	is_new_host           bool
	Vm_input              *VMInput
	HostdbData            []string                     //json strings
	ctype_map             map[string]map[string]uint64 // a map of component type in each baremetal
	existing_hosts        map[string]bool
}

func NewExampleScheduler(q *beanstalk.Conn, uri string) *ExampleScheduler {
	return &ExampleScheduler{
		tasksLaunched:         0,
		tasksFinished:         0,
		uri:                   uri,
		q:                     q,
		beanstalk_tube:        beanstalk.NewTubeSet(q, "mesos"),
		beanstalk_tube_finish: &beanstalk.Tube{q, "mesos_finished"},
	}
}

type VMInput struct {
	hostname  string
	mac       string
	os        string
	cpu       uint
	mem       uint64
	executor  string
	comp_type string
	baremetal string
	maxc      int
}
type VMInputJSON struct {
	Hostname  string `json:"hostname"`
	Mac       string `json:"mac"`
	Cpu       string `json:"cpu"`
	Mem       string `json:"mem"`
	OS        string `json:"os"`
	Executor  string `json:"executor"`
	Comp_type string `json:"comp_type"`
	Baremetal string `json:"baremetal"`
	Maxc      string `json:"maxc"`
}

func (sched *ExampleScheduler) GetDataFromHostDB() {
	files, _ := ioutil.ReadDir(HostDBDir)
	var x []string
	sect := 2 
	timeOut := time.Duration(sect) * time.Second

	for _, f := range files {
		fn := HostDBDir + "/" + f.Name()
		data, _ := ioutil.ReadFile(fn)
		if len(data) < 10 {
			continue
		}
		hname := f.Name()
		log.Infoln(" Checking SSH", hname)
		_, herr := net.DialTimeout("tcp", hname+":22",timeOut)
		if herr != nil {
			x = append(x, string(data))
		} else {
			// At this point, verify if the host is already present, if it is then bail out

			log.Infoln( ": Host is already SSHABLE - Moving on")
			var x VMInputJSON
			_ = json.Unmarshal(data, &x)
			comp_type := x.Comp_type
			bm_for_host := x.Baremetal
			//fmt.Printf("Printing THE JSON UNMARSHAL %+v\n", x)
			// Poluplating stats
			if sched.ctype_map[bm_for_host][comp_type] == 0 { //go wtf
				if sched.ctype_map == nil {
					t1 := make(map[string]uint64)
					t1[comp_type] = 0
					sched.ctype_map = make(map[string]map[string]uint64)
					sched.ctype_map[bm_for_host] = t1
				}
				if sched.ctype_map[bm_for_host] == nil {
					sched.ctype_map[bm_for_host] = make(map[string]uint64)
					sched.ctype_map[bm_for_host][comp_type] = 0

				}

			}
			sched.ctype_map[bm_for_host][comp_type]++
			sched.existing_hosts = make(map[string]bool)
			sched.existing_hosts[hname] = true
		}
	}
	sched.HostdbData = x
}

func (sched *ExampleScheduler) DeleteFromQ() {
	fmt.Println("DELETING", sched.mid)
	sched.q.Delete(sched.mid)
}

func (sched *ExampleScheduler) ReleaseFromQ() {
	fmt.Println("Releasing", sched.mid)
	sched.q.Release(sched.mid, 10, 5*time.Second)
}

func (sched *ExampleScheduler) UpdateFinishQ(s string) {
	id, err := sched.beanstalk_tube_finish.Put([]byte(s), 1, 0, time.Minute)
	if err != nil {
		panic(err)
	}
	fmt.Println("job", id)
}

func (sched *ExampleScheduler) UpdateHostDB() *string {

	cpuval := fmt.Sprintf("%v", sched.Vm_input.cpu)
	memval := fmt.Sprintf("%v", sched.Vm_input.mem)
	v := &VMInputJSON{
		Hostname:  sched.Vm_input.hostname,
		Mac:       sched.Vm_input.mac,
		OS:        sched.Vm_input.os,
		Cpu:       cpuval,
		Mem:       memval,
		Executor:  sched.Vm_input.executor,
		Comp_type: sched.Vm_input.comp_type,
		Baremetal: sched.Vm_input.baremetal,
	}

	d, _ := json.Marshal(v)
	log.Infoln("Printing the struct: %+v\n", v)
	log.Infoln("Writing out:", string(d))
	fname := HostDBDir + "/" + sched.Vm_input.hostname
	if sched.is_new_host == false {
		fmt.Println(">>>> EXISTING HOST ... No Hostdb update")
	} else {
		log.Infoln(">>>>>>>>>>>>>>>>>>>>  Writing to hostdb")
		err := ioutil.WriteFile(fname, d, 0644)
		if err != nil {
			fmt.Println(fname, ": Updattion of HOSTDB FAILED !!!!!!!!!!!!!!!!!")
		}
	}
	encoded := b64.StdEncoding.EncodeToString([]byte(d))
	return &encoded

}
func (sched *ExampleScheduler) FetchFromQ() {

	var str []byte
	// We first exhaust all hosts in the hostdb
	if len(sched.HostdbData) > 0 {
		//pop
		var strb string
		//strb, sched.HostdbData = sched.HostdbData[len(sched.HostdbData)-1], sched.HostdbData[:len(sched.HostdbData)-1]
		strb = sched.HostdbData[len(sched.HostdbData)-1]
		str = []byte(strb)
		sched.is_new_host = false
	} else {
		//tubeSet := beanstalk.NewTubeSet(sched.q, "mesos")
		id, body, err := sched.beanstalk_tube.Reserve(15 * time.Second)
		fmt.Println("GOT ID = ", id, "String:\n", string(body))
		if err != nil {
			return
		}
		sched.mid = id
		k := string(body)

		str, err = b64.StdEncoding.DecodeString(k)
		if err != nil {
			fmt.Println("GOT ERROR", err)
			os.Exit(0)
		} else {
			log.Infoln("Printing JSON STR\n", string(str))
		}
		sched.is_new_host = true
	}

	var x VMInputJSON
	x.Baremetal = ""
	_ = json.Unmarshal(str, &x)
	fmt.Printf("Printing THE JSON UNMARSHAL %+v\n", x)
	cpuval, _ := strconv.Atoi(x.Cpu)
	memval, _ := strconv.ParseUint(x.Mem, 10, 64)
	if x.Maxc == "" {
		x.Maxc = "1"
	}
	maxcval, _ := strconv.Atoi(x.Maxc)

	if sched.is_new_host == false {
		_, err := net.Dial("tcp", x.Hostname+":22")
		if err == nil {
			// gotta figure this out, this is just interim
			// cpuval = 1
			// memval = 1
		}
	}

	sched.Vm_input = &VMInput{
		hostname:  x.Hostname,
		mac:       x.Mac,
		os:        x.OS,
		executor:  x.Executor,
		comp_type: x.Comp_type,
		cpu:       uint(cpuval),
		mem:       memval,
		baremetal: x.Baremetal,
		maxc:      maxcval,
	}
	log.Infof("PRINTING THE STRUCT %+v", sched.Vm_input)

}
func (sched *ExampleScheduler) PrepareExecutorInfo() *mesos.ExecutorInfo {
	executorUris := []*mesos.CommandInfo_URI{
		{
			Value: &sched.uri,
			//Executable: proto.Bool(true),
		},
	}
	virt_cmd := "./virtmesos -h " + sched.Vm_input.hostname + " -mac " + sched.Vm_input.mac + " -ct " + sched.Vm_input.comp_type + " -C " + fmt.Sprintf("%v", sched.Vm_input.cpu) + " -M " + fmt.Sprintf("%v", sched.Vm_input.mem) + " -o " + sched.Vm_input.os
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
	var attrib_arbitary_high uint64
	var chosen_offer *mesos.Offer
	var bm_for_host string // There is no default bm for new hosts, so we use this string as a placeholder

	log.Infoln("\n\n>>>>>>>>>>>>>>>>>> CALLBACK BEGINS >>>>>>>>>>>>>>>>>>>>>>>>>>")
	defer log.Infoln(">>>>>>>>>>>>>>>>>> CALLBACK RETURNS  >>>>>>>>>>>>>>>>>>>>>>>>>>\n\n")
	logOffers(offers)
	log.Infof("\n\nPrinting the sched at entry: %+v\n\n", sched)
	sched.Vm_input = nil
	//log.Infof("VM_INPUT: %+v\n", sched.Vm_input)
	if sched.tasksLaunched == 0 {

		log.Infoln("FETCHING DATA FROM HOSTDB")
		sched.GetDataFromHostDB() //Be  Idempotent
	}
	//fmt.Println(sched)

	sched.FetchFromQ()
	if sched.Vm_input == nil { // Make sure this is not blocking
		for _, offer := range offers {
			driver.DeclineOffer(offer.Id, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
		}
		return
	}
	if sched.existing_hosts[sched.Vm_input.hostname] == true {
		log.Infof("HOST ALREADY EXISTS:\nVM_INPUT:\n %+v\n", sched.Vm_input)
		for _, offer := range offers {
			driver.DeclineOffer(offer.Id, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
		}
		sched.DeleteFromQ()
		return
	}

	exec := sched.PrepareExecutorInfo()
	attrib_arbitary_high = 100

	var tasks []*mesos.TaskInfo
	for _, offer := range offers {
		remainingCpus := getOfferCpu(offer)
		remainingMems := getOfferMem(offer)
		gotchosenoffer := false
		log.Infoln("Hostname = ", *offer.Hostname)
		if sched.Vm_input.baremetal == "" {
			bm_for_host = *offer.Hostname // New hosts from q
			log.Infoln("ASSIGNING BAREMETAL=", sched.Vm_input.baremetal)
		} else {
			bm_for_host = sched.Vm_input.baremetal // hosts from file
			log.Infoln("ALREADY PRESENT BAREMETAL=", sched.Vm_input.baremetal)
			if sched.Vm_input.baremetal == *offer.Hostname {
				chosen_offer = offer
				gotchosenoffer = true
				fmt.Println(">>>>>>>>> VM already present on ", sched.Vm_input.baremetal, " , got the chosen offer")
				break // thats it, this is it
			} else {
				continue
			}

		}
		log.Infof("CPU Required=%v, Mem Required = %v, RemCPUS = %v, RemMem=%v\n", sched.Vm_input.cpu, sched.Vm_input.mem, uint(remainingCpus), uint64(remainingMems))

		if sched.Vm_input.cpu <= uint(remainingCpus) && sched.Vm_input.mem <= uint64(remainingMems) && gotchosenoffer == false {
			host_ok := GetAttribVal(offer)
			get_attrib_for_offer := sched.ctype_map[bm_for_host][sched.Vm_input.comp_type]
			log.Infoln("\nATTRIB FOR OFFER:", get_attrib_for_offer, "\n")
			if (host_ok == true) && (get_attrib_for_offer < attrib_arbitary_high) && (get_attrib_for_offer < uint64(sched.Vm_input.maxc)) {
				attrib_arbitary_high = get_attrib_for_offer
				chosen_offer = offer
				gotchosenoffer = true
				if attrib_arbitary_high == 0 {
					break // why check more hosts ?
				}

			}
		}
	}

	// We need to decline all other offers, so we are presented with it at a later point
	if chosen_offer == nil {
		log.Infof("UNABLE TO GET A VALID OFFER:\nVM_INPUT:\n %+v\n", sched.Vm_input)
		sched.ReleaseFromQ()
		sched.existing_hosts[sched.Vm_input.hostname] = false
		for _, offer := range offers {
			driver.DeclineOffer(offer.Id, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
		}
		return
	}
	log.Infof(">>>>>>>>>> CHOSEN OFFER:  %v\n\n", chosen_offer)
	cv := chosen_offer.Id.GetValue()
	for _, offer := range offers {
		log.Infof("+++++++++++++++  Offer <%v> with cpus=%v mem=%v", offer.Id.GetValue(), getOfferCpu(offer), getOfferMem(offer))
		ov := offer.Id.GetValue()
		if strings.EqualFold(cv, ov) {
			log.Infoln("RETAINED", *offer.Hostname)
		} else {
			log.Infoln("DECLINED")
			driver.DeclineOffer(offer.Id, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
		}
	}
	log.Infoln("MAP VAL<<<<<<<<<<<<", bm_for_host, sched.Vm_input.comp_type, sched.ctype_map[bm_for_host][sched.Vm_input.comp_type], ">>>>>>>>>>>>\n")

	if sched.ctype_map[bm_for_host][sched.Vm_input.comp_type] == 0 { //go wtf
		if sched.ctype_map == nil {
			t1 := make(map[string]uint64)
			t1[sched.Vm_input.comp_type] = 0
			sched.ctype_map = make(map[string]map[string]uint64)
			sched.ctype_map[bm_for_host] = t1
		}
		if sched.ctype_map[bm_for_host] == nil {
			sched.ctype_map[bm_for_host] = make(map[string]uint64)
			sched.ctype_map[bm_for_host][sched.Vm_input.comp_type] = 0

		}

	}
	sched.ctype_map[bm_for_host][sched.Vm_input.comp_type]++
	sched.existing_hosts[sched.Vm_input.hostname] = true
	/*
		log.Infoln("\n\n\nMAP VAL\n>>>>>>>>>>>", sched.ctype_map[sched.Vm_input.baremetal][sched.Vm_input.comp_type], "\n\n\n\n>>>>>>>>>>>>\n")
		log.Infoln(">>>>>>>>>> CHOSEN OFFER:\n", chosen_offer, "\n<<<<<<<<<<<<<<< CHOSEN OFFER")
		log.Infoln("\n>>>>>>>>>>>>>>>>>>> PRINT SCHEDULER INFO >>>>>>>>>>>>>>>>>>>>>>>>")
		log.Infoln("%+v\n%+v\n", sched, sched.Vm_input)
		log.Infoln("\n>>>>>>>>>>>>>>>>>>> PRINT SCHEDULER INFO >>>>>>>>>>>>>>>>>>>>>>>>")
	*/

	taskId := &mesos.TaskID{
		Value: proto.String(strconv.Itoa(sched.tasksLaunched)),
	}

	task := &mesos.TaskInfo{
		Name:     proto.String("kvm-(" + sched.Vm_input.hostname + ")-" + taskId.GetValue()),
		TaskId:   taskId,
		SlaveId:  chosen_offer.SlaveId,
		Executor: exec,
		Resources: []*mesos.Resource{
			util.NewScalarResource("cpus", float64(sched.Vm_input.cpu)),
			util.NewScalarResource("mem", float64(sched.Vm_input.mem)),
		},
	}
	log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), chosen_offer.Id.GetValue())

	tasks = append(tasks, task)
	log.Infoln("Launching ", len(tasks), "tasks for offer", chosen_offer.Id.GetValue())
	fmt.Printf("CHOSEN OFFER: \n%+v\n", chosen_offer)
	sched.Vm_input.baremetal = *chosen_offer.Hostname
	encoded_str := sched.UpdateHostDB()
	if encoded_str != nil {
		sched.UpdateFinishQ(*encoded_str)
	}
	driver.LaunchTasks([]*mesos.OfferID{chosen_offer.Id}, tasks, &mesos.Filters{RefuseSeconds: proto.Float64(0)})
	sched.tasksLaunched++
	return
}

func (sched *ExampleScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())
	fmt.Printf("%+v\n", status)
	fmt.Printf("%+v\n", sched)
	fmt.Printf("VM_INPUT: %+v\n", sched.Vm_input)
	if "TASK_RUNNING" == status.State.Enum().String() {
		// Sometimes an offer with the specific baremetal is never made, so we need to keep retrying
		// Hence, shift the HostdbData iff the task is in TASK_RUNNING state
		if len(sched.HostdbData) > 0 {
			log.Infoln("Shifting Hostdbdata")
			sched.HostdbData = sched.HostdbData[:len(sched.HostdbData)-1]
		} else {
			sched.DeleteFromQ()
		}
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
