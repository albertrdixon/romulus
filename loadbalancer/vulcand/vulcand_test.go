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
		is = assert.New(te)
		v  = new(vulcan)
	)

	if testing.Verbose() {
		logger.Configure("debug", "[romulus-test] ", os.Stdout)
		defer logger.SetLevel("error")
	}

	r := kubernetes.NewResource("foo", "", map[string]string{
		"romulus/max_resp_size": "10Mi",
		"romulus/max_req_size":  "3Mi",
	})
	logger.Debugf(r.String())
	f, e := v.NewFrontend(r)
	if is.NoError(e) && is.NotNil(f) {
		fr := f.(*frontend)
		is.Equal(int64(10485760), fr.HTTPSettings().Limits.MaxRespBodyBytes)
		is.Equal(int64(3145728), fr.HTTPSettings().Limits.MaxMemBodyBytes)
		is.Equal("foo", f.GetID())
	}
}
