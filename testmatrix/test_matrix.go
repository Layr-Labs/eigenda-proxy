package testmatrix

// TestMatrix consists of a list of dimensions. Each dimension has a name, and list of potential values.
// The TestMatrix knows how to create a list of test configurations which includes every possible combination of
// the test dimension values.
type TestMatrix struct {
	dimensions []Dimension
}

// Dimension defines an aspect of a test with multiple potential values.
type Dimension struct {
	// key is the name of the config value with multiple potential values
	key    string
	values []any
}

// NewDimension creates a new test dimension, with a key and list of potential values
func NewDimension(key string, values []any) Dimension {
	return Dimension{
		key:    key,
		values: values,
	}
}

// NewTestMatrix creates a new test matrix
func NewTestMatrix() *TestMatrix {
	return &TestMatrix{
		dimensions: make([]Dimension, 0),
	}
}

// AddDimension adds a new dimension to the test matrix
func (tm *TestMatrix) AddDimension(dimension Dimension) {
	tm.dimensions = append(tm.dimensions, dimension)
}

// GenerateTestConfigurations produces a list of test configurations based on the defined TestMatrix dimensions.
func (tm *TestMatrix) GenerateTestConfigurations() []TestConfiguration {
	if len(tm.dimensions) == 0 {
		return nil
	}

	// start with a single empty test configuration
	configs := []TestConfiguration{NewTestConfiguration()}

	// for each defined dimension, we will multiply the existing list of configurations by the number of possible values
	// for the new dimension
	for _, dimension := range tm.dimensions {
		if len(dimension.values) == 0 {
			continue
		}

		var expandedConfigs []TestConfiguration
		for _, existingConfig := range configs {
			for _, dimensionValue := range dimension.values {
				newConfig := existingConfig.Copy()
				newConfig.AddKeyValue(NewConfigKeyValue(dimension.key, dimensionValue))

				expandedConfigs = append(expandedConfigs, newConfig)
			}
		}

		configs = expandedConfigs
	}

	return configs
}
