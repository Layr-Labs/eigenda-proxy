package fault

import (
	"encoding/json"
	"os"
	"time"
)

type Mode int

const (
	Honest = iota
	Byzantine
	IntervalByzantine
)

// Behavior represents a system actor return behavior within a L2 (e.g, sequencer, challenger, etc)
type Behavior struct {
	Mode     Mode          `json:"mode"`
	Interval time.Duration `json:"interval"` // only used for IntervalByzantine
}

type Config struct {
	Actors map[string]Behavior
}

func LoadConfig(path string) (*Config, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()

	var config Config
	err = json.NewDecoder(jsonFile).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
