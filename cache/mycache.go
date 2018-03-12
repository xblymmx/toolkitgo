package cache

import (
	"time"
	"sync"
	"fmt"
	"io"
	"encoding/gob"
	"runtime"
)


type Item struct {
	Object     interface{}
	Expiration int64
}

func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	NoExpiration time.Duration = -1

	DefaultExpiration time.Duration = 0
)

type Cache struct {
	*cache
}

type cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{})
	janitor *janitor
}

func (c *cache) Set(key string, val interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}

	c.mu.Lock()
	c.items[key] = Item{
		Object:     val,
		Expiration: e,
	}
	c.mu.Unlock()
}

// method do not contain any lock ops
func (c *cache) set(key string, val interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}

	c.items[key] = Item{
		Object:     val,
		Expiration: e,
	}
}

func (c *cache) SetDefault(key string, val interface{}) {
	c.Set(key, val, DefaultExpiration)
}

func (c *cache) Add(key string, val interface{}, d time.Duration) error {
	c.mu.Lock()
	if _, found := c.get(key); found {
		c.mu.Unlock()
		return fmt.Errorf("item %s already exists", key)
	}

	c.set(key, val, d)
	c.mu.Unlock()
	return nil
}

func (c *cache) Replace(key string, val interface{}, d time.Duration) error {
	c.mu.Lock()
	if _, found := c.get(key); !found {
		c.mu.Unlock()
		return fmt.Errorf("item %s do not exists", key)
	}

	c.set(key, val, d)
	c.mu.Unlock()
	return nil
}

func (c *cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.items[key]
	if !found || item.Expired() {
		c.mu.RUnlock()
		return nil, false
	}


	c.mu.RUnlock()
	return item.Object, true
}

func (c *cache) GetWithExpiration(key string) (interface{}, time.Time, bool) {
	c.mu.RLock()
	item, found := c.items[key]
	if !found {
		c.mu.RUnlock()
		return nil, time.Time{}, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration { // expired
			c.mu.RUnlock()
			return nil, time.Time{}, false
		}

		// Return the item and the expiration time
		c.mu.RUnlock()
		return item.Object, time.Unix(0, item.Expiration), true
	}

	// exp < 0
	c.mu.RUnlock()
	return item.Object, time.Time{}, true
}


func (c *cache) get(key string) (interface{}, bool) {
	item, found := c.items[key]
	if !found {
		return nil, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration { // expired
			return nil, false
		}
	}

	return item.Object, true
}


func (c *cache) Increment(key string, n int64) error {
	c.mu.Lock()
	item, found := c.items[key]
	if !found || item.Expired() {
		return fmt.Errorf("item %s not found", key)
	}

	switch item.Object.(type) {
	case int:
		item.Object = item.Object.(int) + int(n)
	case int8:
		item.Object = item.Object.(int8) + int8(n)
	case int16:
		item.Object = item.Object.(int16) + int16(n)
	case int32:
		item.Object = item.Object.(int32) + int32(n)
	case int64:
		item.Object = item.Object.(int64) + n
	case uint:
		item.Object = item.Object.(uint) + uint(n)
	case uintptr:
		item.Object = item.Object.(uintptr) + uintptr(n)
	case uint8:
		item.Object = item.Object.(uint8) + uint8(n)
	case uint16:
		item.Object = item.Object.(uint16) + uint16(n)
	case uint32:
		item.Object = item.Object.(uint32) + uint32(n)
	case uint64:
		item.Object = item.Object.(uint64) + uint64(n)
	case float32:
		item.Object = item.Object.(float32) + float32(n)
	case float64:
		item.Object = item.Object.(float64) + float64(n)
	default:
		return fmt.Errorf("item %s is not numeric", key)
	}

	c.items[key] = item
	c.mu.Unlock()
	return nil
}

func (c *cache) Decrement(key string, n int64) error {
	c.mu.Lock()
	item, found := c.items[key]
	if !found || item.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("item %s not found", key)
	}
	switch item.Object.(type) {
	case int:
		item.Object = item.Object.(int) - int(n)
	case int8:
		item.Object = item.Object.(int8) - int8(n)
	case int16:
		item.Object = item.Object.(int16) - int16(n)
	case int32:
		item.Object = item.Object.(int32) - int32(n)
	case int64:
		item.Object = item.Object.(int64) - n
	case uint:
		item.Object = item.Object.(uint) - uint(n)
	case uintptr:
		item.Object = item.Object.(uintptr) - uintptr(n)
	case uint8:
		item.Object = item.Object.(uint8) - uint8(n)
	case uint16:
		item.Object = item.Object.(uint16) - uint16(n)
	case uint32:
		item.Object = item.Object.(uint32) - uint32(n)
	case uint64:
		item.Object = item.Object.(uint64) - uint64(n)
	case float32:
		item.Object = item.Object.(float32) - float32(n)
	case float64:
		item.Object = item.Object.(float64) - float64(n)
	default:
		c.mu.Unlock()
		return fmt.Errorf("The value for %s is not an integer", key)
	}
	c.items[key] = item
	c.mu.Unlock()
	return nil
}

func (c *cache) Delete(key string)  {
	c.mu.Lock()
	v, evicted := c.delete(key)
	c.mu.Unlock()

	if evicted {
		c.onEvicted(key, v)
	}
}

func (c *cache) delete(key string) (interface{}, bool) {
	if c.onEvicted != nil {
		if item, found := c.items[key]; found {
			delete(c.items, key)
			return item.Object, true
		}
	}

	delete(c.items, key)
	return nil, false
}

type KeyAndValue struct {
	key string
	value interface{}
}

func (c *cache) DeleteExpired() {
	var evictedItems []KeyAndValue
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.Expired() {
			v, evicted := c.delete(key)
			if evicted {
				evictedItems = append(evictedItems, KeyAndValue{key, v})
			}
		}
	}

	c.mu.Unlock()
	for _, kv := range evictedItems {
		c.onEvicted(kv.key, kv.value)
	}
}

func (c *cache) OnEvicted(f func(string, interface{})) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// Write the cache's items (using Gob) to an io.Writer.
func (c *cache) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error registering item types with Gob library")
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range c.items {
		gob.Register(item.Object)
	}

	err = enc.Encode(&c.items)
	return
}

func (c *cache) Items() map[string]Item {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]Item)

	for k, item := range c.items {
		if !item.Expired() {
			m[k] = item
		}
	}

	return m
}

func (c *cache) ItemCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]Item)
}

type janitor struct {
	Interval time.Duration
	stop chan bool
}

func (j *janitor) Run(c *cache) {
	tick := time.NewTicker(j.Interval)

	for {
		select {
		case <- tick.C:
			c.DeleteExpired()
		case <- j.stop:
			tick.Stop()
			return
		}
	}
}

func StopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func RunJanitor(c *cache, d time.Duration) {
	j := &janitor{
		Interval: d,
		stop: make(chan bool),
	}
	c.janitor = j

	go j.Run(c)
}

func newCache(d time.Duration, m map[string]Item) *cache {
	if d == 0 {
		d = -1
	}

	c := &cache{
		defaultExpiration: d,
		items: m,
	}

	return c
}

func newCacheWithJanitor(d time.Duration, interval time.Duration, m map[string]Item) *Cache {
	c := newCache(d, m)
	C := &Cache{c}

	if interval > 0 {
		RunJanitor(c, interval)
		runtime.SetFinalizer(C, StopJanitor)
	}
	return C
}

func NewCache(defaultExpiration time.Duration, interval time.Duration) *Cache {
	items := make(map[string]Item)
	return newCacheWithJanitor(defaultExpiration, interval, items)

}

func NewFrom(defaultExpiration time.Duration, interval time.Duration, items map[string]Item) *Cache {
	return newCacheWithJanitor(defaultExpiration, interval, items)
}



