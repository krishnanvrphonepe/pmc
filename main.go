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
	"github.com/gogo/protobuf/proto"
	log "github.com/golang/glog"
	. "github.com/krishnanvrphonepe/pmc/scheduler"
	. "github.com/krishnanvrphonepe/pmc/server"
	mesos "github.com/mesos/mesos-go/mesosproto"
	sched "github.com/mesos/mesos-go/scheduler"
	"net"
	"os"
	"fmt"
	"github.com/kr/beanstalk"
)

const (
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
	qep          = flag.String("q", "", "Q Endpoint")
	cpu          = flag.Float64("cpu", 1, "CPU Count")
	mem          = flag.Float64("mem", 1024, "Mem Count")
	uri          string
)

func init() {
	flag.Parse()
}

func main() {

	// Start HTTP server hosting executor binary
	uri = ServeExecutorArtifact(*address, *artifactPort, *executorPath)
	CreateAndRunMesosTask(uri)

	/*
		// Handle Cmd Line args if Any
		//vm_input := NewVMInputter(*hostname, *mac, *mem, *cpu, *executorPath, *comp_type)
		if vm_input != nil {
			vm_input.CreateAndRunMesosTask(uri)
		} else {
			fmt.Println("This is going to be a noop", vm_input)
		}
		if *qep != "" {
			fmt.Println("QEP=", *qep)
			for { // endless loop
				vm_inputq,id := FetchFromQ(*qep)
				fmt.Println("ID=",id)
				if vm_inputq != nil {
					vm_inputq.CreateAndRunMesosTask(uri)
					//DeleteFromQ(*qep,id)
				}
			}
		}
	*/
}

func CreateAndRunMesosTask(uri string) {

	beanstalk_conn, e := beanstalk.Dial("tcp", *qep)
	if e != nil {
		fmt.Println(">> Beanstalk got error:",e) 
		os.Exit(1) 
	}
		
	scheduler := NewExampleScheduler(beanstalk_conn, uri)

	// Framework
	fwinfo := &mesos.FrameworkInfo{
		User: proto.String("root"), // Mesos-go will fill in user.
		Name: proto.String("New PMC Framework (Go)"),
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
