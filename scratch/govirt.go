package main

import "fmt"
import "github.com/rgbkrk/libvirt-go"
import "os"
import "flag"
import "io/ioutil"
import "strings"
import "strconv"
import "os/exec"
import "github.com/satori/go.uuid"


var (
        hostname = flag.String("h", "", "VM Hostname")
        mac = flag.String("mac", "52:54:00:a8:fe:69", "MAC Address")
        kernel = flag.String("k", "/var/local/pmc/trusty-server-cloudimg-amd64-vmlinuz-generic", "Kernel Image")
        fqdn = flag.String("f", "phonepe.int", "FQDN")
        cpu = flag.Int("c", 1, "VCPUs")
        mem = flag.Int("m", 1, "Mem")
	cloud_init = "/etc/default/cloud-init.go" 
	local_pmc_dir = "/var/local/pmc"
	cloud_local_ds = "/usr/bin/cloud-localds"
        original_source_image = "/var/lib/libvirt/images/trusty.ORIG.img"
	host_image_location = "/var/lib/libvirt/images"

)

func init() {
	flag.Parse() ; 
}

func main() {
    xml := GenXMLForDom() 
    domxml := GenDomXML(xml) 
    fmt.Println(domxml) 
    conn,err := libvirt.NewVirConnection("qemu:///system") 
    if(err != nil) {
    	fmt.Println(err) 
	os.Exit(1) 
    }

    doms, err := conn.ListAllDomains(libvirt.VIR_CONNECT_LIST_DOMAINS_PERSISTENT)
    if err != nil {
	fmt.Println(err) 
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

} 
func GenDomXML(xml string) string {
	cloud_init_img := GenCloudInitConfig() 
	host_img := GenHostImg() 
	fmt.Println(cloud_init_img) 
	xml = strings.Replace(xml,"__PMC__HOSTNAME__",*hostname,1) 
	uuid := fmt.Sprintf("%s",uuid.NewV4()) 
	xml = strings.Replace(xml,"__PMC__UUID__",uuid,1) 
	mem := getMem() 
	xml = strings.Replace(xml,"__PMC__MEMORY__",mem,2) 
	xml = strings.Replace(xml,"__PMC__VCPU__",strconv.Itoa(*cpu),1) 
	xml = strings.Replace(xml,"__PMC__KERNEL__",*kernel,1) 
	//cloud_init_img := GenCloudInitConfig(*hostname) 
	xml = strings.Replace(xml,"__PMC__CLOUDINITIMAGE__",cloud_init_img,1) 
	xml = strings.Replace(xml,"__PMC__HOSTIMAGE__",host_img,1) 
	xml = strings.Replace(xml,"__PMC__MAC__",*mac,1) 
	return xml 
	
}

func GenHostImg () string {
	image_loc := fmt.Sprintf("%s/%s.img",host_image_location,*hostname) 
	Removefile(image_loc) 
	cmd := exec.Command("cp",original_source_image,image_loc) 
	if err := cmd.Run(); err != nil {
	  fmt.Println(err)
	  os.Exit(1) 
	} 
	return image_loc
}
	

func GenCloudInitConfig() string {
	dat, err := ioutil.ReadFile(cloud_init)
	if err != nil {
		fmt.Println(err) 
		os.Exit(1) 
	}
	cloud_init_yaml := string(dat) 
	cloud_init_yaml = strings.Replace(cloud_init_yaml,"__HOSTNAME__",*hostname,1) 
	cloud_init_yaml = strings.Replace(cloud_init_yaml,"__FQDN__",*fqdn,1) 
	d1 := []byte(cloud_init_yaml)
	ci_input := fmt.Sprintf("%s/%s",local_pmc_dir,*hostname ) 
	ci_input_img := fmt.Sprintf("%s/%s.img",local_pmc_dir,*hostname ) 
	Removefile(ci_input) 
	Removefile(ci_input_img) 
        err = ioutil.WriteFile(ci_input, d1, 0644)
	if err != nil {
		fmt.Println(err) 
		os.Exit(1) 
	}
	cmd := exec.Command(cloud_local_ds,ci_input_img,ci_input) 
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
	         fmt.Println("DELETION FAILED",f,err)
	         os.Exit(1) 
	  }
	}
}

func getMem() string {
	m := *mem*1024*1024
	return strconv.Itoa(m)
}
func GenXMLForDom() string {
	dat, err := ioutil.ReadFile("/etc/default/PMCLibvirtTemplate.xml")
	if err != nil {
		fmt.Println(err) 
		os.Exit(1) 
	}
	fmt.Print(string(dat) )
	xmlstr := string(dat) 
	return xmlstr 
	
}


