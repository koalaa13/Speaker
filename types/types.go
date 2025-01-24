package types

import "sync"

type AudioPart []int32

type buffer []AudioPart

type AudioCache struct {
	mutex sync.Mutex
	cache buffer
}

func (ac *AudioCache) Len() int {
	return len(ac.cache)
}

func (ac *AudioCache) Write(data AudioPart) {
	ac.mutex.Lock()
	ac.cache = append(ac.cache, data)
	ac.mutex.Unlock()
}

func (ac *AudioCache) Read() AudioPart {
	ac.mutex.Lock()
	data := ac.cache[0]
	ac.cache = ac.cache[1:]
	ac.mutex.Unlock()
	return data
}
