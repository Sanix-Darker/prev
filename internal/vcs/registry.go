package vcs

import (
	"fmt"
	"sort"
	"sync"
)

// Factory creates a VCSProvider from a token and base URL.
type Factory func(token, baseURL string) (VCSProvider, error)

// Registry is a thread-safe store of VCS provider factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

var globalRegistry = NewRegistry()

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a VCS provider factory under the given name.
// It panics if the name is already registered.
func (r *Registry) Register(name string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("vcs: factory already registered for %q", name))
	}
	r.factories[name] = f
}

// Get creates a VCS provider instance by name.
func (r *Registry) Get(name string, token, baseURL string) (VCSProvider, error) {
	r.mu.RLock()
	f, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("vcs: unknown provider %q (registered: %v)", name, r.Names())
	}
	return f(token, baseURL)
}

// Names returns a sorted list of registered VCS provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Register adds a VCS provider factory to the global registry.
func Register(name string, f Factory) {
	globalRegistry.Register(name, f)
}

// Get resolves a VCS provider by name from the global registry.
func Get(name string, token, baseURL string) (VCSProvider, error) {
	return globalRegistry.Get(name, token, baseURL)
}

// Names returns all registered VCS provider names from the global registry.
func Names() []string {
	return globalRegistry.Names()
}
