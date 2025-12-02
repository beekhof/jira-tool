package jira

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Cache holds cached Jira data
type Cache struct {
	Priorities []Priority             `json:"priorities,omitempty"`
	Sprints    []SprintParsed         `json:"sprints,omitempty"`
	Releases   []ReleaseParsed        `json:"releases,omitempty"`
	Users      map[string][]User      `json:"users,omitempty"`      // keyed by search query
	Components map[string][]Component `json:"components,omitempty"` // keyed by project key
	mu         sync.RWMutex
	path       string
}

// GetCachePath returns the path for the cache file
// If configDir is empty, uses the default ~/.jira-tool
func GetCachePath(configDir string) string {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "./.jira-tool/cache.json"
		}
		configDir = filepath.Join(homeDir, ".jira-tool")
	}
	return filepath.Join(configDir, "cache.json")
}

// NewCache creates a new cache instance
func NewCache(path string) *Cache {
	return &Cache{
		Users:      make(map[string][]User),
		Components: make(map[string][]Component),
		path:       path,
	}
}

// Load loads the cache from disk
func (c *Cache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Cache file doesn't exist yet, that's okay
			return nil
		}
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Initialize Users map if it's nil
	if c.Users == nil {
		c.Users = make(map[string][]User)
	}

	// Initialize Components map if it's nil
	if c.Components == nil {
		c.Components = make(map[string][]Component)
	}

	return nil
}

// Save saves the cache to disk
func (c *Cache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Clear clears the cache
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Priorities = nil
	c.Sprints = nil
	c.Releases = nil
	c.Users = make(map[string][]User)
	c.Components = make(map[string][]Component)

	// Delete the cache file
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	return nil
}

// ClearComponentsForProject clears the cached components for a specific project
func (c *Cache) ClearComponentsForProject(projectKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Components != nil {
		delete(c.Components, projectKey)
		// Save the updated cache
		_ = c.saveUnlocked() // Ignore save errors
	}
}

// saveUnlocked saves the cache without acquiring the lock (assumes lock is already held)
func (c *Cache) saveUnlocked() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}
