package loadbalancer

import (
	kube "github.com/timelinelabs/romulus/kubernetes"
)

type Loadbalancer interface {
	NewFrontend(meta *kube.Metadata) Frontend
	GetFrontend(store Store) (Frontend, error)
	NewBackend(meta *kube.Metadata)
}
