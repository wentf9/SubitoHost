package ringbuf

import (
	"sync"
	"testing"
)

func TestWriteRead(t *testing.T) {
	rb := New[int](4)
	if !rb.Write(1) {
		t.Fatal("Write to empty buffer should succeed")
	}
	if !rb.Write(2) {
		t.Fatal("Write should succeed")
	}

	v, ok := rb.Read()
	if !ok || v != 1 {
		t.Fatalf("Read = (%d, %v), want (1, true)", v, ok)
	}
	v, ok = rb.Read()
	if !ok || v != 2 {
		t.Fatalf("Read = (%d, %v), want (2, true)", v, ok)
	}
	_, ok = rb.Read()
	if ok {
		t.Fatal("Read from empty buffer should return false")
	}
}

func TestFull(t *testing.T) {
	rb := New[int](4) // rounds up to 4
	for i := range 4 {
		if !rb.Write(i) {
			t.Fatalf("Write(%d) should succeed", i)
		}
	}
	if rb.Write(99) {
		t.Fatal("Write to full buffer should fail")
	}
}

func TestConcurrentSPSC(t *testing.T) {
	rb := New[int](1024)
	count := 100_000
	var wg sync.WaitGroup
	wg.Add(2)

	// Producer
	go func() {
		defer wg.Done()
		for i := range count {
			for !rb.Write(i) {
				// spin until space available
			}
		}
	}()

	// Consumer
	got := make([]int, 0, count)
	go func() {
		defer wg.Done()
		for len(got) < count {
			if v, ok := rb.Read(); ok {
				got = append(got, v)
			}
		}
	}()

	wg.Wait()
	for i, v := range got {
		if v != i {
			t.Fatalf("got[%d] = %d, want %d", i, v, i)
		}
	}
}
