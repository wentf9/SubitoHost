package ringbuf

import "sync/atomic"

// Buffer is a bounded lock-free multi-producer multi-consumer ring buffer.
// Size is rounded up to the next power of 2.
type Buffer[T any] struct {
	slots []slot[T]
	mask  uint64
	head  atomic.Uint64 // written by producer
	tail  atomic.Uint64 // written by consumer
}

type slot[T any] struct {
	sequence atomic.Uint64
	value    T
}

// New creates a ring buffer with capacity rounded up to the next power of 2.
func New[T any](size int) *Buffer[T] {
	n := nextPow2(size)
	b := &Buffer[T]{
		slots: make([]slot[T], n),
		mask:  uint64(n - 1),
	}
	for i := range b.slots {
		b.slots[i].sequence.Store(uint64(i))
	}
	return b
}

// Write enqueues an item. Returns false if the buffer is full.
func (b *Buffer[T]) Write(item T) bool {
	for {
		head := b.head.Load()
		s := &b.slots[head&b.mask]
		difference := int64(s.sequence.Load() - head)
		if difference == 0 {
			if b.head.CompareAndSwap(head, head+1) {
				s.value = item
				s.sequence.Store(head + 1)
				return true
			}
			continue
		}
		if difference < 0 {
			return false
		}
	}
}

// Read dequeues an item. Returns zero value and false if the buffer is empty.
func (b *Buffer[T]) Read() (T, bool) {
	for {
		tail := b.tail.Load()
		s := &b.slots[tail&b.mask]
		difference := int64(s.sequence.Load() - (tail + 1))
		if difference == 0 {
			if b.tail.CompareAndSwap(tail, tail+1) {
				item := s.value
				var zero T
				s.value = zero
				s.sequence.Store(tail + uint64(len(b.slots)))
				return item, true
			}
			continue
		}
		if difference < 0 {
			var zero T
			return zero, false
		}
	}
}

// Len returns an approximate instantaneous item count. It is intended for
// telemetry and must not be used to decide whether a subsequent operation can
// succeed under concurrency.
func (b *Buffer[T]) Len() int {
	head := b.head.Load()
	tail := b.tail.Load()
	if head <= tail {
		return 0
	}
	length := head - tail
	if length > uint64(len(b.slots)) {
		length = uint64(len(b.slots))
	}
	return int(length)
}

// Cap returns the fixed queue capacity.
func (b *Buffer[T]) Cap() int { return len(b.slots) }

func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}
