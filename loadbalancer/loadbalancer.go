package loadbalancer

import (
	"errors"

	"github.com/timelinelabs/romulus/kubernetes"
)

var (
	ErrUnexpectedFrontendType = errors.New("Frontend is of unexpected type")
	ErrUnexpectedBackendType  = errors.New("Backend is of unexpected type")
)

type LoadBalancer interface {
	NewFrontend(*kubernetes.Service) (Frontend, error)
	GetFrontend(string) (Frontend, error)
	UpsertFrontend(Frontend) error
	DeleteFrontend(Frontend) error
	NewBackend(*kubernetes.Service) (Backend, error)
	GetBackend(string) (Backend, error)
	UpsertBackend(Backend) error
	DeleteBackend(Backend) error
	NewServers(*kubernetes.Service) ([]Server, error)
	GetServers(string) ([]Server, error)
	UpsertServer(Backend, Server) error
	DeleteServer(Backend, Server) error
	NewMiddlewares(*kubernetes.Service) ([]Middleware, error)

	Kind() string
	Status() error
}

type LoadbalancerObject interface {
	GetID() string
}

type Frontend interface {
	LoadbalancerObject
	AddMiddleware(mid Middleware)
}
type Backend interface {
	LoadbalancerObject
	AddServer(srv Server)
}
type Server interface {
	LoadbalancerObject
}
type Middleware interface {
	LoadbalancerObject
}

type ServerMap map[string]Server
