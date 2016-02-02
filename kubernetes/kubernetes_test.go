package kubernetes

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

type fakeClient struct {
	*testclient.Fake
	*testclient.FakeExperimental
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		Fake:             testclient.NewSimpleFake(),
		FakeExperimental: testclient.NewSimpleFakeExp(),
	}
}

func (f *fakeClient) updateFake(objs ...runtime.Object) {
	f.Fake = testclient.NewSimpleFake(objs...)
}

func (f *fakeClient) updateFakeExp(objs ...runtime.Object) {
	f.FakeExperimental = testclient.NewSimpleFakeExp(objs...)
}

func objectFromFile(t *testing.T, name, kind string, obj runtime.Object) {
	var (
		file  = path.Join("test", fmt.Sprintf("%s-%s.yaml", name, kind))
		p, er = ioutil.ReadFile(file)
	)

	if assert.NoError(t, er) {
		require.NoError(t, yaml.Unmarshal(p, obj))
		// t.Logf("Endpoints: %v", spew.Sdump(obj))
	}
}
