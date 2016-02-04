package vulcand

import (
	"os"
	"testing"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/stretchr/testify/assert"
	"github.com/timelinelabs/romulus/kubernetes"
)

func TestNewFrontend(te *testing.T) {
	var (
		is    = assert.New(te)
		v     = new(vulcan)
		tests = []struct {
			resource   *kubernetes.Resource
			assertions func(*frontend)
		}{
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/max_resp_size": "10Mi",
				"romulus/max_req_size":  "3Mi",
			}), func(f *frontend) {
				is.Equal(int64(10485760), f.HTTPSettings().Limits.MaxRespBodyBytes)
				is.Equal(int64(3145728), f.HTTPSettings().Limits.MaxMemBodyBytes)
				is.Equal("foo", f.GetID())
			}},
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/pass_host_header":      "true",
				"romulus/trust_forward_headers": "true",
			}), func(f *frontend) {
				is.Equal("foo", f.GetID())
				is.True(f.HTTPSettings().PassHostHeader)
				is.True(f.HTTPSettings().TrustForwardHeader)
			}},
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/failover_expression": `RequestMethod() == "GET" && ResponseCode() == 408`,
			}), func(f *frontend) {
				is.Equal("foo", f.GetID())
				is.Equal(`RequestMethod() == "GET" && ResponseCode() == 408`, f.HTTPSettings().FailoverPredicate)
			}},
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/pass_host_header":      "true",
				"romulus/trust_forward_headers": "true",
				"romulus/frontend_settings":     `{"Limits":{"MaxMemBodyBytes":12},"TrustForwardHeader":false}`,
			}), func(f *frontend) {
				is.Equal("foo", f.GetID())
				is.False(f.HTTPSettings().TrustForwardHeader)
				is.Equal(int64(12), f.HTTPSettings().Limits.MaxMemBodyBytes)
			}},
		}
	)

	if testing.Verbose() {
		logger.Configure("debug", "[test-vulcan-newfrontend] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	for _, t := range tests {
		fr, e := v.NewFrontend(t.resource)
		f, ok := fr.(*frontend)
		if is.NoError(e) && is.NotNil(fr) && is.True(ok, "Could not cast to frontend") {
			t.assertions(f)
		}
	}
}

func TestNewBackend(te *testing.T) {
	var (
		is    = assert.New(te)
		v     = new(vulcan)
		tests = []struct {
			resource   *kubernetes.Resource
			assertions func(*backend)
		}{
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/dial_timeout":            "50m",
				"romulus/read_timeout":            "50m",
				"romulus/max_idle_conns_per_host": "15",
			}), func(b *backend) {
				is.Equal("foo", b.GetID())
				is.Equal("50m0s", b.HTTPSettings().Timeouts.Dial)
				is.Equal("50m0s", b.HTTPSettings().Timeouts.Read)
				is.Equal(15, b.HTTPSettings().KeepAlive.MaxIdleConnsPerHost)
			}},
			{kubernetes.NewResource("foo", "", map[string]string{
				"romulus/dial_timeout":            "50s",
				"romulus/read_timeout":            "50s",
				"romulus/max_idle_conns_per_host": "15",
				"romulus/backend_settings":        `{"Timeouts":{"Read":"30m","TLSHandshake":"30m"},"KeepAlive":{"MaxIdleConnsPerHost":30}}`,
			}), func(b *backend) {
				is.Equal("foo", b.GetID())
				is.Equal("30m", b.HTTPSettings().Timeouts.TLSHandshake)
				is.Equal("30m", b.HTTPSettings().Timeouts.Read)
				is.Equal(30, b.HTTPSettings().KeepAlive.MaxIdleConnsPerHost)
			}},
		}
	)

	if testing.Verbose() {
		logger.Configure("debug", "[test-vulcan-newbackend] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	for _, t := range tests {
		ba, e := v.NewBackend(t.resource)
		b, ok := ba.(*backend)
		if is.NoError(e) && is.NotNil(ba) && is.True(ok, "Could not cast to backend") {
			t.assertions(b)
		}
	}
}
