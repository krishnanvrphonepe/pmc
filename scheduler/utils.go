package scheduler

import (
	"fmt"
	log "github.com/golang/glog"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
)

func GetAttribVal ( offer *mesos.Offer, ct string , h string) (int,bool)  {
	host_ok := false
	retval := 0 
	vm_on_host := false
	for _, attrib := range offer.Attributes {
	fmt.Println("ATTRIB:\n",attrib) 
		if (*attrib.Name == "vt_enabled") && (*attrib.Scalar.Value == 1) {
			host_ok = true
		} 
		// If VM is already on this host, send this as the offer
		// This takes care of redundant calls

		if (*attrib.Name == h) && (*attrib.Scalar.Value == 1) {
			vm_on_host = true
		} 
		if (*attrib.Name == ct) {
			retval = int(*attrib.Scalar.Value)
		}
	}
	if host_ok {
		return retval,vm_on_host
	} 
	return 100,vm_on_host
}
func getOfferScalar(offer *mesos.Offer, name string) float64 {

	for _, attrib := range offer.Attributes {
		//fmt.Println(attrib)
		//fmt.Println("ATTRIB")
		//fmt.Println(*attrib.Name, *attrib.Scalar.Value)

		if (*attrib.Name == "vt_enabled") && (*attrib.Scalar.Value == 1) {
			resources := util.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
				return res.GetName() == name
			})

			value := 0.0
			for _, res := range resources {
				value += res.GetScalar().GetValue()
			}
			return value
		}
	}
	return 0.0
}

func getOfferCpu(offer *mesos.Offer) float64 {
	return getOfferScalar(offer, "cpus")
}

func getOfferMem(offer *mesos.Offer) float64 {
	return getOfferScalar(offer, "mem")
}

func logOffers(offers []*mesos.Offer) {
	for _, offer := range offers {
		log.Infof("Received Offer <%v> with cpus=%v mem=%v", offer.Id.GetValue(), getOfferCpu(offer), getOfferMem(offer))
	}
}
