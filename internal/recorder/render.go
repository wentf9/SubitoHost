package recorder

import (
	"log"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/setlist"
	"github.com/wentf9/subitohost/internal/synth"
)

// RenderOffline creates a headless FluidSynth instance, loads midPath via
// fluid_player, and renders audio to wavPath in a tight loop.
// broadcastFn is called with ("record_stop", data) when done (or on error).
// Intended to be run in a background goroutine.
func RenderOffline(midPath, wavPath string, profile *setlist.Profile, audioCfg config.Audio, broadcastFn func(evType string, data map[string]interface{})) {
	fail := func(msg string) {
		log.Printf("RenderOffline: %s", msg)
		broadcastFn("record_stop", map[string]interface{}{"error": msg, "mid": midPath})
	}

	s, err := synth.NewHeadless(audioCfg)
	if err != nil {
		fail("create synth: " + err.Error())
		return
	}
	defer s.Close()

	if err := s.LoadSoundFont(profile.SFPath); err != nil {
		fail("load soundfont: " + err.Error())
		return
	}
	for _, p := range profile.Programs {
		s.ProgramChange(p.Channel, p.Bank, p.Program)
	}

	player, err := s.OpenPlayer(midPath)
	if err != nil {
		fail("open player: " + err.Error())
		return
	}
	defer player.Close()

	if err := player.Play(); err != nil {
		fail("play: " + err.Error())
		return
	}

	bufSize := audioCfg.BufferSize
	if bufSize <= 0 {
		bufSize = 128
	}
	wav, err := NewWavWriter(wavPath, audioCfg.SampleRate, 2)
	if err != nil {
		fail("create wav: " + err.Error())
		return
	}

	buf := make([]int16, bufSize*2)
	var renderErr error
	for !player.IsDone() {
		if err := s.WriteS16(buf); err != nil {
			log.Printf("RenderOffline: write_s16: %v", err)
			renderErr = err
			break
		}
		if err := wav.Write(buf); err != nil {
			log.Printf("RenderOffline: wav write: %v", err)
			renderErr = err
			break
		}
	}

	if err := wav.Close(); err != nil {
		fail("wav close: " + err.Error())
		return
	}

	if renderErr != nil {
		fail("render incomplete: " + renderErr.Error())
		return
	}

	broadcastFn("record_stop", map[string]interface{}{"wav": wavPath, "mid": midPath})
}
