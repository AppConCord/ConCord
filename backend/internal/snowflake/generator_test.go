package snowflake

import (
	"errors"
	"testing"
	"time"
)

func TestGeneratorLayoutAndSequence(t *testing.T) {
	t.Parallel()
	fixed := time.UnixMilli(EpochMillis + 123)
	generator, err := newGenerator(7, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	first, err := generator.Next()
	if err != nil {
		t.Fatal(err)
	}
	second, err := generator.Next()
	if err != nil {
		t.Fatal(err)
	}
	want := int64(123<<(nodeBits+sequenceBits) | 7<<sequenceBits)
	if first != want || second != want+1 {
		t.Fatalf("IDs = %d, %d; want %d, %d", first, second, want, want+1)
	}
}

func TestGeneratorRejectsUnsafeClock(t *testing.T) {
	t.Parallel()
	times := []time.Time{time.UnixMilli(EpochMillis + 2), time.UnixMilli(EpochMillis + 1)}
	index := 0
	generator, err := newGenerator(0, func() time.Time {
		value := times[index]
		index++
		return value
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Next(); !errors.Is(err, ErrClockMovedBackwards) {
		t.Fatalf("error = %v, want ErrClockMovedBackwards", err)
	}
}

func TestGeneratorValidation(t *testing.T) {
	t.Parallel()
	if _, err := NewGenerator(-1); err == nil {
		t.Fatal("expected invalid node ID error")
	}
	generator, err := newGenerator(0, func() time.Time { return time.UnixMilli(EpochMillis - 1) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generator.Next(); !errors.Is(err, ErrClockMovedBackwards) {
		t.Fatalf("error = %v, want ErrClockMovedBackwards", err)
	}
}
