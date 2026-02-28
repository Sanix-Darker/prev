package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Store is a minimal configuration store that replaces viper.
// It supports dot-notation keys, YAML file loading, defaults, and env overrides.
type Store struct {
	data     map[string]interface{}
	defaults map[string]interface{}
}

// NewStore creates an empty Store.
func NewStore() *Store {
	return &Store{
		data:     make(map[string]interface{}),
		defaults: make(map[string]interface{}),
	}
}

// LoadYAMLFile reads a YAML config file into the store.
func (s *Store) LoadYAMLFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	s.data = flatten("", m)
	return nil
}

// Set stores a value under the given dot-notation key.
func (s *Store) Set(key string, value interface{}) {
	s.data[key] = value
}

// SetDefault sets a default value that is used when no explicit value exists.
func (s *Store) SetDefault(key string, value interface{}) {
	s.defaults[key] = value
}

// IsSet returns true if the key has an explicit (non-default) value.
func (s *Store) IsSet(key string) bool {
	_, ok := s.data[key]
	return ok
}

// Get returns the raw value for a key, checking data then defaults.
func (s *Store) Get(key string) (interface{}, bool) {
	if v, ok := s.data[key]; ok {
		return v, true
	}
	if v, ok := s.defaults[key]; ok {
		return v, true
	}
	return nil, false
}

// GetString returns the string value for a key.
func (s *Store) GetString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		return ""
	}
	return toString(v)
}

// GetInt returns the integer value for a key.
func (s *Store) GetInt(key string) int {
	v, ok := s.Get(key)
	if !ok {
		return 0
	}
	return toInt(v)
}

// GetBool returns the boolean value for a key.
func (s *Store) GetBool(key string) bool {
	v, ok := s.Get(key)
	if !ok {
		return false
	}
	return toBool(v)
}

// GetDuration returns the duration value for a key.
func (s *Store) GetDuration(key string) time.Duration {
	v, ok := s.Get(key)
	if !ok {
		return 0
	}
	return toDuration(v)
}

// GetStringSlice returns a string slice for a key.
func (s *Store) GetStringSlice(key string) []string {
	v, ok := s.Get(key)
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		out := make([]string, len(val))
		for i, item := range val {
			out[i] = toString(item)
		}
		return out
	case []string:
		return val
	default:
		return nil
	}
}

// Sub returns a new Store scoped to the given prefix.
// For example, Sub("providers.openai") returns a store where
// "api_key" maps to the original "providers.openai.api_key".
func (s *Store) Sub(prefix string) *Store {
	sub := NewStore()
	dot := prefix + "."
	for k, v := range s.data {
		if strings.HasPrefix(k, dot) {
			sub.data[strings.TrimPrefix(k, dot)] = v
		}
	}
	for k, v := range s.defaults {
		if strings.HasPrefix(k, dot) {
			sub.defaults[strings.TrimPrefix(k, dot)] = v
		}
	}
	// Return nil if the sub-store is completely empty (matches viper behavior).
	if len(sub.data) == 0 && len(sub.defaults) == 0 {
		return nil
	}
	return sub
}

// flatten converts a nested map into dot-notation keys.
func flatten(prefix string, m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			for fk, fv := range flatten(key, val) {
				out[fk] = fv
			}
		case map[interface{}]interface{}:
			// YAML sometimes produces map[interface{}]interface{}.
			converted := make(map[string]interface{}, len(val))
			for mk, mv := range val {
				converted[fmt.Sprint(mk)] = mv
			}
			for fk, fv := range flatten(key, converted) {
				out[fk] = fv
			}
		default:
			out[key] = v
		}
	}
	return out
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprint(val)
	}
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		b, _ := strconv.ParseBool(val)
		return b
	case int:
		return val != 0
	default:
		return false
	}
}

func toDuration(v interface{}) time.Duration {
	switch val := v.(type) {
	case time.Duration:
		return val
	case string:
		d, err := time.ParseDuration(val)
		if err != nil {
			return 0
		}
		return d
	case int:
		return time.Duration(val) * time.Millisecond
	case int64:
		return time.Duration(val) * time.Millisecond
	case float64:
		return time.Duration(val) * time.Millisecond
	default:
		return 0
	}
}
