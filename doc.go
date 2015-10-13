// Package romulus runs the romulusd deamon
package romulus

import "fmt"

var (
	version = "v0.1.3"

	// SHA is the build sha
	SHA string
)

func version() string {
	if SHA != "" {
		return fmt.Sprintf("%s-%s", version, SHA)
	}
	return version
}
