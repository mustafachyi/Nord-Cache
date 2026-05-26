package cache

import "sync/atomic"

type Data struct {
	RawPayload  []byte
	GzipPayload []byte
	ETag        string
}

type Store struct {
	data atomic.Pointer[Data]
}

func New() *Store {
	return &Store{}
}

func (s *Store) Get() *Data {
	return s.data.Load()
}

func (s *Store) Set(data *Data) {
	s.data.Store(data)
}
