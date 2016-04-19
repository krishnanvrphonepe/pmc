package main

import (
	"fmt"
	"github.com/kr/beanstalk"
	"log"
	"strings"
	"time"
	"strconv"
)

type VMInput struct {
	hostname  string
	mac       string
	cpu       float64
	mem       float64
	executor  string
	comp_type string
}

const (
	CPUS_PER_TASK = 1
	MEM_PER_TASK  = 1024
)

func DeleteFromQ(q string,id uint64) {
	conn, e := beanstalk.Dial("tcp", q)
	defer conn.Close()
	if e != nil {
		log.Fatal(e)
	}
	e = conn.Delete(id) 
}

func FetchFromQ(q string) (*VMInput,uint64) {
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
	str := strings.Replace(string(body), "---", "", -1)
	var ret VMInput
	fmt.Println(str)
	s := strings.Split(str, "\n")
	fmt.Println(s)
	for _, m := range s {
		m = strings.Replace(m, " ", "", -1)
		if len(m) < 3 {
			continue
		}
			
		kv := strings.Split(m, "=")
		//fmt.Println("K=",kv[0],"V=",kv[1]) 
		v := kv[1]
		switch kv[0] {
		case "hostname":
			ret.hostname = kv[1]
		case "mac":
			ret.mac = kv[1]
		case "executor":
			ret.executor = kv[1]
		case "comp_type":
			ret.comp_type = kv[1]
		case "cpu":
			ret.cpu,err =  strconv.ParseFloat(v, 64)
		case "mem":
			ret.mem,err =  strconv.ParseFloat(v, 64)
		}
	}
	fmt.Printf("PRINTING THE STRUCT %+v",ret)
	return &ret,id

}

func NewVMInputter(hostname string, mac string, mem float64, cpu float64, exr string, ct string) *VMInput {
	if hostname == "" {
		fmt.Println("Hostname should be defined")
		return nil
	}
	if mac == "" {
		fmt.Println("MAC should be defined")
		return nil
	}
	if mem == 0 {
		mem = MEM_PER_TASK
	}
	if cpu == 0 {
		cpu = CPUS_PER_TASK
	}
	if ct == "" {
		fmt.Println("Component type should be defined")
		return nil
	}
	if exr == "" {
		exr = "./exec.tgz"
	}
	return &VMInput{
		hostname:  hostname,
		mac:       mac,
		cpu:       cpu,
		mem:       mem,
		executor:  exr,
		comp_type: ct,
	}
}
