package htmlimages

import (
	"container/list"
	"sync"
)

// textCache - rewritten texts by their source's hash; past max bytes the
// least recently used go. Safe to use concurrently; a nil one keeps
// nothing.
type textCache struct {
	mu    sync.Mutex
	max   int
	size  int
	order *list.List // of *textEntry, the most recently used first
	byKey map[[32]byte]*list.Element
}

type textEntry struct {
	key [32]byte
	out string
}

func newTextCache(maxBytes int) *textCache {
	return &textCache{max: maxBytes, order: list.New(), byKey: map[[32]byte]*list.Element{}}
}

func (c *textCache) get(key [32]byte) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.byKey[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(el)
	return el.Value.(*textEntry).out, true
}

// put keeps out under key; one larger than the whole budget isn't kept.
func (c *textCache) put(key [32]byte, out string) {
	if c == nil || len(out) > c.max {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.byKey[key]; ok {
		c.order.MoveToFront(el)
		return
	}
	c.byKey[key] = c.order.PushFront(&textEntry{key: key, out: out})
	c.size += len(out)
	for c.size > c.max {
		last := c.order.Back()
		e := last.Value.(*textEntry)
		c.order.Remove(last)
		delete(c.byKey, e.key)
		c.size -= len(e.out)
	}
}

func (c *textCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	clear(c.byKey)
	c.size = 0
}
