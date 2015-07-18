package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"time"

	"github.com/ghodss/yaml"
	"github.com/mgutz/logxi/v1"
	"github.com/timelinelabs/romulus/romulus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	ep = kingpin.Flag("etcd", "etcd peers").Short('e').Default("http://127.0.0.1:2379").OverrideDefaultFromEnvar("ETCD_PEERS").URLList()
	km = kingpin.Flag("kube", "kubernetes endpoint").Short('k').Default("http://127.0.0.1:8080").OverrideDefaultFromEnvar("KUBE_MASTER").URL()
	ku = kingpin.Flag("kube-user", "kubernetes username").Short('U').Default("").OverrideDefaultFromEnvar("KUBE_USER").String()
	kp = kingpin.Flag("kube-pass", "kubernetes password").Short('P').Default("").OverrideDefaultFromEnvar("KUBE_PASS").String()
	kv = kingpin.Flag("kube-api", "kubernetes api version").Default("v1").OverrideDefaultFromEnvar("KUBE_API_VER").String()
	kc = kingpin.Flag("kubecfg", "path to kubernetes cfg file").Short('C').ExistingFile()
	sl = kingpin.Flag("svc-selector", "service selectors. Leave blank for Everything(). Form: key=value").Short('s').Default("type=external").OverrideDefaultFromEnvar("SVC_SELECTOR").StringMap()
)

func main() {
	kingpin.Version(romulus.Version())
	kingpin.Parse()
	l := log.New("romulusd")

	eps := []string{}
	kcc := romulus.KubeClientConfig{
		Host:     (*km).String(),
		Username: *ku,
		Password: *kp,
		Insecure: true,
	}
	for _, e := range *ep {
		eps = append(eps, e.String())
	}
	if *kc != "" {
		b, _ := ioutil.ReadFile(*kc)
		yaml.Unmarshal(b, &kcc)
		if kcc.CAFile != "" || kcc.CertFile != "" {
			kcc.Insecure = false
		}
	}

	l.Info("Starting up romulusd", "version", romulus.Version())
	c, e := romulus.NewClient(&romulus.Config{
		PeerList:   eps,
		Version:    (romulus.ResourceVersion)(*kv),
		KubeConfig: kcc,
		Selector:   *sl,
	})
	if e != nil {
		l.Error("Configuration Error!", "err", e)
		os.Exit(255)
	}

	if e := romulus.Start(c); e != nil {
		l.Error("Runtime Error!", "error", e)
		romulus.Stop()
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	l.Info("Received interrupt, shutting down")
	romulus.Stop()
	time.Sleep(1)
	os.Exit(0)
}
