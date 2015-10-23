package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"sort"
	"strings"
)

func isDebug() bool {
	return *debug || *logLevel == "debug"
}

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

type ppSlice []string

func (p ppSlice) String() string {
	sort.Strings(p)
	return fmt.Sprintf("[%s]", strings.Join(p, ", "))
}

func annotationf(p, n string) string {
	if !strings.HasPrefix(p, "romulus/") {
		return fmt.Sprintf("romulus/%s%s", p, n)
	}
	return fmt.Sprintf("%s%s", p, n)
}
func labelf(l string, s ...string) string {
	la := strings.Join(append([]string{l}, s...), ".")
	if !strings.HasPrefix(la, "romulus/") {
		return strings.Join([]string{"romulus", la}, "/")
	}
	return l
}

func backendf(id string) string       { return fmt.Sprintf("backends/%s/backend", id) }
func frontendf(id string) string      { return fmt.Sprintf("frontends/%s/frontend", id) }
func backendDirf(id string) string    { return fmt.Sprintf("backends/%s", id) }
func frontendDirf(id string) string   { return fmt.Sprintf("frontends/%s", id) }
func serverf(b, id string) string     { return fmt.Sprintf("backends/%s/servers/%s", b, id) }
func serverDirf(id string) string     { return fmt.Sprintf("backends/%s/servers", id) }
func middlewaref(f, id string) string { return fmt.Sprintf("frontends/%s/middlewares/%s", f, id) }
func middlewareDirf(f string) string  { return fmt.Sprintf("frontends/%s/middlewares", f) }
