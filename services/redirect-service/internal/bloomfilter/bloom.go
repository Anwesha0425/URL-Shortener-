package bloomfilter

import (
	"math"
	"sync"

	"github.com/spaolacci/murmur3"
)

/**
 * Bloom Filter — Cache Penetration Defense
 *
 * PROBLEM — Cache Penetration Attack:
 *   An attacker sends millions of requests for non-existent short codes:
 *     GET /xxxxxxxx  → Redis MISS → PostgreSQL SELECT (miss) → 404
 *     GET /yyyyyyyy  → Redis MISS → PostgreSQL SELECT (miss) → 404
 *     ... repeat 1M times/second
 *
 *   The database gets hammered with guaranteed misses — it will crash.
 *
 * SOLUTION — Bloom Filter:
 *   1. At startup, load ALL existing short_codes from PostgreSQL into the filter.
 *   2. On every request, check the filter FIRST (before Redis or DB).
 *   3. If filter says "DEFINITELY NOT IN SET" → return 404 immediately.
 *   4. If filter says "POSSIBLY IN SET"        → proceed to Redis/DB.
 *
 * Properties:
 *   - False Positive Rate: ~1% (filter says "exists" but it doesn't)
 *     → These still hit the DB, but only 1% of legitimate traffic does
 *   - False Negative Rate: 0% (filter will NEVER miss a real short code)
 *     → 100% safe: no real URLs will be blocked
 *
 * Memory:
 *   For 100M URLs with 1% false positive rate:
 *     m = -n*ln(p) / ln(2)^2 ≈ 958MB → Use Redis Bloom Filter in prod
 *     For 10M URLs: ~95MB in-memory (acceptable for a single pod)
 *
 * Implementation:
 *   - k = 7 hash functions (optimal for 1% FPR)
 *   - Uses MurmurHash3 (fast, non-cryptographic, good distribution)
 *   - Two independent hashes + linear combination: h_i(x) = h1(x) + i*h2(x)
 *   - Thread-safe via RWMutex (Add acquires write lock, Test acquires read lock)
 */

type BloomFilter struct {
	bits    []uint64 // Bit array stored in uint64 chunks
	m       uint64   // Total number of bits
	k       uint     // Number of hash functions
	mu      sync.RWMutex
	count   uint64   // Number of elements added
}

// NewBloomFilter creates a filter optimized for n elements at false positive rate p.
func NewBloomFilter(n uint64, p float64) *BloomFilter {
	m := optimalM(n, p)
	k := optimalK(m, n)
	return &BloomFilter{
		bits: make([]uint64, (m+63)/64),
		m:    m,
		k:    k,
	}
}

// Add inserts a short_code into the filter. Thread-safe.
func (bf *BloomFilter) Add(shortCode string) {
	h1, h2 := hashPair(shortCode)

	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := uint(0); i < bf.k; i++ {
		pos := (h1 + uint64(i)*h2) % bf.m
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
	bf.count++
}

// Test returns false if the short_code DEFINITELY does not exist.
// Returns true if it POSSIBLY exists (check Redis/DB to confirm).
// Thread-safe.
func (bf *BloomFilter) Test(shortCode string) bool {
	h1, h2 := hashPair(shortCode)

	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for i := uint(0); i < bf.k; i++ {
		pos := (h1 + uint64(i)*h2) % bf.m
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false // DEFINITELY not in set
		}
	}
	return true // POSSIBLY in set
}

// Count returns the number of elements added to the filter.
func (bf *BloomFilter) Count() uint64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.count
}

// FalsePositiveRate returns the current estimated false positive rate.
func (bf *BloomFilter) FalsePositiveRate() float64 {
	bf.mu.RLock()
	n := bf.count
	bf.mu.RUnlock()
	// Formula: (1 - e^(-k*n/m))^k
	exponent := -float64(bf.k) * float64(n) / float64(bf.m)
	return math.Pow(1-math.Exp(exponent), float64(bf.k))
}

// ── Internal hash functions ───────────────────────────────────────

// hashPair returns two independent 64-bit hashes using MurmurHash3.
// We use the "double hashing" technique to simulate k independent hashes.
func hashPair(key string) (uint64, uint64) {
	data := []byte(key)
	h1 := uint64(murmur3.Sum32(data))
	h2 := uint64(murmur3.Sum32WithSeed(data, 0xDEADBEEF))
	return h1, h2
}

// optimalM computes the optimal bit array size for n elements at FPR p.
// Formula: m = -n * ln(p) / (ln(2))^2
func optimalM(n uint64, p float64) uint64 {
	return uint64(math.Ceil(-float64(n) * math.Log(p) / (math.Log(2) * math.Log(2))))
}

// optimalK computes the optimal number of hash functions.
// Formula: k = (m/n) * ln(2)
func optimalK(m, n uint64) uint {
	return uint(math.Round(float64(m) / float64(n) * math.Log(2)))
}
