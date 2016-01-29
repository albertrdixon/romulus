package url

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromJSON(t *testing.T) {
	is := assert.New(t)
	var tests = []struct {
		run   int
		data  []byte
		url   URL
		valid bool
	}{
		{1, []byte(`"http://1.2.3.4"`), URL{Scheme: "http", Host: "1.2.3.4"}, true},
		{2, []byte(`"://this.is#bad"`), URL{}, false},
	}

	for _, test := range tests {
		var u = URL{}
		switch test.valid {
		case true:
			is.NoError(json.Unmarshal(test.data, &u), "test %d", test.run)
		case false:
			is.Error(json.Unmarshal(test.data, &u), "test %d", test.run)
		}
		is.Equal(test.url.String(), u.String(), "test %d", test.run)
	}
}

func TestParse(t *testing.T) {
	is := assert.New(t)
	var tests = []struct {
		run   int
		raw   string
		url   URL
		valid bool
	}{
		{1, "http://1.2.3.4", URL{Scheme: "http", Host: "1.2.3.4"}, true},
		{2, "://this.is#bad", URL{}, false},
	}

	for _, test := range tests {
		u, er := Parse(test.raw)
		is.Equal(test.valid, er == nil, "test %d", test.run)
		if u != nil {
			is.Equal(test.url.String(), u.String(), "test %d", test.run)
		}
	}
}
