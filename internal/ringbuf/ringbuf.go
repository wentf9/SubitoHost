package ringbuf

import "sync/atomic"

// Buffer is a lock-free single-producer single-consumer ring buffer.
// Size is rounded up to the next power of 2.
type Buffer[T any] struct {
	slots []T
	mask  uint64
	head  atomic.Uint64 // written by producer
	tail  atomic.Uint64 // written by consumer
}

// New creates a ring buffer with capacity rounded up to the next power of 2.
func New[T any](size int) *Buffer[T] {
	n := nextPow2(size)
	return &Buffer[T]{
		slots: make([]T, n),
		mask:  uint64(n - 1),
	}
}

// Write enqueues an item. Returns false if the buffer is full.
func (b *Buffer[T]) Write(item T) bool {
	head := b.head.Load()
	if head-b.tail.Load() >= uint64(len(b.slots)) {
		return false
	}
	b.slots[head&b.mask] = item
	b.head.Store(head + 1)
	return true
}

// Read dequeues an item. Returns zero value and false if the buffer is empty.
func (b *Buffer[T]) Read() (T, bool) {
	tail := b.tail.Load()
	if tail >= b.head.Load() {
		var zero T
		return zero, false
	}
	item := b.slots[tail&b.mask]
	b.tail.Store(tail + 1)
	return item, true
}

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
