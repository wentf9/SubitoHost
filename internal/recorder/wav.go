package recorder

import (
	"encoding/binary"
	"os"
)

// WavWriter writes 16-bit PCM WAV files with configurable channel count.
// Call NewWavWriter, Write one or more times, then Close to finalize sizes.
type WavWriter struct {
	f          *os.File
	sampleRate int
	channels   int
	dataBytes  uint32
}

// NewWavWriter creates a WAV file and writes a 44-byte PCM header with placeholder sizes.
func NewWavWriter(path string, sampleRate, channels int) (*WavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &WavWriter{f: f, sampleRate: sampleRate, channels: channels}
	if err := w.writeHeader(); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	return w, nil
}

func (w *WavWriter) writeHeader() error {
	bits := uint16(16)
	byteRate := uint32(w.sampleRate) * uint32(w.channels) * uint32(bits) / 8
	blockAlign := uint16(w.channels) * bits / 8
	return binary.Write(w.f, binary.LittleEndian, struct {
		RIFF          [4]byte
		ChunkSize     uint32 // patched on Close
		WAVE          [4]byte
		Fmt           [4]byte
		Subchunk1Size uint32
		AudioFormat   uint16
		NumChannels   uint16
		SampleRate    uint32
		ByteRate      uint32
		BlockAlign    uint16
		BitsPerSample uint16
		Data          [4]byte
		Subchunk2Size uint32 // patched on Close
	}{
		RIFF:          [4]byte{'R', 'I', 'F', 'F'},
		WAVE:          [4]byte{'W', 'A', 'V', 'E'},
		Fmt:           [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1,
		NumChannels:   uint16(w.channels),
		SampleRate:    uint32(w.sampleRate),
		ByteRate:      byteRate,
		BlockAlign:    blockAlign,
		BitsPerSample: bits,
		Data:          [4]byte{'d', 'a', 't', 'a'},
	})
}

// Write appends interleaved int16 PCM samples to the file.
func (w *WavWriter) Write(samples []int16) error {
	if err := binary.Write(w.f, binary.LittleEndian, samples); err != nil {
		return err
	}
	w.dataBytes += uint32(len(samples)) * 2
	return nil
}

// Close patches ChunkSize and Subchunk2Size, then closes the file.
func (w *WavWriter) Close() error {
	if w.f == nil {
		return nil
	}
	// Subchunk2Size at offset 40
	if _, err := w.f.Seek(40, 0); err != nil {
		return err
	}
	if err := binary.Write(w.f, binary.LittleEndian, w.dataBytes); err != nil {
		return err
	}
	// ChunkSize at offset 4: 36 + dataBytes
	if _, err := w.f.Seek(4, 0); err != nil {
		return err
	}
	if err := binary.Write(w.f, binary.LittleEndian, uint32(36)+w.dataBytes); err != nil {
		return err
	}
	err := w.f.Close()
	w.f = nil
	return err
}
