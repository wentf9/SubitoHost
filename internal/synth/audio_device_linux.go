//go:build linux

package synth

import (
	"fmt"
	"time"

	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
)

type pulseAudioDevice struct {
	client *pulse.Client
	stream *pulse.PlaybackStream
}

func openPlatformAudioDevice(
	renderer *AudioRenderer,
	sampleRate, _, _ int,
	requestedLatency time.Duration,
) (audioDevice, time.Duration, string, error) {
	client, err := pulse.NewClient(pulse.ClientApplicationName("SubitoHost"))
	if err != nil {
		return nil, 0, "pulseaudio", err
	}

	stream, err := client.NewPlayback(
		pulse.NewReader(renderer, proto.FormatFloat32LE),
		pulse.PlaybackMediaName("SubitoHost"),
		pulse.PlaybackStereo,
		pulse.PlaybackSampleRate(sampleRate),
		pulse.PlaybackLatency(requestedLatency.Seconds()),
	)
	if err != nil {
		client.Close()
		return nil, 0, "pulseaudio", err
	}
	stream.Start()

	negotiatedFrames := stream.BufferSize()
	latency := time.Duration(int64(time.Second) * int64(negotiatedFrames) / int64(sampleRate))
	backend := fmt.Sprintf("pulseaudio-direct negotiated_frames=%d", negotiatedFrames)
	return &pulseAudioDevice{client: client, stream: stream}, latency, backend, nil
}

func (d *pulseAudioDevice) Close() {
	if d.stream != nil {
		d.stream.Pause()
		d.stream.Close()
	}
	if d.client != nil {
		d.client.Close()
	}
}
