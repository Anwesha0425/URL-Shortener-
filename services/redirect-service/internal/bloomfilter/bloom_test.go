package bloomfilter

import (
	"context"
	"testing"
)

func TestBloomFilter_BasicInsertAndTest(t *testing.T) {
	bf := NewBloomFilter(10_000, 0.01) // 10K elements, 1% FPR

	codes := []string{"aB3xY", "zK9mP", "Qr7nL", "Wx2sT"}
	for _, code := range codes {
		bf.Add(code)
	}

	// All added codes must be found (zero false negatives)
	for _, code := range codes {
		if !bf.Test(code) {
			t.Errorf("false negative: %s should be in filter", code)
		}
	}

	// A clearly non-existent code should NOT be found most of the time
	// (can't assert 100% — Bloom filters have false positives)
	if bf.Test("XXXXXXXXXX") {
		t.Log("false positive on 'XXXXXXXXXX' — statistically acceptable")
	}
}

func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	n := uint64(100_000)
	bf := NewBloomFilter(n, 0.01)

	// Insert n elements
	for i := uint64(0); i < n; i++ {
		bf.Add(string(rune('a' + i%26)) + string(rune('0'+i%10)))
	}

	// FPR should be close to 1%
	fpr := bf.FalsePositiveRate()
	if fpr > 0.02 { // allow up to 2% as a test tolerance
		t.Errorf("FPR too high: %.4f (expected <= 0.02)", fpr)
	}
	t.Logf("FPR for %d elements: %.4f%%", n, fpr*100)
}

func TestBloomFilter_ZeroFalseNegatives(t *testing.T) {
	bf := NewBloomFilter(1_000, 0.001)

	inserted := make([]string, 500)
	for i := range inserted {
		code := generateCode(i)
		inserted[i] = code
		bf.Add(code)
	}

	// Every inserted element MUST be found — no false negatives allowed
	for _, code := range inserted {
		if !bf.Test(code) {
			t.Fatalf("FATAL: false negative detected for %s — Bloom Filter is broken!", code)
		}
	}
}

func generateCode(i int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 5)
	for j := range code {
		code[j] = chars[(i+j*7)%len(chars)]
	}
	return string(code)
}

// BenchmarkBloomFilter_Test measures the cost of checking a Bloom filter.
// Should be < 1µs on modern hardware.
func BenchmarkBloomFilter_Test(b *testing.B) {
	bf := NewBloomFilter(1_000_000, 0.01)
	bf.Add("aB3xY")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = bf.Test("aB3xY")
	}
}

// Ensure the package builds (context import used in startup loader)
var _ = context.Background
