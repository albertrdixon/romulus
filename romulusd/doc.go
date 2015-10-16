// package main runs the romulusd deamon.
// Romulus is a utility to automatically register kubernetes services in vulcand.
// Kubernetes services are configured with annotations in the 'romulus/' namespace like so
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
	version = "v0.1.3"

	// SHA is the build sha
	SHA string
)

func getVersion() string {
	if SHA != "" {
		return fmt.Sprintf("%s-%s", version, SHA)
	}
	return version
}
