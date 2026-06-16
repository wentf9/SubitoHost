package router

import (
	"fmt"

	"github.com/wentf9/subitohost/internal/midi"
	"github.com/wentf9/subitohost/internal/setlist"
)

// BuildRules converts a list of RuleConfig into Rule implementations.
// Validates that chord_expand, if present, is the last rule.
func BuildRules(cfgs []setlist.RuleConfig) ([]Rule, error) {
	rules := make([]Rule, 0, len(cfgs))
	for i, cfg := range cfgs {
		if cfg.Type == "chord_expand" && i != len(cfgs)-1 {
			return nil, fmt.Errorf("chord_expand must be the last rule (found at index %d of %d)", i, len(cfgs))
		}
		rule, err := BuildRule(cfg)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", cfg.ID, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// BuildRule converts a single RuleConfig into a Rule implementation.
func BuildRule(cfg setlist.RuleConfig) (Rule, error) {
	switch cfg.Type {
	case "key_split":
		return buildKeySplit(cfg)
	case "transpose":
		return buildTranspose(cfg)
	case "velocity_curve":
		return buildVelocityCurve(cfg)
	case "cc_filter":
		return buildCCFilter(cfg)
	case "cc_map":
		return buildCCMap(cfg)
	case "chord_expand":
		return buildChordExpand(cfg)
	default:
		return nil, fmt.Errorf("unknown rule type: %q", cfg.Type)
	}
}

func buildKeySplit(cfg setlist.RuleConfig) (Rule, error) {
	r, err := midi.ParseNoteRange(cfg.Range[0], cfg.Range[1])
	if err != nil {
		return nil, err
	}
	if cfg.TargetChannel == nil {
		return nil, fmt.Errorf("key_split requires target_channel")
	}
	return &KeySplitRule{Range: r, TargetChannel: *cfg.TargetChannel}, nil
}

func buildTranspose(cfg setlist.RuleConfig) (Rule, error) {
	r, err := midi.ParseNoteRange(cfg.Range[0], cfg.Range[1])
	if err != nil {
		return nil, err
	}
	ch := -1
	if cfg.TargetChannel != nil {
		ch = *cfg.TargetChannel
	}
	semitones := 0
	if cfg.Value != nil {
		semitones = *cfg.Value
	}
	return &TransposeRule{Range: r, Channel: ch, Semitones: semitones}, nil
}

func buildVelocityCurve(cfg setlist.RuleConfig) (Rule, error) {
	r, err := midi.ParseNoteRange(cfg.Range[0], cfg.Range[1])
	if err != nil {
		return nil, err
	}
	ch := -1
	if cfg.TargetChannel != nil {
		ch = *cfg.TargetChannel
	}
	var curve CurveType
	switch cfg.Curve {
	case "linear", "":
		curve = CurveLinear
	case "exponential":
		curve = CurveExponential
	case "logarithmic":
		curve = CurveLogarithmic
	default:
		return nil, fmt.Errorf("unknown curve: %q", cfg.Curve)
	}
	return &VelocityCurveRule{Range: r, Channel: ch, Curve: curve}, nil
}

func buildCCFilter(cfg setlist.RuleConfig) (Rule, error) {
	if len(cfg.CCNumbers) == 0 {
		return nil, fmt.Errorf("cc_filter requires cc_numbers")
	}
	return &CCFilterRule{BlockCCs: cfg.CCNumbers}, nil
}

func buildCCMap(cfg setlist.RuleConfig) (Rule, error) {
	if cfg.FromCC == nil || cfg.ToCC == nil {
		return nil, fmt.Errorf("cc_map requires from_cc and to_cc")
	}
	return &CCMapRule{FromCC: *cfg.FromCC, ToCC: *cfg.ToCC}, nil
}

func buildChordExpand(cfg setlist.RuleConfig) (Rule, error) {
	r, err := midi.ParseNoteRange(cfg.Range[0], cfg.Range[1])
	if err != nil {
		return nil, err
	}
	if len(cfg.Intervals) == 0 {
		return nil, fmt.Errorf("chord_expand requires intervals")
	}
	ch := -1
	if cfg.TargetChannel != nil {
		ch = *cfg.TargetChannel
	}
	return &ChordExpandRule{Range: r, Channel: ch, Intervals: cfg.Intervals}, nil
}
