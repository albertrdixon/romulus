// Romulus is a kubernetes ingress controller.
// Romulus will create loadbalancer resources based on Ingress objects and Service annotations
//
//    kind: Service
//    metadata:
//      annotations:
//        romulus/host: 'www.example.com'
//        romulus/path: '/web'
//
//
package main

import "fmt"

var (
	version = "v0.2.0"

	// SHA is the build sha
	SHA string
)

func getVersion() string {
	if SHA != "" {
		return fmt.Sprintf("%s-%s", version, SHA)
	}
	return version
}
