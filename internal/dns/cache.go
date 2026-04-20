package dns

import (
	"net"
	"sync"
	"time"
)

type entry struct {
	Domain string
	Expiry time.Time
}

type Cache struct {
	mu         sync.RWMutex
	ipToDomain map[string]entry
}

func NewCache() *Cache {
	return &Cache{
		ipToDomain: make(map[string]entry),
	}
}

func (c *Cache) Add(domain string, ips []net.IP, ttl uint32) {
	if domain == "" || len(ips) == 0 {
		return
	}
	if ttl == 0 {
		ttl = 30
	}

	expiry := time.Now().Add(time.Duration(ttl) * time.Second)

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}
		c.ipToDomain[ip.String()] = entry{
			Domain: domain,
			Expiry: expiry,
		}
	}
}

func (c *Cache) Lookup(ip net.IP) (string, bool) {
	if ip == nil {
		return "", false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	key := ip.String()

	c.mu.RLock()
	e, ok := c.ipToDomain[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(e.Expiry) {
		c.mu.Lock()
		delete(c.ipToDomain, key)
		c.mu.Unlock()
		return "", false
	}
	return e.Domain, true
}
