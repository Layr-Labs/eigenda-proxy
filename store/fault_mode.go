package store

import (
	"encoding/json"
	"fmt"
	"os"
)

type Mode int

const (
	HonestMode = iota
	ByzantineFaultMode
	IntervalByzFaultMode
	Unknown

	AllPolicyKey = "all"
)

func StringToMode(s string) Mode {
	switch s {
	case "honest":
		return HonestMode
	case "byzantine":
		return ByzantineFaultMode
	case "interval_byzantine":
		return IntervalByzFaultMode
	default:
		return Unknown
	}
}

// Behavior represents a system actor return behavior within a L2 (e.g, sequencer, challenger, etc)
// this is key for testing fraud proofs in the case where we want force a commitment chain poster updater 
// (e.g, arbitrum MakeNode validator, op-proposer) to post an invalid machine state proof that can be challenged
// where bisection is resolved by a READPREIMAGE opcode for an eigenda pre-image type.
type Behavior struct {
	Mode     Mode `json:"mode"`
	Interval uint `json:"interval"` // only used for IntervalByzantine
}

type FaultConfig struct {
	Actors map[string]Behavior
}

func (cfg *FaultConfig) AllPolicyExists() bool {
	_, exists := cfg.Actors[AllPolicyKey]
	return exists
}

func LoadFaultConfig(path string) (*FaultConfig, error) {
	println(fmt.Sprintf("Loading config from %s", path))
	jsonFile, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()

	var config map[string]interface{}
	err = json.NewDecoder(jsonFile).Decode(&config)
	if err != nil {
		return nil, err
	}

	// translate config to the expected format
	actors := make(map[string]Behavior)
	for actor, behavior := range config {
		behaviorMap := behavior.(map[string]interface{})
		actors[actor] = Behavior{
			Mode:     StringToMode(behaviorMap["mode"].(string)),
			Interval: uint(behaviorMap["interval"].(float64)),
		}
	}

	println(fmt.Sprintf("Config: %v", config))

	return &FaultConfig{actors}, nil
}
