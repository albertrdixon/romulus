package main

import "errors"

var (
	ErrUnexpectedFrontendType = errors.New("Frontend is of unexpected type")
	ErrUnexpectedBackendType  = errors.New("Backend is of unexpected type")
)

type LoadBalancer interface {
	NewFrontend(meta *Metadata) (Frontend, error)
	GetFrontend(meta *Metadata) (Frontend, error)
	UpsertFrontend(fr Frontend) error
	DeleteFrontend(fr Frontend) error
	NewBackend(meta *Metadata) (Backend, error)
	GetBackend(meta *Metadata) (Backend, error)
	UpsertBackend(ba Backend) error
	DeleteBackend(ba Backend) error
	NewServers(addr Addresses, meta *Metadata) ([]Server, error)
	GetServers(meta *Metadata) ([]Server, error)
	UpsertServer(ba Backend, srv Server) error
	DeleteServer(ba Backend, srv Server) error
	NewMiddlewares(meta *Metadata) ([]Middleware, error)

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
