package domain

import (
	"math/rand"
	"time"
)

// Snowflake-inspired ID generator
// Format: [41 bits timestamp][10 bits machine ID][12 bits sequence]
// Produces time-sortable, distributed-safe unique IDs

const (
	// Epoch: 2024-01-01 00:00:00 UTC (custom epoch to maximize time range)
	epoch int64 = 1704067200000 // milliseconds

	machineIDBits = 10
	sequenceBits  = 12

	maxMachineID = (1 << machineIDBits) - 1 // 1023
	maxSequence  = (1 << sequenceBits) - 1  // 4095

	machineIDShift  = sequenceBits
	timestampShift  = sequenceBits + machineIDBits
)

// Base62 alphabet (URL-safe, no ambiguous characters)
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// IDGenerator generates unique snowflake-style IDs and Base62 short codes
type IDGenerator struct {
	machineID    int64
	sequence     int64
	lastTimestamp int64
}

// NewIDGenerator creates a new ID generator with a random machine ID
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		machineID: int64(rand.Intn(int(maxMachineID) + 1)),
	}
}

// NextID generates the next unique snowflake ID
func (g *IDGenerator) NextID() int64 {
	now := currentMillis()

	if now == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// Sequence exhausted, wait for next millisecond
			for now <= g.lastTimestamp {
				now = currentMillis()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = now

	return ((now - epoch) << timestampShift) |
		(g.machineID << machineIDShift) |
		g.sequence
}

// ToBase62 converts a numeric ID to a Base62 short code
// Example: 1234567890 → "1LY7VK"
func ToBase62(id int64) string {
	if id == 0 {
		return "0"
	}

	result := make([]byte, 0, 11) // max 11 chars for int64
	for id > 0 {
		result = append(result, base62Alphabet[id%62])
		id /= 62
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// FromBase62 converts a Base62 short code back to numeric ID
func FromBase62(code string) int64 {
	var result int64
	for _, c := range code {
		result *= 62
		switch {
		case c >= '0' && c <= '9':
			result += int64(c - '0')
		case c >= 'A' && c <= 'Z':
			result += int64(c-'A') + 10
		case c >= 'a' && c <= 'z':
			result += int64(c-'a') + 36
		}
	}
	return result
}

func currentMillis() int64 {
	return time.Now().UnixNano() / 1e6
}
