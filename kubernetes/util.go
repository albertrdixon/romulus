package kubernetes

import (
	"crypto/md5"
	"fmt"
	"path"
	"strings"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/util"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/sets"
)

func HasServiceIP(s *api.Service) bool {
	return s.Spec.Type == api.ServiceTypeClusterIP && api.IsServiceIPSet(s)
}

func GetServicePort(svc *api.Service, port intstr.IntOrString) (api.ServicePort, bool) {
	for _, svcPort := range svc.Spec.Ports {
		if port.String() == svcPort.Name || port.IntValue() == svcPort.Port {
			return svcPort, true
		}
	}
	return api.ServicePort{}, false
}

func GenResourceID(namespace, name string, port intstr.IntOrString) string {
	id := []string{namespace, name, port.String()}
	return strings.Join(id, ".")
}

func GenServerID(namespace, name, ip string, port int) string {
	id := []string{namespace, name, util.Hashf(md5.New(), ip, port, namespace, name)[:hashLen]}
	return strings.Join(id, ".")
}

func matchPort(a api.ServicePort, b api.EndpointPort) bool {
	if a.Name != "" && b.Name != "" {
		return a.Name == b.Name
	}
	return true
}

func matchIntStr(str string, num int, is intstr.IntOrString) bool {
	switch is.Type {
	default:
		return false
	case intstr.Int:
		return num != 0 && is.IntValue() == num
	case intstr.String:
		return str != "" && is.String() == str
	}
}

func matchIngressBackend(serviceName string, servicePort api.ServicePort, backend extensions.IngressBackend) bool {
	logger.Debugf("Comparing Service(Name=%q, Port=%v) with IngressBackend(%v)", serviceName, servicePort, backend)
	nameMatch := serviceName == backend.ServiceName
	isMatch := matchIntStr(servicePort.Name, servicePort.Port, backend.ServicePort)
	logger.Debugf("NameMatch = %v intstrMatch = %v", nameMatch, isMatch)
	return serviceName == backend.ServiceName &&
		matchIntStr(servicePort.Name, servicePort.Port, backend.ServicePort)
}

func cacheLookupKey(namespace, name string) cache.ExplicitKey {
	if namespace == "" {
		return cache.ExplicitKey(name)
	}
	k := fmt.Sprintf("%s/%s", namespace, name)
	return cache.ExplicitKey(k)
}

func selectorFromMap(m Selector) labels.Selector {
	var s = labels.NewSelector()

	for k, val := range m {
		key := k
		if !strings.HasPrefix(k, Keyspace) {
			key = path.Join(Keyspace, k)
		}
		if req, er := labels.NewRequirement(key, labels.DoubleEqualsOperator, sets.NewString(val)); er == nil {
			s = s.Add(*req)
		} else {
			logger.Warnf("Unable to add selector %s=%s: %v", key, val, er)
		}
	}
	if s.Empty() {
		return labels.Everything()
	}
	return s
}

func intstrFromPort(name string, port int) intstr.IntOrString {
	kind := intstr.Int
	if name != "" {
		kind = intstr.String
	}
	return intstr.IntOrString{
		StrVal: name,
		IntVal: int32(port),
		Type:   kind,
	}
}

func isRegexp(expr string) bool {
	return strings.HasPrefix(expr, "|") && strings.HasSuffix(expr, "|")
}
