package id

import (
	"sync"
	"time"
)

// SnowflakeIDGenerator generates unique IDs using snowflake algorithm
type SnowflakeIDGenerator struct {
	lastTime    int64
	sequence    int32
	machineID   int64
}

var generator *SnowflakeIDGenerator
var once sync.Once

// InitGenerator initializes the ID generator
func InitGenerator(machineID int32) {
	once.Do(func() {
		generator = &SnowflakeIDGenerator{
			machineID: int64(machineID),
		}
	})
}

// GenerateID generates a unique ID
func GenerateID() int64 {
	return generator.Next()
}

// Next generates the next ID
func (g *SnowflakeIDGenerator) Next() int64 {
	now := time.Now().UnixMilli()

	if now == g.lastTime {
		g.sequence = (g.sequence + 1) & 0xFFF
		if g.sequence == 0 {
			// Wait for next millisecond
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTime = now

	id := ((now-1288839720000)<<22) | (g.machineID << 12) | int64(g.sequence)
	return id
}

// BatchGenerate generates multiple IDs at once
func BatchGenerate(count int) []int64 {
	ids := make([]int64, count)
	for i := 0; i < count; i++ {
		ids[i] = GenerateID()
	}
	return ids
}

// SimpleIDGenerator provides a simple counter-based ID generator
type SimpleIDGenerator struct {
	counter int64
	mu      sync.Mutex
}

// NewSimpleIDGenerator creates a new simple ID generator
func NewSimpleIDGenerator() *SimpleIDGenerator {
	return &SimpleIDGenerator{}
}

// Next generates the next ID
func (g *SimpleIDGenerator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return g.counter
}
