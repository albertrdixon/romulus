package store

import (
	"strconv"
	"testing"
)

var result []string

func BenchmarkEtcdStore(b *testing.B) {
	s := NewEtcdStore([]string{"http://127.0.0.1:2379"})
	benchmarkStore(b, s)
}

func BenchmarkMemStore(b *testing.B) {
	s := NewMemStore()
	benchmarkStore(b, s)
}

func benchmarkStore(b *testing.B, s Store) {
	r := []string{}
	for n := 0; n < b.N; n++ {
		t := strconv.Itoa(n)
		deleteKeys(s, t)
		s.Set(t, []byte(t), 0)
		s.Get(t)
		s.GetMulti(t)
		r, _ = s.Keys(t)
	}
	result = r
}
