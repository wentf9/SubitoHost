package meltysynth

import "math"

type voiceCollection struct {
	synthesizer      *Synthesizer
	voices           []*voice
	activeVoiceCount int32 // number of active voices (including fading out)
	maxActive        int32 // max active voices excluding fade pool
}

func newVoiceCollection(s *Synthesizer, maxActiveVoiceCount int32) *voiceCollection {
	// Allocate extra slots for the fade pool: when a voice is stolen, it
	// continues to render one more block while fading out. The new voice
	// uses a fade pool slot so it can start immediately.
	fadePoolSize := maxActiveVoiceCount / 4
	if fadePoolSize < 8 {
		fadePoolSize = 8
	}
	totalSlots := maxActiveVoiceCount + fadePoolSize

	result := &voiceCollection{
		synthesizer: s,
		voices:      make([]*voice, totalSlots),
		maxActive:   maxActiveVoiceCount,
	}
	for i := 0; i < len(result.voices); i++ {
		result.voices[i] = newVoice(s)
	}
	result.activeVoiceCount = 0

	return result
}

func (vc *voiceCollection) requestNew(region *InstrumentRegion, channel int32) *voice {
	// If an exclusive class is assigned to the region, find a voice with the same class.
	// If found, reuse it to avoid playing multiple voices with the same class at a time.
	exclusiveClass := region.GetExclusiveClass()
	if exclusiveClass != 0 {
		for i := int32(0); i < vc.activeVoiceCount; i++ {
			voice := vc.voices[i]
			if voice.exclusiveClass == exclusiveClass && voice.channel == channel {
				return voice
			}
		}
	}

	// If we haven't reached the max active limit, use a free slot.
	if vc.activeVoiceCount < vc.maxActive {
		free := vc.voices[vc.activeVoiceCount]
		vc.activeVoiceCount++
		return free
	}

	// At max active voices. Try to use a fade pool slot if available.
	if int(vc.activeVoiceCount) < len(vc.voices) {
		// Steal the lowest priority non-fading voice.
		candidate := vc.findStealCandidate()
		if candidate != nil {
			candidate.voiceState = voice_FadingOut
			candidate.voiceLength = 0 // reset so FadingOut logic knows it's the first block
		}
		// Use a fade pool slot for the new voice
		free := vc.voices[vc.activeVoiceCount]
		vc.activeVoiceCount++
		return free
	}

	// All slots (including fade pool) are full. Fall back to direct reuse
	// of the lowest priority voice without fade-out.
	candidate := vc.findStealCandidate()
	if candidate != nil {
		return candidate
	}

	// Absolute fallback: return nil, note is dropped.
	return nil
}

// findStealCandidate returns the lowest-priority non-fading voice, or nil if
// all active voices are fading out.
func (vc *voiceCollection) findStealCandidate() *voice {
	var candidate *voice = nil
	var lowestPriority float32 = math.MaxFloat32
	for i := int32(0); i < vc.activeVoiceCount; i++ {
		voice := vc.voices[i]
		if voice.voiceState == voice_FadingOut {
			continue
		}
		priority := voice.getPriority()
		if priority < lowestPriority {
			lowestPriority = priority
			candidate = voice
		} else if priority == lowestPriority {
			// Same priority...
			// The older one should be more suitable for reuse.
			if voice.voiceLength > candidate.voiceLength {
				candidate = voice
			}
		}
	}
	return candidate
}

func (vc *voiceCollection) process() {
	var i int32

	for {
		if i == vc.activeVoiceCount {
			return
		}

		if vc.voices[i].process() {
			i++
		} else {
			vc.activeVoiceCount--

			tmp := vc.voices[i]
			vc.voices[i] = vc.voices[vc.activeVoiceCount]
			vc.voices[vc.activeVoiceCount] = tmp
		}
	}
}

func (vc *voiceCollection) clear() {
	vc.activeVoiceCount = 0
}
