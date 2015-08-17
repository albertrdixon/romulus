package romulus

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/api"
)

var (
	endpointsFile = "test/test-endpoints.yaml"
	serviceFile   = "test/test-svc.yaml"
)

func setup() (*api.Endpoints, *api.Service, error) {
	var e *api.Endpoints
	var s *api.Service
	ed, err := ioutil.ReadFile(endpointsFile)
	if err != nil {
		return nil, nil, err
	}
	sd, err := ioutil.ReadFile(serviceFile)
	if err != nil {
		return nil, nil, err
	}
	if err := yaml.Unmarshal(ed, e); err != nil {
		return nil, nil, err
	}
	if err := yaml.Unmarshal(sd, s); err != nil {
		return nil, nil, err
	}
	return e, s, nil
}

func TestRegister(te *testing.T) {

}
