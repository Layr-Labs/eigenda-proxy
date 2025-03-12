package testmatrix

import (
	"fmt"
	"strings"
)

// TestConfiguration defines a specific set of configuration values for a test.
type TestConfiguration struct {
	// configKeyValues contains the values of the config fields for the test configuration
	configKeyValues []ConfigKeyValue
	// configMap maps config key to index where the value exists in configKeyValues. we maintain both a map and a slice,
	// since we want to maintain consistent ordering with config key value pairs
	configMap map[string]int
}

// ConfigKeyValue is a single concrete configuration key / value pair
type ConfigKeyValue struct {
	// key is the name of the config field
	key string
	// value is the concrete value for the field
	value any
}

// NewConfigKeyValue creates a new ConfigKeyValue pair
func NewConfigKeyValue(key string, value any) ConfigKeyValue {
	return ConfigKeyValue{
		key:   key,
		value: value,
	}
}

// NewTestConfiguration creates a new empty test configuration
func NewTestConfiguration() TestConfiguration {
	return TestConfiguration{
		make([]ConfigKeyValue, 0),
		make(map[string]int),
	}
}

// AddKeyValue adds a new ConfigKeyValue to the TestConfiguration
func (tc *TestConfiguration) AddKeyValue(configKeyValue ConfigKeyValue) {
	tc.configMap[configKeyValue.key] = len(tc.configKeyValues)
	tc.configKeyValues = append(tc.configKeyValues, configKeyValue)
}

// GetValue returns the value defined in the TestConfiguration for the input config key
//
// This method will panic if the requested key isn't found in the TestConfiguration
func (tc *TestConfiguration) GetValue(key string) any {
	index, ok := tc.configMap[key]
	if !ok {
		panicMessage := fmt.Sprintf("key %s not found in test configuration", key)
		panic(panicMessage)
	}
	return tc.configKeyValues[index].value
}

// Copy creates a copy of the TestConfiguration
func (tc *TestConfiguration) Copy() TestConfiguration {
	configCopy := NewTestConfiguration()
	for _, configKeyValue := range tc.configKeyValues {
		configCopy.AddKeyValue(configKeyValue)
	}

	return configCopy
}

// ToString returns a multiline string representation of the TestConfiguration
func (tc *TestConfiguration) ToString() string {
	stringBuilder := strings.Builder{}

	stringBuilder.WriteString("\t{")
	for _, configPair := range tc.configKeyValues {
		stringBuilder.WriteString(configPair.key)
		stringBuilder.WriteString(": ")
		stringBuilder.WriteString(fmt.Sprintf("%v", configPair.value))
		stringBuilder.WriteString(", ")
	}
	stringBuilder.WriteString("},\n")

	return stringBuilder.String()
}

// TestConfigurationsToString produces a multiline string representing a list of test configurations
func TestConfigurationsToString(testConfigurations []TestConfiguration) string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("[\n")
	for _, config := range testConfigurations {
		stringBuilder.WriteString(config.ToString())
	}

	stringBuilder.WriteString("]")

	return stringBuilder.String()
}
