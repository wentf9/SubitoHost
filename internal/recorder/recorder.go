package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/midi"
	"github.com/wentf9/subitohost/internal/setlist"
	"github.com/wentf9/subitohost/internal/util"
)

// Recorder manages recording of processed MIDI events for a single session segment.
type Recorder struct {
	cfg      config.Recording
	audioCfg config.Audio
	smf      *SMFWriter
	midPath  string
	wavPath  string
	profile  *setlist.Profile
}

// New creates a Recorder with the given configuration.
func New(cfg config.Recording, audioCfg config.Audio) *Recorder {
	return &Recorder{cfg: cfg, audioCfg: audioCfg}
}

// Start opens the SMF buffer and sets output file paths under OutputDir.
// Creates the output directory if it does not exist.
func (r *Recorder) Start(profile *setlist.Profile) error {
	dir := util.ExpandHome(r.cfg.OutputDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create recording dir: %w", err)
	}
	base := filepath.Join(dir,
		time.Now().Format("2006-01-02T15-04-05")+"_"+sanitizeName(profile.Name))
	r.midPath = base + ".mid"
	r.wavPath = base + ".wav"
	r.smf = NewSMFWriter()
	r.profile = profile
	return nil
}

// Stop finalizes the SMF file to disk and returns (midPath, wavPath, nil).
func (r *Recorder) Stop() (midPath, wavPath string, err error) {
	if r.smf == nil {
		return "", "", fmt.Errorf("recorder not started")
	}
	if err := r.smf.Flush(r.midPath); err != nil {
		return "", "", fmt.Errorf("flush smf: %w", err)
	}
	r.smf = nil
	return r.midPath, r.wavPath, nil
}

// Feed adds a processed MIDI event to the in-memory SMF buffer.
// No-op if Start has not been called or Stop has already been called.
func (r *Recorder) Feed(ev midi.Event) {
	if r.smf != nil {
		r.smf.AddEvent(ev)
	}
}

// Profile returns the setlist profile that was active when Start was called.
func (r *Recorder) Profile() *setlist.Profile {
	return r.profile
}

// AudioCfg returns the audio configuration stored at construction time.
func (r *Recorder) AudioCfg() config.Audio {
	return r.audioCfg
}

func sanitizeName(name string) string {
	var out []byte
	hasAlphaNum := false
	for _, c := range []byte(name) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			out = append(out, c)
			hasAlphaNum = true
		case c == ' ':
			out = append(out, '_')
		}
	}
	if !hasAlphaNum {
		return "recording"
	}
	return string(out)
}
