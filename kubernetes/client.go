package kubernetes

import (
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
)

var FakeKubeClient = &testclient.Fake{}

func GetKubeClient(cfg *ClientConfig) (unversioned.Interface, error) {
	switch {
	case cfg.Testing:
		return FakeKubeClient, nil
	case cfg.InCluster:
		if cc, er := unversioned.InClusterConfig(); er == nil {
			return unversioned.New(cc)
		}
		return unversioned.New(&cfg.Config)
	default:
		return unversioned.New(&cfg.Config)
	}
}

func ResetFakeClient() {
	FakeKubeClient = &testclient.Fake{}
}
