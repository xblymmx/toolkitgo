package context

import (
	"net/http"
	"time"
	"sync"
)

/**
context for http.Request
 */

var (
	data  = make(map[*http.Request]map[interface{}]interface{})
	datat = make(map[*http.Request]int64)
	mux   sync.RWMutex
)

func Get(r *http.Request, key interface{}) interface{} {
	mux.RLock()
	defer mux.RUnlock()

	if ctx, ok := data[r]; ok {
		return ctx[key]
	}

	return nil
}

func Set(r *http.Request, key interface{}, val interface{}) {
	mux.Lock()
	defer mux.Unlock()

	if data[r] == nil {
		data[r] = make(map[interface{}]interface{})
		datat[r] = time.Now().Unix()
	}

	data[r][key] = val
}

func GetOK(r *http.Request, key interface{}) (interface{}, bool) {
	mux.RLock()
	mux.RUnlock()

	if ctx, ok := data[r]; ok {
		val, found := ctx[key]
		return val, found
	}

	return nil, false
}

func GetAll(r *http.Request) map[interface{}]interface{} {
	mux.RLock()
	mux.RUnlock()

	if ctx, ok := data[r]; ok {
		m := make(map[interface{}]interface{}, len(ctx))
		for k, v := range data[r] {
			m[k] = v
		}

		return m
	}

	return nil
}

func Delete(r *http.Request, key interface{}) {
	mux.Lock()
	defer mux.Unlock()

	if ctx, ok := data[r]; ok {
		delete(ctx, key)
	}
}

func Clear(r *http.Request) {
	mux.Lock()
	defer mux.Unlock()

	clear(r)
}

func clear(r *http.Request) {
	delete(data, r)
	delete(datat, r)
}

func Purge(maxAge int) int {
	mux.Lock()
	defer mux.Unlock()

	cnt := 0
	if maxAge <= 0 {
		cnt = len(data)
		data = make(map[*http.Request]map[interface{}]interface{})
		datat = make(map[*http.Request]int64)
	} else {
		min := time.Now().Unix() - int64(maxAge)
		for r := range datat {
			if datat[r] < min {
				cnt++
				clear(r)
			}
		}
	}
	return cnt
}

func ClearHandler(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer Clear(r)
		handler.ServeHTTP(w, r)
	})
}
