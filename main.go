package main

import (
	"os"
	"os/signal"

	"gopkg.in/alecthomas/kingpin.v1"
)

var (
	ep = kingpin.Flag("etcd", "etcd peers").Short('e').Default("http://127.0.0.1:2379").URLList()
	km = kingpin.Flag("kube", "kubernetes endpoint").Short('k').Default("http://127.0.0.1:8080").URL()
	ku = kingpin.Flag("kube-user", "kubernetes username").Short('U').Default("").String()
	kp = kingpin.Flag("kube-pass", "kubernetes password").Short('P').Default("").String()
	kv = kingpin.Flag("kube-api", "kubernetes api version").Default("v1").String()
)

func main() {
	kingpin.Parse()
	eps := []string{}
	for _, e := range *ep {
		eps = append(eps, e.String())
	}
	c, e := NewClient(&Config{
		p: eps,
		v: (ResourceVersion)(*kv),
		k: KubeClientConfig{
			Host:     (*km).String(),
			Username: *ku,
			Password: *kp,
		},
	})
	if e != nil {
		panic(e)
	}

	done := StopChan(make(chan struct{}))
	e = StartWatch(c, done)
	if e != nil {
		panic(e)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
	done <- struct{}{}
	os.Exit(0)
}
