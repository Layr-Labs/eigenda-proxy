package testmatrix

import (
	"fmt"
	"strings"
)

// ConfigurationSet defines a specific set of configuration values for a test.
type ConfigurationSet struct {
	// configKeyValues contains the values of the config fields for the configuration set
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

// NewConfigurationSet creates a new empty configuration set
func NewConfigurationSet() ConfigurationSet {
	return ConfigurationSet{
		make([]ConfigKeyValue, 0),
		make(map[string]int),
	}
}

// AddKeyValue adds a new ConfigKeyValue to the TestConfiguration
func (tc *ConfigurationSet) AddKeyValue(configKeyValue ConfigKeyValue) {
	tc.configMap[configKeyValue.key] = len(tc.configKeyValues)
	tc.configKeyValues = append(tc.configKeyValues, configKeyValue)
}

// GetValue returns the value defined in the TestConfiguration for the input config key
//
// This method will panic if the requested key isn't found in the TestConfiguration
func (tc *ConfigurationSet) GetValue(key string) any {
	index, ok := tc.configMap[key]
	if !ok {
		panicMessage := fmt.Sprintf("key %s not found in configuration set", key)
		panic(panicMessage)
	}
	return tc.configKeyValues[index].value
}

// Copy creates a copy of the ConfigurationSet
func (tc *ConfigurationSet) Copy() ConfigurationSet {
	configCopy := NewConfigurationSet()
	for _, configKeyValue := range tc.configKeyValues {
		configCopy.AddKeyValue(configKeyValue)
	}

	return configCopy
}

// ToString returns a multiline string representation of the ConfigurationSet
func (tc *ConfigurationSet) ToString() string {
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

// ConfigurationSetsToString produces a multiline string representing a list of configuration sets
func ConfigurationSetsToString(testConfigurations []ConfigurationSet) string {
	stringBuilder := strings.Builder{}
	stringBuilder.WriteString("[\n")
	for _, config := range testConfigurations {
		stringBuilder.WriteString(config.ToString())
	}

	stringBuilder.WriteString("]")

	return stringBuilder.String()
}
