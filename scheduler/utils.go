package scheduler

import (
	"fmt"
	log "github.com/golang/glog"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
)

func getOfferScalar(offer *mesos.Offer, name string) float64 {

	for _, attrib := range offer.Attributes {
		fmt.Println(attrib)
		fmt.Println("ATTRIB")
		fmt.Println(*attrib.Name, *attrib.Scalar.Value)

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
