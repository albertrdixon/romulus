package vulcand

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timelinelabs/romulus/kubernetes"
	vroute "github.com/vulcand/route"
)

func TestBuildRoute(te *testing.T) {
	is := assert.New(te)

	var tests = []struct {
		expected    string
		annotations map[string]string
	}{
		{"PathRegexp(`.*`)", map[string]string{}},
		{"Host(`abc`) && PathRegexp(`/f.*`)", map[string]string{"host": "abc", "prefix": "/f"}},
		{"Method(`GET`) && Method(`POST`)", map[string]string{"methods": "get; post"}},
		{
			"Header(`X-Foo`, `Bar`) && HeaderRegexp(`X-Bif`, `Baz.*`)",
			map[string]string{"headers": "X-Foo=Bar; X-Bif=|Baz.*|"},
		},
		{
			"HostRegexp(`.*local`) && PathRegexp(`/f/b.*`)",
			map[string]string{"host": "|.*local|", "path": "|/f/b.*|"},
		},
	}

	for _, t := range tests {
		rt := kubernetes.NewRoute("foo", t.annotations)
		actual := NewRoute(rt).String()
		te.Logf("Actual: %v", actual)
		is.Equal(t.expected, actual)
		is.True(vroute.IsValid(actual))
	}
}
