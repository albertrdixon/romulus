// package main runs the romulusd deamon
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
