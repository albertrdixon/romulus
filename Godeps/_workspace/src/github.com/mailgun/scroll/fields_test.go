package scroll

import (
	"net/http"
	"net/url"

	. "gopkg.in/check.v1"
)

type FieldsSuite struct{}

var _ = Suite(&FieldsSuite{})

func (s *FieldsSuite) TestSafe(c *C) {
	request, _ := http.NewRequest("GET", "http://example.com", nil)
	request.Form = make(url.Values)
	request.Form["p"] = []string{"foo"}

	value, err := GetStringFieldSafe(request, "p", NewAllowSetBytes("f", 3))
	c.Assert(err, NotNil)

	value, err = GetStringFieldSafe(request, "p", NewAllowSetBytes("fo", 3))
	c.Assert(err, IsNil)
	c.Assert(value, Equals, "foo")

	value, err = GetStringFieldSafe(request, "p", NewAllowSetStrings([]string{"bar", "baz"}))
	c.Assert(err, NotNil)

	value, err = GetStringFieldSafe(request, "p", NewAllowSetStrings([]string{"foo", "bar"}))
	c.Assert(err, IsNil)
	c.Assert(value, Equals, "foo")
}

func (s *FieldsSuite) TestGetMultipleFields(c *C) {
	request, _ := http.NewRequest("GET", "http://example.com", nil)
	request.Form = make(url.Values)
	request.Form["p"] = []string{"1", "2"}

	values, err := GetMultipleFields(request, "p")
	c.Assert(err, IsNil)
	c.Assert(values[0], Equals, "1")
	c.Assert(values[1], Equals, "2")
}

func (s *FieldsSuite) TestGetMultipleFieldsRubyPHP(c *C) {
	request, _ := http.NewRequest("GET", "http://example.com", nil)
	request.Form = make(url.Values)
	request.Form["ruby[]"] = []string{"1", "2"}
	request.Form["php[0]"] = []string{"3"}
	request.Form["php[1]"] = []string{"4"}

	values, err := GetMultipleFields(request, "ruby")
	c.Assert(err, IsNil)
	c.Assert(values[0], Equals, "1")
	c.Assert(values[1], Equals, "2")

	values, err = GetMultipleFields(request, "php")
	c.Assert(err, IsNil)
	c.Assert(len(values), Equals, 2)
}

func (s *FieldsSuite) TestGetMultipleFieldsMissingValue(c *C) {
	request, _ := http.NewRequest("GET", "http://example.com", nil)
	request.Form = make(url.Values)
	request.Form["p"] = []string{"1", "2"}

	values, err := GetMultipleFields(request, "missing")
	c.Assert(err, NotNil)
	c.Assert(len(values), Equals, 0)
}

func (s *FieldsSuite) TestGetFloatField(c *C) {
	request, _ := http.NewRequest("GET", "http://example.com", nil)
	request.Form = make(url.Values)

	// Missing field.
	value, err := GetFloatField(request, "p")
	c.Assert(err, Equals, MissingFieldError{"p"})
	c.Assert(value, Equals, float64(0))

	// Invalid value.
	request.Form["p"] = []string{"abracadabra"}
	value, err = GetFloatField(request, "p")
	c.Assert(err, Equals, InvalidFormatError{"p", "abracadabra"})
	c.Assert(value, Equals, float64(0))

	// Success.
	request.Form["p"] = []string{"0.0000001"}
	value, err = GetFloatField(request, "p")
	c.Assert(err, IsNil)
	c.Assert(value, Equals, float64(0.0000001))
}
