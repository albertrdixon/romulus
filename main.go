package main

import (
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/timelinelabs/vulcand/plugin/registry"
	"golang.org/x/net/context"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	lbs = []string{"vulcand"}

	ro = kingpin.New("romulusd", "A utility for automatically registering Kubernetes services in Vulcand")

	kubeAPI   = ro.Flag("kube-api", "URL for kubernetes api").Short('k').Default("http://127.0.0.1:8080").OverrideDefaultFromEnvar("KUBE_API").URL()
	kubeVer   = ro.Flag("kube-api-ver", "kubernetes api version").Default("v1").OverrideDefaultFromEnvar("KUBE_API_VER").String()
	kubeSec   = ro.Flag("kube-insecure", "Run kubernetes client in insecure mode").OverrideDefaultFromEnvar("KUBE_INSECURE").Bool()
	selector  = ro.Flag("svc-selector", "service selectors. Leave blank for Everything(). Form: key=value").Short('s').PlaceHolder("key=value[,key=value]").OverrideDefaultFromEnvar("SVC_SELECTOR").StringMap()
	provider  = ro.Flag("provider", "LoadBalancer provider").Short('p').Default("vulcand").Enum(lbs...)
	resync    = ro.Flag("sync-interval", "Resync period with kube api").Default("30m").Duration()
	timeout   = ro.Flag("lb-timeout", "Timeout for communicating with loadbalancer provider").Default("10s").Duration()
	vulcanAPI = ro.Flag("vulcan-api", "URL for vulcand api").Default("http://127.0.0.1:8182").OverrideDefaultFromEnvar("VULCAN_API").URL()
	logLevel  = ro.Flag("log-level", "log level. One of: fatal, error, warn, info, debug").Short('l').Default("info").OverrideDefaultFromEnvar("LOG_LEVEL").Enum(logger.Levels...)
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
	ng, er := newEngine((*kubeAPI).String(), *kubeVer, *kubeSec, *selector, lb, ctx)
	if er != nil {
		logger.Fatalf(er.Error())
	}

	if er := ng.Start(*resync); er != nil {
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

func getLBProvider(kind string, c context.Context) (LoadBalancer, error) {
	switch kind {
	default:
		return nil, errors.New("Unknown LB type")
	case "vulcand":
		lb, er := newVulcanLB((*vulcanAPI).String(), registry.GetRegistry(), c)
		if er != nil {
			return nil, er
		}
		return lb, nil
	}
}
