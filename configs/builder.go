package configs

import (
	"sync"
)

type (
	ConfBuilder struct {
		m   map[string]string
		def Config
	}
)

var cbMU sync.RWMutex

func NewBuilder(def func(string) string, m map[string]string) ConfBuilder {
	return ConfBuilder{m: m, def: def}
}
func (b ConfBuilder) With(k, v string) ConfBuilder {
	return b.WithMap(map[string]string{k: v})
}
func (b ConfBuilder) WithMap(m map[string]string) ConfBuilder {
	b2 := b.clone()
	cbMU.Lock()
	defer cbMU.Unlock()
	for k, v := range m {
		b2.m[k] = v
	}
	return b2
}
func (b ConfBuilder) WithDefaultFN(fn func(string) string) ConfBuilder {
	b2 := b.clone()
	b2.def = fn
	return b2
}
func (b ConfBuilder) Build() Config {
	var (
		b2  = b.clone()
		m   = b2.m
		def = b2.def
	)
	return func(k string) string {
		if v, ok := m[k]; ok {
			return v
		}
		if def != nil {
			return def(k)
		}
		return ""
	}
}
func (b ConfBuilder) clone() ConfBuilder {
	cbMU.RLock()
	defer cbMU.RUnlock()
	var m = make(map[string]string)
	for k, v := range b.m {
		m[k] = v
	}
	return ConfBuilder{
		m: m,
		def: func(k string) string {
			if b.def != nil {
				return b.def(k)
			}
			return ""
		},
	}
}
