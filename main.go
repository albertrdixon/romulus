package main

import (
	"errors"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/timelinelabs/romulus/kubernetes"
	"github.com/timelinelabs/romulus/loadbalancer"
	"github.com/timelinelabs/romulus/loadbalancer/traefik"
	"github.com/timelinelabs/romulus/loadbalancer/vulcand"
	"golang.org/x/net/context"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	lbs = []string{"vulcand", "traefik"}

	ro = kingpin.New("romulusd", "A utility for automatically registering Kubernetes services in Vulcand")

	kubeAPI     = ro.Flag("kube-api", "URL for kubernetes api").Short('k').Default("http://127.0.0.1:8080").OverrideDefaultFromEnvar("KUBE_API").URL()
	kubeVer     = ro.Flag("kube-api-ver", "kubernetes api version").Default("v1").OverrideDefaultFromEnvar("KUBE_API_VER").String()
	kubeUser    = ro.Flag("kube-user", "kubernetes username").String()
	kubePass    = ro.Flag("kube-pass", "kubernetes password").String()
	kubeSec     = ro.Flag("kube-insecure", "Run kubernetes client in insecure mode").OverrideDefaultFromEnvar("KUBE_INSECURE").Bool()
	selector    = ro.Flag("selector", "label selectors. Leave blank for Everything(). Form: key=value").Short('s').PlaceHolder("label=value").OverrideDefaultFromEnvar("SVC_SELECTOR").StringMap()
	annoKey     = ro.Flag("annotations-prefix", "annotations key prefix").Short('a').Default("romulus/").String()
	provider    = ro.Flag("provider", "LoadBalancer provider").Short('p').Default("vulcand").Enum(lbs...)
	resync      = ro.Flag("sync-interval", "Resync period with kube api").Default("1h").Duration()
	timeout     = ro.Flag("lb-timeout", "Timeout for communicating with loadbalancer provider").Default("10s").Duration()
	vulcanAPI   = ro.Flag("vulcan-api", "URL for vulcand api").Default("http://127.0.0.1:8182").OverrideDefaultFromEnvar("VULCAN_API").URL()
	traefikEtcd = ro.Flag("traefik-etcd", "etcd peers for traefik").OverrideDefaultFromEnvar("TRAEFIK_ETCD").URLList()
	logLevel    = ro.Flag("log-level", "log level. One of: fatal, error, warn, info, debug").Short('l').Default("info").OverrideDefaultFromEnvar("LOG_LEVEL").Enum(logger.Levels...)
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	kingpin.Version(getVersion())
	kingpin.MustParse(ro.Parse(os.Args[1:]))
	logger.Configure(*logLevel, "[romulusd] ", os.Stdout)
	logger.Infof("Starting up romulusd version=%s", getVersion())

	ctx, cancel := context.WithCancel(context.Background())
	lb, er := getLBProvider(*provider, ctx)
	if er != nil {
		logger.Fatalf(er.Error())
	}

	kubernetes.Keyspace = normalizeAnnotationsKey(*annoKey)
	ng, er := NewEngine((*kubeAPI).String(), *kubeUser, *kubePass, *kubeSec, lb, *timeout, ctx)
	if er != nil {
		logger.Fatalf(er.Error())
	}

	if er := ng.Start(*selector, *resync); er != nil {
		logger.Fatalf(er.Error())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-sig:
		logger.Infof("Shutting Down...")
		cancel()
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}
}

func getLBProvider(kind string, c context.Context) (loadbalancer.LoadBalancer, error) {
	switch kind {
	default:
		return nil, errors.New("Unknown LB type")
	case "vulcand":
		return vulcand.New((*vulcanAPI).String(), nil, c)
	case "traefik":
		peers := make([]string, 0, len(*traefikEtcd))
		for _, u := range *traefikEtcd {
			peers = append(peers, u.String())
		}
		return traefik.New(traefik.DefaultPrefix, peers, *timeout, c)
	}
}

func normalizeAnnotationsKey(key string) string {
	if !strings.HasSuffix(key, "/") {
		return key + "/"
	}
	return key
}
