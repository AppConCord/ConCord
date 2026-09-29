// Package snowflake generates signed 64-bit identifiers using Concord's custom epoch.
package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	// EpochMillis is 2026-01-01T00:00:00Z represented as Unix milliseconds.
	EpochMillis int64 = 1767225600000

	nodeBits      = 10
	sequenceBits  = 12
	maxNodeID     = (1 << nodeBits) - 1
	maxSequence   = (1 << sequenceBits) - 1
	timestampBits = 41
	maxTimestamp  = (1 << timestampBits) - 1
)

var (
	// ErrClockMovedBackwards indicates that generating an ID could risk a collision.
	ErrClockMovedBackwards = errors.New("clock moved backwards")
	// ErrEpochOverflow indicates that the configured Snowflake timestamp space is exhausted.
	ErrEpochOverflow = errors.New("snowflake epoch overflow")
)

// Generator creates concurrency-safe Snowflake IDs with a 41/10/12 bit layout.
type Generator struct {
	mu            sync.Mutex
	nodeID        int64
	lastTimestamp int64
	sequence      int64
	now           func() time.Time
}

// NewGenerator creates a generator for nodeID, which must be in the inclusive range 0..1023.
func NewGenerator(nodeID int64) (*Generator, error) {
	return newGenerator(nodeID, time.Now)
}

func newGenerator(nodeID int64, now func() time.Time) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("node ID must be between 0 and %d", maxNodeID)
	}
	if now == nil {
		return nil, errors.New("clock function is required")
	}
	return &Generator{nodeID: nodeID, now: now, lastTimestamp: -1}, nil
}

// Next returns the next unique Snowflake ID or an error if the clock is unsafe.
func (g *Generator) Next() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timestamp := g.now().UTC().UnixMilli() - EpochMillis
	if timestamp < 0 {
		return 0, fmt.Errorf("%w: current time precedes custom epoch", ErrClockMovedBackwards)
	}
	if timestamp < g.lastTimestamp {
		return 0, fmt.Errorf("%w: current=%d previous=%d", ErrClockMovedBackwards, timestamp, g.lastTimestamp)
	}
	if timestamp > maxTimestamp {
		return 0, ErrEpochOverflow
	}

	if timestamp == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			var err error
			timestamp, err = g.waitForNextMillisecond(timestamp)
			if err != nil {
				return 0, err
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = timestamp
	return (timestamp << (nodeBits + sequenceBits)) | (g.nodeID << sequenceBits) | g.sequence, nil
}

func (g *Generator) waitForNextMillisecond(previous int64) (int64, error) {
	for attempts := 0; attempts < 1000; attempts++ {
		timestamp := g.now().UTC().UnixMilli() - EpochMillis
		if timestamp < previous {
			return 0, ErrClockMovedBackwards
		}
		if timestamp > maxTimestamp {
			return 0, ErrEpochOverflow
		}
		if timestamp > previous {
			return timestamp, nil
		}
		time.Sleep(time.Millisecond)
	}
	return 0, errors.New("snowflake sequence exhausted while clock remained unchanged")
}
