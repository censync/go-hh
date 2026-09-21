package hh_test

import (
	"fmt"
	"sync"

	hh "github.com/censync/go-hh"
)

type digestCache struct {
	mu      sync.Mutex
	digests map[string]hh.BaseDigest // keyed by the decoded input, not by its spelling
}

func (c *digestCache) ofAddress(address []byte) (hh.BaseDigest, error) {
	c.mu.Lock()
	d, ok := c.digests[string(address)]
	c.mu.Unlock()
	if ok {
		return d, nil
	}
	d, err := hh.NewBaseDigest(address) // outside the lock: this is the slow call
	if err != nil {
		return hh.BaseDigest{}, err
	}
	c.mu.Lock()
	if c.digests == nil {
		c.digests = make(map[string]hh.BaseDigest)
	}
	c.digests[string(address)] = d
	c.mu.Unlock()
	return d, nil
}

// The base digest is the slow step and it is public, so a long-running program
// keeps it per input. The zero value of this cache is ready to use. Bound the
// map if the inputs come from outside.
func Example_digestCache() {
	var cache digestCache
	address := []byte{
		0x5a, 0xAe, 0xb6, 0x05, 0x3F, 0x3E, 0x94, 0xC9, 0xb9, 0xA0,
		0x9f, 0x33, 0x66, 0x94, 0x35, 0xE7, 0xEf, 0x1B, 0xeA, 0xed,
	}
	first, err := cache.ofAddress(address)
	if err != nil {
		fmt.Println(err)
		return
	}
	again, _ := cache.ofAddress(address)
	_, err = cache.ofAddress(nil)
	fmt.Println(first == again, len(cache.digests), hh.Universal(first).Tag())
	fmt.Println(err)
	// Output:
	// true 1 TKSPVH
	// hh: empty_input: the input has no bytes
}
