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

func TestMultipleProducers(t *testing.T) {
	const producers = 8
	const perProducer = 2000
	rb := New[int](256)
	var producersDone sync.WaitGroup
	producersDone.Add(producers)
	for producer := 0; producer < producers; producer++ {
		go func(producer int) {
			defer producersDone.Done()
			for i := 0; i < perProducer; i++ {
				value := producer*perProducer + i
				for !rb.Write(value) {
				}
			}
		}(producer)
	}

	seen := make([]bool, producers*perProducer)
	for read := 0; read < len(seen); {
		value, ok := rb.Read()
		if !ok {
			continue
		}
		if value < 0 || value >= len(seen) || seen[value] {
			t.Fatalf("invalid or duplicate value %d", value)
		}
		seen[value] = true
		read++
	}
	producersDone.Wait()
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

func TestLenAndCapacityTelemetry(t *testing.T) {
	rb := New[int](3)
	if rb.Cap() != 4 || rb.Len() != 0 {
		t.Fatalf("initial telemetry = len %d cap %d, want 0/4", rb.Len(), rb.Cap())
	}
	rb.Write(1)
	rb.Write(2)
	if rb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", rb.Len())
	}
	rb.Read()
	if rb.Len() != 1 {
		t.Fatalf("Len() after read = %d, want 1", rb.Len())
	}
}

func BenchmarkMPSCQueueParallel(b *testing.B) {
	rb := New[int](4096)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				rb.Read()
			}
		}
	}()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for !rb.Write(1) {
			}
		}
	})
	close(done)
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
