/**
* Licensed to the Apache Software Foundation (ASF) under one
G or more contributor license agreements.  See the NOTICE file
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
	"net"
	"os"
	"strconv"
	"github.com/gogo/protobuf/proto"
	log "github.com/golang/glog"
	. "github.com/krishnanvrphonepe/pmc/scheduler"
	. "github.com/krishnanvrphonepe/pmc/server"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"
)

const (
	CPUS_PER_TASK       = 1
	MEM_PER_TASK        = 1024
	defaultArtifactPort = 12345
)

var (
	address      = flag.String("address", "127.0.0.1", "Binding address for artifact server")
	artifactPort = flag.Int("artifactPort", defaultArtifactPort, "Binding port for artifact server")
	master       = flag.String("master", "192.168.254.10:5050", "Master address <ip:port>")
	executorPath = flag.String("executor", "./exec.tgz", "Path to test executor")
	taskCount    = flag.String("task-count", "1", "Total task count to run.")
	hostname     = flag.String("h", "", "hostname")
	mac          = flag.String("mac", "", "mac")
	comp_type    = flag.String("ct", "", "Component-Type")
)

func init() {
	flag.Parse()
}

func main() {

	// Start HTTP server hosting executor binary
	if !(*hostname != "" && *mac != "") {
		fmt.Println("Both -h <hostname> & -mac <MAC> needs to be defined")
		os.Exit(1)
	}
	uri := ServeExecutorArtifact(*address, *artifactPort, *executorPath)

	// Executor
	exec := prepareExecutorInfo(uri, getExecutorCmd(*executorPath),hostname,mac)

	// Scheduler
	numTasks, err := strconv.Atoi(*taskCount)
	if err != nil {
		log.Fatalf("Failed to convert '%v' to an integer with error: %v\n", taskCount, err)
		os.Exit(-1)
	}

	scheduler := NewExampleScheduler(exec, numTasks, CPUS_PER_TASK, MEM_PER_TASK,hostname,mac,comp_type)
	if err != nil {
		log.Fatalf("Failed to create scheduler with error: %v\n", err)
		os.Exit(-2)
	}

	// Framework
	fwinfo := &mesos.FrameworkInfo{
		User: proto.String("root"), // Mesos-go will fill in user.
		Name: proto.String("PMC Framework (Go)"),
	}

	// Scheduler Driver
	config := sched.DriverConfig{
		Scheduler:      scheduler,
		Framework:      fwinfo,
		Master:         *master,
		Credential:     (*mesos.Credential)(nil),
		BindingAddress: parseIP(*address),
	}

	driver, err := sched.NewMesosSchedulerDriver(config)

	if err != nil {
		log.Fatalf("Unable to create a SchedulerDriver: %v\n", err.Error())
		os.Exit(-3)
	}

	if stat, err := driver.Run(); err != nil {
		log.Fatalf("Framework stopped with status %s and error: %s\n", stat.String(), err.Error())
		os.Exit(-4)
	}
}

func prepareExecutorInfo(uri string, cmd string, hn *string, mac *string) *mesos.ExecutorInfo {
	executorUris := []*mesos.CommandInfo_URI{
		{
			Value: &uri,
			//Executable: proto.Bool(true),
		},
	}
	virt_cmd := "./virtmesos -h " + *hn + " -mac " + *mac
	fmt.Println("Command to be exec: ",virt_cmd) 
	return &mesos.ExecutorInfo{
		ExecutorId: util.NewExecutorID(*hn),
		Name:       proto.String("kvm"),
		Source:     proto.String("virt_executor"),
		Command: &mesos.CommandInfo{
			Value: proto.String(virt_cmd),
			Uris:  executorUris,
			//Arguments: args,
		},
	}
}

func getExecutorCmd(path string) string {
	return "." + GetHttpPath(path)
}

func parseIP(address string) net.IP {
	addr, err := net.LookupIP(address)
	if err != nil {
		log.Fatal(err)
	}
	if len(addr) < 1 {
		log.Fatalf("failed to parse IP from address '%v'", address)
	}
	return addr[0]
}
