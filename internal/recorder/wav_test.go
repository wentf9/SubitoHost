package recorder

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWavWriterRIFFTag(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.wav")

	w, err := NewWavWriter(path, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var tag [4]byte
	if err := binary.Read(f, binary.LittleEndian, &tag); err != nil {
		t.Fatalf("reading RIFF tag: %v", err)
	}
	if string(tag[:]) != "RIFF" {
		t.Errorf("tag = %q, want RIFF", string(tag[:]))
	}
}

func TestWavWriterSamplesAndSize(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.wav")

	w, err := NewWavWriter(path, 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	// 4 interleaved stereo samples = 2 frames: [L0, R0, L1, R1]
	samples := []int16{100, 200, 300, 400}
	if err := w.Write(samples); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Total size: 44-byte header + 4 samples * 2 bytes = 52
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 52 {
		t.Errorf("file size = %d, want 52", info.Size())
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// ChunkSize at offset 4 = fileSize - 8 = 44
	if _, err := f.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var chunkSize uint32
	if err := binary.Read(f, binary.LittleEndian, &chunkSize); err != nil {
		t.Fatalf("reading ChunkSize: %v", err)
	}
	if chunkSize != 44 {
		t.Errorf("ChunkSize = %d, want 44", chunkSize)
	}

	// Subchunk2Size at offset 40 = dataBytes = 8
	if _, err := f.Seek(40, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var sub2 uint32
	if err := binary.Read(f, binary.LittleEndian, &sub2); err != nil {
		t.Fatalf("reading Subchunk2Size: %v", err)
	}
	if sub2 != 8 {
		t.Errorf("Subchunk2Size = %d, want 8", sub2)
	}

	// PCM data at offset 44
	if _, err := f.Seek(44, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var got [4]int16
	if err := binary.Read(f, binary.LittleEndian, &got); err != nil {
		t.Fatalf("reading PCM samples: %v", err)
	}
	if got != [4]int16{100, 200, 300, 400} {
		t.Errorf("samples = %v, want [100 200 300 400]", got)
	}
}
