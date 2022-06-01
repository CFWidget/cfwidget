package main

import (
	"github.com/cfwidget/cfwidget/env"
	"sync"
	"time"
)

type CachedResponse struct {
	Data        interface{}
	ExpireAt    time.Time
	Status      int
	ContentType string
}

var cacheTtl time.Duration
var memcache = sync.Map{}

func init() {
	envCache := env.Get("CACHE_TTL")
	cacheTtl = time.Hour
	if envCache != "" {
		var err error
		cacheTtl, err = time.ParseDuration(envCache)
		if err != nil {
			panic(err)
		}
	}

	go func() {
		cleanCache()
	}()
}

func GetFromCache(site, key string) (CachedResponse, bool) {
	val, exists := memcache.Load(site + ":" + key)
	if !exists {
		return CachedResponse{}, false
	}

	res, ok := val.(CachedResponse)
	if !ok || time.Now().After(res.ExpireAt) {
		memcache.Delete(site + ":" + key)
		return CachedResponse{}, false
	}

	return res, true
}

func SetInCache(site, key string, status int, contentType string, data interface{}) time.Time {
	cache := CachedResponse{Data: data, Status: status, ExpireAt: time.Now().Add(cacheTtl), ContentType: contentType}
	memcache.Store(site+":"+key, cache)
	return cache.ExpireAt
}

func RemoveFromCache(site, key string) {
	memcache.Delete(site + ":" + key)
}

func cleanCache() {
	memcache.Range(func(k, v interface{}) bool {
		res, ok := v.(CachedResponse)
		if !ok || time.Now().After(res.ExpireAt) {
			memcache.Delete(k)
			return true
		}

		return true
	})
}
