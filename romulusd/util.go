package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"k8s.io/kubernetes/pkg/client/unversioned"
)

// func LogLevel(s kingpin.Settings) (lvl *capnslog.LogLevel) {
// 	lvl = new(capnslog.LogLevel)
// 	s.SetValue(lvl)
// 	return
// }

func md5Hash(ss ...interface{}) string {
	if len(ss) < 1 {
		return ""
	}

	h := md5.New()
	for i := range ss {
		io.WriteString(h, fmt.Sprintf("%v", ss[i]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func kubeClient() (unversioned.Interface, error) {
	if test {
		return tKubeClient, nil
	}

	cfg := &unversioned.Config{
		Host:     (*kubeAddr).String(),
		Username: *kubeUser,
		Password: *kubePass,
		Insecure: true,
	}
	if useTLS() {
		cfg.Insecure = false
		cfg.CertFile = *kubeCert
		cfg.KeyFile = *kubeKey
		cfg.CAFile = *kubeCA
	}
	if *kubeUseClust {
		if cc, er := unversioned.InClusterConfig(); er == nil {
			cfg = cc
		}
	}
	return unversioned.New(cfg)
}

func useTLS() bool {
	return *kubeCert != "" && (*kubeKey != "" || *kubeCA != "")
}

func formatSelectors() {
	ss := make(map[string]string, len(*svcSel))
	for k := range *svcSel {
		key := k
		if !strings.HasPrefix(k, "romulus/") {
			key = fmt.Sprintf("romulus/%s", key)
		}
		ss[key] = (*svcSel)[k]
	}
	*svcSel = ss
}
