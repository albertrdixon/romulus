package romulus

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/client/unversioned"
)

// Config is used to configure the Registrar
type Config struct {
	PeerList      []string
	EtcdTimeout   time.Duration
	KubeConfig    *unversioned.Config
	APIVersion    string
	Selector      map[string]string
	VulcanEtcdKey string
}

func configure(c *Config) {
	conf = c
	kubeClient = unversioned.New(c.KubeConfig)
	etcdClient = NewEtcdClient(
		c.PeerList,
		fmt.Sprintf("/%s", strings.Trim(c.VulcanEtcdKey, "/")),
		c.EtcdTimeout,
	)
}
