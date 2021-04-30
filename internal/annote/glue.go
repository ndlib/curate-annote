package annote

import (
	"log"
	"sync"
	"time"
)

type ItemAnnotationInfo struct {
	// the item these annotations are for
	PID string

	// The Primary RAP is the one this user has uploaded. It's UUID is "" if
	// this user has not copied the item to the annotation service.
	Primary RAPInfo

	// the other RAPs are ones other users have uploaded
	Others []RAPInfo
}

type RAPInfo struct {
	UUID      string
	Count     int    // number of annotations on it
	Depositor string // username of who uploaded it
}

func GetAnnotationInfoForItem(pid string, username string) ItemAnnotationInfo {
	result := ItemAnnotationInfo{PID: pid}

	raps, err := Datasource.SearchItemUUID(pid, "", "")
	if err != nil {
		log.Println("GetAnnotationInfoForItem", err)
		return result
	}

	for _, rap := range raps {
		// exclude ones that are still processing?
		info := RAPInfo{
			UUID:      rap.UUID,
			Depositor: rap.Username,
			Count:     AnnotationCounts(rap.UUID),
		}
		if rap.Username == username {
			result.Primary = info
		} else {
			result.Others = append(result.Others, info)
		}
	}

	return result
}

type expiringCache struct {
	m        sync.Mutex
	data     map[string]expiringItem
	ttl      time.Duration
	inflight map[string]chan struct{}
	f        func(string) int
}

type expiringItem struct {
	expires time.Time
	value   int
}

func (e *expiringCache) Get(key string) int {
retry:
	e.m.Lock()
	if e.data == nil {
		e.data = make(map[string]expiringItem)
	}
	v, ok := e.data[key]
	// if there is an unexpired value, return it
	if ok && time.Now().Before(v.expires) {
		e.m.Unlock()
		return v.value
	}
	if e.inflight == nil {
		e.inflight = make(map[string]chan struct{})
	}
	// are we already getting this item?
	c, ok := e.inflight[key]
	if ok {
		// wait for previous request to finish
		e.m.Unlock()
		<-c
		goto retry
	}
	c = make(chan struct{})
	e.inflight[key] = c
	e.m.Unlock()

	n := e.f(key)

	e.m.Lock()
	delete(e.inflight, key)
	e.data[key] = expiringItem{
		expires: time.Now().Add(e.ttl),
		value:   n,
	}
	e.m.Unlock()
	close(c)
	return n
}

var (
	uuidcounts = expiringCache{ttl: 5 * time.Minute,
		f: func(key string) int {
			if AnnotationStore == nil {
				// there is no configured annotation store
				return 0
			}
			annos, err := AnnotationStore.AnnotationListByUUID(key)
			if err != nil {
				log.Println(err)
				return 0
			}
			if annos.Error != "" {
				log.Println(annos.Error)
				return 0
			}
			return len(annos.Annotations)
		},
	}
)

// AnnotationCounts will return the number of annotations on item uuid.
func AnnotationCounts(uuid string) int {
	return uuidcounts.Get(uuid)
}
