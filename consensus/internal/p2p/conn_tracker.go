package p2p

import (
	"container/list"
	"fmt"
	"net/netip"
	"sync"
	"time"
)

type connTracker struct {
	cache       map[netip.Addr]uint
	lastConnect map[netip.Addr]*list.Element
	expiry      list.List
	expiryTimer *time.Timer
	mutex       sync.RWMutex
	max         uint
	window      time.Duration
}

type connTrackerEntry struct {
	address netip.Addr
	last    time.Time
}

func newConnTracker(max uint, window time.Duration) *connTracker {
	return &connTracker{
		cache:       map[netip.Addr]uint{},
		lastConnect: map[netip.Addr]*list.Element{},
		max:         max,
		window:      window,
	}
}

func (rat *connTracker) Len() int {
	rat.mutex.RLock()
	defer rat.mutex.RUnlock()
	return len(rat.cache)
}

func (rat *connTracker) AddConn(addrPort netip.AddrPort) error {
	address := addrPort.Addr()
	rat.mutex.Lock()
	defer rat.mutex.Unlock()
	now := time.Now()
	rat.expireEntries(now)

	if num := rat.cache[address]; num >= rat.max {
		return fmt.Errorf("%q has %d connections [max=%d]", address, num, rat.max)
	} else if num == 0 {
		// if there is already at least one connection, check to
		// see if it was established before within the window,
		// and error if so.
		if last := rat.lastConnect[address]; last != nil && now.Sub(last.Value.(connTrackerEntry).last) < rat.window {
			return fmt.Errorf("%q tried to connect within window of last %s", address, rat.window)
		}
	}

	rat.cache[address]++
	if rat.window > 0 {
		entry := connTrackerEntry{address: address, last: now}
		if elem := rat.lastConnect[address]; elem != nil {
			elem.Value = entry
			rat.expiry.MoveToBack(elem)
		} else {
			rat.lastConnect[address] = rat.expiry.PushBack(entry)
		}
		if rat.expiryTimer == nil {
			rat.expiryTimer = time.AfterFunc(time.Until(rat.expiry.Front().Value.(connTrackerEntry).last.Add(rat.window)), rat.expire)
		}
	}

	return nil
}

func (rat *connTracker) RemoveConn(addrPort netip.AddrPort) {
	address := addrPort.Addr()
	rat.mutex.Lock()
	defer rat.mutex.Unlock()

	if num := rat.cache[address]; num > 0 {
		rat.cache[address]--
	}
	if num := rat.cache[address]; num <= 0 {
		delete(rat.cache, address)
	}

	rat.expireEntries(time.Now())
}

// expireEntries is called with mutex held. Each address owns exactly one entry,
// ordered by its latest successful connection, so refreshes cannot grow the queue.
func (rat *connTracker) expireEntries(now time.Time) {
	for elem := rat.expiry.Front(); elem != nil; elem = rat.expiry.Front() {
		entry := elem.Value.(connTrackerEntry)
		if now.Sub(entry.last) < rat.window {
			break
		}
		delete(rat.lastConnect, entry.address)
		rat.expiry.Remove(elem)
		if len(rat.lastConnect) == 0 {
			rat.lastConnect = map[netip.Addr]*list.Element{}
		}
	}
}

func (rat *connTracker) expire() {
	rat.mutex.Lock()
	defer rat.mutex.Unlock()
	rat.expireEntries(time.Now())
	if elem := rat.expiry.Front(); elem != nil {
		rat.expiryTimer.Reset(time.Until(elem.Value.(connTrackerEntry).last.Add(rat.window)))
	} else {
		rat.expiryTimer = nil
	}
}
