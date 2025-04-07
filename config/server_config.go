package config

// ServerConfig ... Config for the proxy HTTP server
type ServerConfig struct {
	Host string
	Port int
	// AdminEndpointsEnabled controls whether administrative HTTP endpoints are exposed.
	// When false (default), administrative endpoints like /admin/v2-dispersal are not registered.
	// When true, these endpoints are available for runtime configuration changes.
	AdminEndpointsEnabled bool
}
