package warden

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SecDuckOps/agent/internal/domain/security"
)

// CacheEntry stores a cached policy decision with expiration.
type CacheEntry struct {
	Decision  security.PolicyDecision
	ExpiresAt time.Time
}

// PolicyCache is a thread-safe LRU cache for Cedar policy decisions.
type PolicyCache struct {
	mu       sync.RWMutex
	entries  map[string]CacheEntry
	order    []string // LRU order tracking
	maxSize  int
	ttl      time.Duration
}

// NewPolicyCache creates a new LRU policy cache.
func NewPolicyCache(maxSize int, ttl time.Duration) *PolicyCache {
	return &PolicyCache{
		entries: make(map[string]CacheEntry, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached decision if it exists and hasn't expired.
func (c *PolicyCache) Get(key string) (security.PolicyDecision, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists || time.Now().After(entry.ExpiresAt) {
		if exists {
			c.mu.Lock()
			delete(c.entries, key)
			c.removeFromOrder(key)
			c.mu.Unlock()
		}
		return security.PolicyDecision{}, false
	}

	// Move to front (most recently used)
	c.mu.Lock()
	c.moveToFront(key)
	c.mu.Unlock()

	return entry.Decision, true
}

// Set stores a policy decision in the cache.
func (c *PolicyCache) Set(key string, decision security.PolicyDecision) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it
	if _, exists := c.entries[key]; exists {
		c.entries[key] = CacheEntry{
			Decision:  decision,
			ExpiresAt: time.Now().Add(c.ttl),
		}
		c.moveToFront(key)
		return
	}

	// Evict LRU if at capacity
	if len(c.entries) >= c.maxSize && len(c.order) > 0 {
		lruKey := c.order[len(c.order)-1]
		delete(c.entries, lruKey)
		c.order = c.order[:len(c.order)-1]
	}

	c.entries[key] = CacheEntry{
		Decision:  decision,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	c.order = append([]string{key}, c.order...)
}

// GenerateCacheKey creates a deterministic key from principal, facts, and AST.
func GenerateCacheKey(principalID string, facts map[string]interface{}, command string, args []string) string {
	h := sha256.New()
	h.Write([]byte(principalID))
	h.Write([]byte("|"))
	h.Write([]byte(command))
	h.Write([]byte("|"))
	h.Write([]byte(strings.Join(args, ",")))
	h.Write([]byte("|"))

	// Sort fact keys for determinism
	factKeys := make([]string, 0, len(facts))
	for k := range facts {
		factKeys = append(factKeys, k)
	}
	sort.Strings(factKeys)
	for _, k := range factKeys {
		h.Write([]byte(fmt.Sprintf("%s=%v", k, facts[k])))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Invalidate removes a specific key from the cache.
func (c *PolicyCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	c.removeFromOrder(key)
}

// Clear removes all entries from the cache.
func (c *PolicyCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]CacheEntry, c.maxSize)
	c.order = c.order[:0]
}

// Size returns the current number of entries in the cache.
func (c *PolicyCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *PolicyCache) moveToFront(key string) {
	c.removeFromOrder(key)
	c.order = append([]string{key}, c.order...)
}

func (c *PolicyCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
