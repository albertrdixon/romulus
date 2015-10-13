package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/context"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

var (
	cache *cMap
	etcd  etcdInterface

	test        = false
	tKubeClient unversioned.Interface

	ro = kingpin.New("romulusd", "A utility for automatically registering Kubernetes services in Vulcand")

	vulcanKey    = ro.Flag("vulcan-key", "default vulcand etcd key").Default("vulcand").OverrideDefaultFromEnvar("VULCAND_KEY").String()
	etcdPeers    = ro.Flag("etcd", "etcd peers").Short('e').Default("http://127.0.0.1:2379").OverrideDefaultFromEnvar("ETCD_PEERS").URLList()
	etcdTimeout  = ro.Flag("etcd-timeout", "etcd request timeout").Short('t').Default("5s").OverrideDefaultFromEnvar("ETCD_TIMEOUT").Duration()
	kubeAddr     = ro.Flag("kube", "kubernetes endpoint").Short('k').Default("http://127.0.0.1:8080").OverrideDefaultFromEnvar("KUBE_MASTER").URL()
	kubeUseClust = ro.Flag("kube-cluster-config", "use kubernetes in cluster config for client").Bool()
	kubeUser     = ro.Flag("kube-user", "kubernetes username").Short('U').Default("").OverrideDefaultFromEnvar("KUBE_USER").String()
	kubePass     = ro.Flag("kube-pass", "kubernetes password").Short('P').Default("").OverrideDefaultFromEnvar("KUBE_PASS").String()
	kubeAPIVer   = ro.Flag("kube-api", "kubernetes api version").Default("v1").OverrideDefaultFromEnvar("KUBE_API_VER").String()
	kubeCert     = ro.Flag("kube-cert-file", "kubernetes cert file").ExistingFile()
	kubeKey      = ro.Flag("kube-key-file", "kubernetes key file").ExistingFile()
	kubeCA       = ro.Flag("kube-ca-file", "kubernetes ca file").ExistingFile()
	kubeRetry    = ro.Flag("kube-retry-interval", "interval between attempts to set watches").Default("2s").OverrideDefaultFromEnvar("KUBE_RETRY").Duration()
	kubeConfig   = ro.Flag("kubecfg", "path to kubernetes cfg file").Short('C').PlaceHolder("/path/to/.kubecfg").ExistingFile()
	svcSel       = ro.Flag("svc-selector", "service selectors. Leave blank for Everything(). Form: key=value").Short('s').PlaceHolder("key=value[,key=value]").OverrideDefaultFromEnvar("SVC_SELECTOR").StringMap()
	debug        = ro.Flag("debug", "Enable debug logging. e.g. --log-level debug").Short('d').Bool()
	logLevel     = ro.Flag("log-level", "log level. One of: fatal, error, warn, info, debug").Short('l').Default("info").OverrideDefaultFromEnvar("LOG_LEVEL").Enum(logLevels...)
	etcdDebug    = ro.Flag("debug-etcd", "Enable cURL debug logging for etcd").Bool()

	serverTagLen = 8
)

func main() {
	kingpin.Version(getVersion())
	kingpin.MustParse(ro.Parse(os.Args[1:]))
	setupLog()
	infoL("Starting up romulusd version=%s", getVersion())

	cache = newCache()
	peers := []string{}
	for _, p := range *etcdPeers {
		peers = append(peers, p.String())
	}

	var er error
	*vulcanKey = etcdKeyf(*vulcanKey)
	etcd, er = NewEtcdClient(peers, *vulcanKey, *etcdTimeout)
	if er != nil {
		fatalL("Failed to get etcd client: %v", er)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w, er := startWatches(ctx)
	if er != nil {
		fatalL("Failed to get kubernetes client: %v", er)
	}
	go processor(w, ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-sig:
		infoL("Recieved interrupt, shutting down")
		cancel()
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}
}
