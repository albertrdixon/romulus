package traefik

import (
	"bytes"

	"github.com/timelinelabs/pkg/store"
)

type Store struct {
	store.Store
}

func (s Store) GetString(key string) (string, error) {
	b := new(bytes.Buffer)
	_, er := b.ReadFrom(s.Get(key))
	return b.String(), er
}

func (s Store) Exists(key string) error {
	b := new(bytes.Buffer)
	_, er := b.ReadFrom(s.Get(key))
	return er
}
