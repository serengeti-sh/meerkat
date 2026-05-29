// Package config loads and validates application configuration.
//
// Configuration is sourced from YAML files (via Viper) and unmarshaled into
// strongly-typed structs with mapstructure tags. It also provides helper
// methods such as DSN() for database connection strings.
package config
