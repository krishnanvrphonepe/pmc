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

	"encoding/json"

	log "github.com/golang/glog"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"
)

type ExampleScheduler struct {
	tasksLaunched int
	tasksFinished int
	totalTasks    int
	q             string
	uri           string
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

func DeleteFromQ(q string, id uint64) {
	conn, e := beanstalk.Dial("tcp", q)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	e = conn.Delete(id)
}

func FetchFromQ(q string) (*VMInput, uint64) {
	conn, e := beanstalk.Dial("tcp", q)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	tubeSet := beanstalk.NewTubeSet(conn, "mesos")
	id, body, err := tubeSet.Reserve(10 * time.Hour)
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

	var ret VMInput
	ret.hostname = x.Hostname
	ret.mac = x.Mac
	ret.executor = x.Executor
	ret.comp_type = x.Comp_type
	ret.cpu, err = strconv.ParseFloat(x.Cpu, 64)
	ret.mem, err = strconv.ParseFloat(x.Mem, 64)
	fmt.Printf("PRINTING THE STRUCT %+v", ret)
	return &ret, id

}
func prepareExecutorInfo(uri string, m *VMInput) *mesos.ExecutorInfo {
	executorUris := []*mesos.CommandInfo_URI{
		{
			Value: &uri,
			//Executable: proto.Bool(true),
		},
	}
	virt_cmd := "./virtmesos -h " + m.hostname + " -mac " + m.mac + " -ct " + m.comp_type
	fmt.Println("Command to be exec: ", virt_cmd)
	return &mesos.ExecutorInfo{
		ExecutorId: util.NewExecutorID(m.hostname),
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
	m, mid := FetchFromQ(sched.q)
	exec := prepareExecutorInfo(sched.uri, m)
	attrib_arbitary_high := 100
	var chosen_offer *mesos.Offer

	var tasks []*mesos.TaskInfo
	for _, offer := range offers {
		remainingCpus := getOfferCpu(offer)
		remainingMems := getOfferMem(offer)

		if m.cpu <= remainingCpus && m.mem <= remainingMems {
			get_attrib_for_offer := GetAttribVal(offer, m.comp_type)
			if get_attrib_for_offer < attrib_arbitary_high {
				attrib_arbitary_high = get_attrib_for_offer
				chosen_offer = offer
			}
		}
	}

	if chosen_offer == nil {
		fmt.Println("NO OFFER MATCHED REQUIREMENT, RETURNING")
		return
	}
	fmt.Println(chosen_offer)
	taskId := &mesos.TaskID{
		Value: proto.String(strconv.Itoa(sched.tasksLaunched)),
	}

	task := &mesos.TaskInfo{
		Name:     proto.String("kvm-(" + m.hostname + ")" + taskId.GetValue()),
		TaskId:   taskId,
		SlaveId:  chosen_offer.SlaveId,
		Executor: exec,
		Resources: []*mesos.Resource{
			util.NewScalarResource("cpus", m.cpu),
			util.NewScalarResource("mem", m.mem),
		},
	}
	log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), chosen_offer.Id.GetValue())

	tasks = append(tasks, task)
	log.Infoln("Launching ", len(tasks), "tasks for offer", chosen_offer.Id.GetValue())
	driver.LaunchTasks([]*mesos.OfferID{chosen_offer.Id}, tasks, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
	sched.tasksLaunched++
	DeleteFromQ(sched.q, mid)
	return
}

func (sched *ExampleScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())
	if "TASK_RUNNING" == status.State.Enum().String() {
		fmt.Println("has been started Succesfully, exiting")
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
