package types

import (
	"sync"
)

const ClientIdKey string = "clientid"

type AudioPart []int32

type buffer []AudioPart

type AudioCache struct {
	sync.Mutex
	cache buffer
}

func NewAudioCache() *AudioCache {
	return &AudioCache{
		cache: buffer{},
	}
}

func (ac *AudioCache) Len() int {
	return len(ac.cache)
}

func (ac *AudioCache) Write(data AudioPart) {
	ac.Lock()
	defer ac.Unlock()
	ac.cache = append(ac.cache, data)
}

func (ac *AudioCache) Read() (AudioPart, bool) {
	ac.Lock()
	defer ac.Unlock()
	if len(ac.cache) == 0 {
		return nil, false
	}
	data := ac.cache[0]
	ac.cache = ac.cache[1:]
	return data, true
}
