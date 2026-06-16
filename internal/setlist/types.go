package setlist

// Setlist is a complete performance configuration.
type Setlist struct {
	ID       string    `json:"setlist_id"`
	Name     string    `json:"name"`
	Profiles []Profile `json:"profiles"`
}

// Profile is a single-song configuration within a setlist.
type Profile struct {
	ID          string       `json:"profile_id"`
	Name        string       `json:"name"`
	SFPath      string       `json:"soundfont_path"`
	Programs    []Program    `json:"programs"`
	Rules       []RuleConfig `json:"rules"`
	NextTrigger *Trigger     `json:"next_profile_trigger,omitempty"`
}

// Program selects a bank+program on a MIDI channel within a SoundFont.
type Program struct {
	Channel int `json:"channel"`
	Bank    int `json:"bank"`
	Program int `json:"program"`
}

// RuleConfig is the JSON representation of a routing rule.
// Which fields are relevant depends on Type.
type RuleConfig struct {
	ID            string    `json:"rule_id"`
	Type          string    `json:"type"`
	Range         [2]string `json:"range,omitempty"`
	TargetChannel *int      `json:"target_channel,omitempty"`
	Value         *int      `json:"value,omitempty"`
	Curve         string    `json:"curve,omitempty"`
	CCNumbers     []int     `json:"cc_numbers,omitempty"`
	FromCC        *int      `json:"from_cc,omitempty"`
	ToCC          *int      `json:"to_cc,omitempty"`
	Intervals     []int     `json:"intervals,omitempty"`
}

// Trigger defines a MIDI condition that triggers a profile switch.
type Trigger struct {
	Type       string `json:"type"`
	CCNumber   int    `json:"cc_number"`
	CCValueMin int    `json:"cc_value_min"`
}
