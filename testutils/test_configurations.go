package testutils

import "fmt"

// GetBackendAndVersionTestConfigs returns a list of TestConfigs, with all possible combinations of v2 being
// enabled/disabled and memstore being enabled/disabled
func GetBackendAndVersionTestConfigs() []TestConfig {
	outputConfigs := make([]TestConfig, 0)
	outputConfigs = append(outputConfigs, NewTestConfig(false, false))
	outputConfigs = append(outputConfigs, NewTestConfig(true, false))
	outputConfigs = append(outputConfigs, NewTestConfig(false, true))
	outputConfigs = append(outputConfigs, NewTestConfig(true, true))

	return outputConfigs
}

// GetLocalOnlyTestConfigs returns two TestConfigs that use memstore, and have v2 alternately enabled and disabled
func GetLocalOnlyTestConfigs() []TestConfig {
	outputConfigs := make([]TestConfig, 0)
	outputConfigs = append(outputConfigs, NewTestConfig(true, false))
	outputConfigs = append(outputConfigs, NewTestConfig(true, true))

	return outputConfigs
}

// TestConfigString produces a string to represent a test configuration
//
// This method only includes the elements which are varied for a given test, so that iterations can be differentiated
func TestConfigString(testCfg TestConfig) string {
	return fmt.Sprintf("v2 enabled: %v, use memstore: %v", testCfg.UseV2, testCfg.UseMemory)
}
