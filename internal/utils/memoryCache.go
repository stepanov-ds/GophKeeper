package utils

import (
	"sync"
	"time"
)

// Item - кэшируемый элемент
type Item struct {
	Value      any
	Expiration int64
}

// MemoryCache - кэш в памяти
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]Item
}

// NewMemoryCache - новый экземпляр кэша
func NewMemoryCache(cleanupTime time.Duration) *MemoryCache {
	mc := &MemoryCache{
		items: make(map[string]Item),
	}
	// запуск горутины очистки кэша
	go mc.cleanup(cleanupTime)
	return mc
}

// Set - добавляет/обновляет элемент в кэше с указанием времени жизни
func (mc *MemoryCache) Set(key string, value any, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items[key] = Item{
		Value:      value,
		Expiration: time.Now().Add(duration).UnixNano(),
	}
}

// Get - получение элемента из кэша
func (mc *MemoryCache) Get(key string) (any, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, found := mc.items[key]
	if !found {
		return nil, false
	}

	// проверка на время жизни
	if time.Now().UnixNano() > item.Expiration {
		return nil, false
	}

	return item.Value, true
}

// Delete - удаление элемента из кэша
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.items, key)
}

// cleanup - очистка просроченных элементов из кэша
func (mc *MemoryCache) cleanup(n time.Duration) {
	ticker := time.NewTicker(n) // проверяем кэш через каждые n времени
	defer ticker.Stop()

	for range ticker.C {
		mc.mu.Lock()
		for key, item := range mc.items {
			if time.Now().UnixNano() > item.Expiration {
				delete(mc.items, key)
			}
		}
		mc.mu.Unlock()
	}
}
