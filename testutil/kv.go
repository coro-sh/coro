package testutil

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

type KVOption[K comparable, V any] func(r *KV[K, V])

type KVIndex[V any] struct {
	keyFn func(value V) string
	data  map[string]V
}

func WithIndex[K comparable, V any](name string, keyFn func(value V) string) KVOption[K, V] {
	return func(r *KV[K, V]) {
		r.indexes[name] = &KVIndex[V]{
			keyFn: keyFn,
			data:  map[string]V{},
		}
	}
}

// KV is key value data structure to create fake data stores with such as
// store.FakeEntityReadWriter.
type KV[K comparable, V any] struct {
	mu      *sync.RWMutex
	data    map[K]V
	indexes map[string]*KVIndex[V]
}

func NewKV[K comparable, V any](t *testing.T, opts ...KVOption[K, V]) *KV[K, V] {
	t.Helper()
	init := &KV[K, V]{
		mu:      new(sync.RWMutex),
		data:    map[K]V{},
		indexes: map[string]*KVIndex[V]{},
	}
	for _, opt := range opts {
		opt(init)
	}
	return init
}

func (k *KV[K, V]) Put(key K, value V) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[key] = value
	for _, idx := range k.indexes {
		idxKey := idx.keyFn(value)
		idx.data[idxKey] = value
	}
}

func (k *KV[K, V]) Get(key K) (V, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.data[key]
	return v, ok
}

func (k *KV[K, V]) GetByIndex(index string, key string) (V, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var v V

	idx, ok := k.indexes[index]
	if !ok {
		return v, false
	}

	v, ok = idx.data[key]
	return v, ok
}

func (k *KV[K, V]) Delete(key K) {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.data[key]
	if !ok {
		return
	}
	delete(k.data, key)
	for _, idx := range k.indexes {
		idxKey := idx.keyFn(value)
		delete(idx.data, idxKey)
	}
}

func (k *KV[K, V]) Range(fn func(key K, val V) bool) {
	for key, val := range k.data {
		k.mu.RLock()
		cont := fn(key, val)
		k.mu.RUnlock()
		if !cont {
			return
		}
	}
}

type PageFilter struct {
	Size   int32
	Cursor any
}

// List returns a list of values from the KV store in descending order.
// The filter is used to paginate the results returned. Skippers can optionally
// be provided to skip values from the list.
func (k *KV[K, V]) List(filter PageFilter, skippers ...func(K, V) bool) []V {
	k.mu.RLock()
	defer k.mu.RUnlock()

	var keys []K

	k.Range(func(key K, val V) bool {
		for _, skipper := range skippers {
			if skipper(key, val) {
				return true
			}
		}
		keys = append(keys, key)
		return true
	})

	// Sort the keys to ensure deterministic iteration
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprintf("%v", keys[i]) < fmt.Sprintf("%v", keys[j])
	})

	var result []V
	size := filter.Size

	count := int32(0)
	for _, key := range keys {
		if filter.Cursor != nil && fmt.Sprintf("%v", key) < fmt.Sprintf("%v", filter.Cursor) {
			// Skip until the cursor is passed
			continue
		}

		value, ok := k.Get(key)
		if !ok {
			continue
		}
		result = append(result, value)
		count++

		if size > 0 && count >= size {
			// Stop if size is reached
			break
		}
	}

	return result
}
