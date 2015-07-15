package main

import (
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/ghodss/yaml"
	"github.com/mgutz/logxi/v1"
	"gopkg.in/alecthomas/kingpin.v1"
)

var (
	ep = kingpin.Flag("etcd", "etcd peers").Short('e').Default("http://127.0.0.1:2379").URLList()
	km = kingpin.Flag("kube", "kubernetes endpoint").Short('k').Default("http://127.0.0.1:8080").URL()
	ku = kingpin.Flag("kube-user", "kubernetes username").Short('U').Default("").String()
	kp = kingpin.Flag("kube-pass", "kubernetes password").Short('P').Default("").String()
	kv = kingpin.Flag("kube-api", "kubernetes api version").Default("v1").String()
	kc = kingpin.Flag("kubecfg", "path to kubernetes cfg file").Short('C').ExistingFile()
	db = kingpin.Flag("debug", "debug mode").Short('d').Bool()
)

func main() {
	kingpin.Parse()
	if *db {
		os.Setenv("LOGXI", "*")
	}

	eps := []string{}
	kcc := KubeClientConfig{
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
	// spew.Dump(kcc)
	c, e := NewClient(&Config{
		p: eps,
		v: (ResourceVersion)(*kv),
		k: kcc,
	})
	if e != nil {
		log.Fatal("Configuration Error!", "err", e)
	}

	done := StopChan(make(chan struct{}, 1))
	if e = StartWatch(c, done); e != nil {
		log.Fatal("Runtime Error!", "err", e)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	log.Info("Received interrupt, shutting down")
	done <- struct{}{}
	os.Exit(0)
}
