//go:build !linux

package synth

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

var (
	otoCtx     *oto.Context
	otoCtxOnce sync.Once
	otoCtxErr  error
)

type otoAudioDevice struct {
	player *oto.Player
}

func openPlatformAudioDevice(
	renderer *AudioRenderer,
	sampleRate, frames, _ int,
	deviceBuffer time.Duration,
) (audioDevice, time.Duration, string, error) {
	var creationErr error
	otoCtxOnce.Do(func() {
		options := &oto.NewContextOptions{
			SampleRate:      sampleRate,
			ChannelCount:    2,
			Format:          oto.FormatFloat32LE,
			BufferSize:      deviceBuffer,
			ApplicationName: "SubitoHost",
		}
		var ready chan struct{}
		otoCtx, ready, creationErr = oto.NewContext(options)
		if creationErr != nil {
			otoCtxErr = creationErr
			return
		}
		<-ready
	})
	if otoCtxErr != nil {
		return nil, 0, "oto", fmt.Errorf("oto context initialization failed: %w", otoCtxErr)
	}
	if creationErr != nil {
		return nil, 0, "oto", fmt.Errorf("oto context initialization failed: %w", creationErr)
	}

	player := otoCtx.NewPlayer(renderer)
	player.SetBufferSize(frames * 2 * 4)
	player.Play()
	readAhead := time.Duration(int64(time.Second) * int64(frames) / int64(sampleRate))
	return &otoAudioDevice{player: player}, deviceBuffer + readAhead, "oto", nil
}

func (d *otoAudioDevice) Close() {
	if d.player != nil {
		d.player.Pause()
	}
}
