package main

import (
	"fmt"
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
