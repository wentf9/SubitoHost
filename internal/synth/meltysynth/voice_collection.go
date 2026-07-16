package meltysynth

import "math"

type voiceCollection struct {
	synthesizer      *Synthesizer
	voices           []*voice
	activeVoiceCount int32
	maxActive        int32
	fadeTails        []voiceFadeTail
	fadeTailSamples  int32
}

type voiceFadeTail struct {
	active     bool
	position   int32
	left       float32
	right      float32
	slopeLeft  float32
	slopeRight float32
}

func newVoiceCollection(s *Synthesizer, maxActiveVoiceCount int32) *voiceCollection {
	result := &voiceCollection{
		synthesizer:     s,
		voices:          make([]*voice, maxActiveVoiceCount),
		maxActive:       maxActiveVoiceCount,
		fadeTails:       make([]voiceFadeTail, max(int(maxActiveVoiceCount)*2, 16)),
		fadeTailSamples: int32(calcClamp(float32(s.SampleRate)/500, 32, 128)), // 2 ms
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
				vc.addFadeTail(voice)
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

	// At maximum polyphony, preserve the old voice's boundary waveform in an
	// independent preallocated tail before reusing its full synthesis state.
	candidate := vc.findStealCandidate()
	if candidate != nil {
		vc.addFadeTail(candidate)
		return candidate
	}

	// Absolute fallback: return nil, note is dropped.
	return nil
}

// findStealCandidate returns the lowest-priority active voice.
func (vc *voiceCollection) findStealCandidate() *voice {
	var candidate *voice = nil
	var lowestPriority float32 = math.MaxFloat32
	for i := int32(0); i < vc.activeVoiceCount; i++ {
		voice := vc.voices[i]
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

func (vc *voiceCollection) addFadeTail(voice *voice) {
	if len(voice.block) < 2 {
		return
	}
	last := len(voice.block) - 1
	previous := last - 1
	lastSample := voice.block[last]
	previousSample := voice.block[previous]

	var target *voiceFadeTail
	for i := range vc.fadeTails {
		if !vc.fadeTails[i].active {
			target = &vc.fadeTails[i]
			break
		}
	}
	if target == nil {
		// This requires more than two complete polyphony turnovers within 2 ms.
		// Replace the quietest remaining tail rather than allocating or touching
		// a live Voice from the realtime path.
		target = &vc.fadeTails[0]
		for i := 1; i < len(vc.fadeTails); i++ {
			if vc.fadeTails[i].position > target.position {
				target = &vc.fadeTails[i]
			}
		}
	}

	target.active = true
	target.position = 0
	target.left = lastSample * voice.currentMixGainLeft
	target.right = lastSample * voice.currentMixGainRight
	target.slopeLeft = (lastSample - previousSample) * voice.currentMixGainLeft
	target.slopeRight = (lastSample - previousSample) * voice.currentMixGainRight
}

func (vc *voiceCollection) renderFadeTails(left, right []float32, masterVolume float32) {
	for tailIndex := range vc.fadeTails {
		tail := &vc.fadeTails[tailIndex]
		if !tail.active {
			continue
		}
		for i := range left {
			if tail.position >= vc.fadeTailSamples {
				tail.active = false
				break
			}
			step := tail.position + 1
			phase := float64(step) / float64(vc.fadeTailSamples)
			window := float32(0.5 * (1 + math.Cos(math.Pi*phase)))
			leftValue := calcClamp(tail.left+tail.slopeLeft*float32(step), -1, 1)
			rightValue := calcClamp(tail.right+tail.slopeRight*float32(step), -1, 1)
			left[i] += masterVolume * leftValue * window
			right[i] += masterVolume * rightValue * window
			tail.position++
		}
		if tail.position >= vc.fadeTailSamples {
			tail.active = false
		}
	}
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
	for i := int32(0); i < vc.activeVoiceCount; i++ {
		vc.addFadeTail(vc.voices[i])
	}
	vc.activeVoiceCount = 0
}

func (vc *voiceCollection) reset() {
	vc.activeVoiceCount = 0
	for i := range vc.fadeTails {
		vc.fadeTails[i].active = false
	}
}
