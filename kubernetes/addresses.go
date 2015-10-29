package kubernetes

import (
	"fmt"

	"github.com/albertrdixon/gearbox/url"
	log "github.com/timelinelabs/romulus/logger"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/endpoints"
)

func AddressesFromSubsets(subs []api.EndpointSubset) Addresses {
	var addrs = Addresses(make(map[string]*url.URL))
	subs = endpoints.RepackSubsets(subs)
	for i := range subs {
		for _, port := range subs[i].Ports {
			for k := range subs[i].Addresses {
				ur, er := url.Parse(fmt.Sprintf("http://%s:%d", subs[i].Addresses[k].IP, port.Port))
				if er != nil {
					log.Warnf("Failed to parse Endpoint Address: %v", er)
					continue
				}
				addrs[port.Port] = ur
			}
		}
	}
	return addrs
}
