package config

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ContextManager handles context operations
type ContextManager struct {
	config *Config
	loader *Loader
}

// NewContextManager creates a new context manager
func NewContextManager(configPath string) (*ContextManager, error) {
	loader := NewLoader(configPath)
	config, err := loader.Load()
	if err != nil {
		return nil, err
	}

	return &ContextManager{
		config: config,
		loader: loader,
	}, nil
}

// Create creates a new context
func (cm *ContextManager) Create(name, apiURL, apiKey, organization string) error {
	if name == "" {
		// Auto-name from organization slug
		name = organization
	}

	validator := NewValidator()

	if err := validator.ValidateContextName(name); err != nil {
		return err
	}

	if err := validator.ValidateAPIKey(apiKey); err != nil {
		return err
	}

	if err := validator.ValidateOrganization(organization); err != nil {
		return err
	}

	if err := validator.ValidateAPIURL(apiURL); err != nil {
		return err
	}

	// Normalize API URL
	if apiURL == "" {
		apiURL = "https://api.dotenv.com"
	}

	context := Context{
		Name:         name,
		APIURL:       apiURL,
		APIKey:       apiKey,
		Organization: organization,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastUpdate:   time.Now(),
	}

	if err := cm.config.AddContext(name, context); err != nil {
		return err
	}

	return cm.loader.Save(cm.config)
}

// Update updates an existing context
func (cm *ContextManager) Update(name string, updates map[string]interface{}) error {
	ctx, exists := cm.config.Contexts[name]
	if !exists {
		return fmt.Errorf("context '%s' not found", name)
	}

	validator := NewValidator()

	// Apply updates
	for key, value := range updates {
		switch key {
		case "api_url":
			if v, ok := value.(string); ok {
				if err := validator.ValidateAPIURL(v); err != nil {
					return err
				}
				ctx.APIURL = v
			}
		case "api_key":
			if v, ok := value.(string); ok {
				if err := validator.ValidateAPIKey(v); err != nil {
					return err
				}
				ctx.APIKey = v
			}
		case "organization":
			if v, ok := value.(string); ok {
				if err := validator.ValidateOrganization(v); err != nil {
					return err
				}
				ctx.Organization = v
			}
		case "metadata":
			if v, ok := value.(Metadata); ok {
				ctx.Metadata = v
			}
		}
	}

	ctx.UpdatedAt = time.Now()
	ctx.LastUpdate = time.Now()
	cm.config.Contexts[name] = ctx

	return cm.loader.Save(cm.config)
}

// Delete removes a context
func (cm *ContextManager) Delete(name string) error {
	if err := cm.config.RemoveContext(name); err != nil {
		return err
	}

	return cm.loader.Save(cm.config)
}

// Use sets the current context
func (cm *ContextManager) Use(name string) error {
	if err := cm.config.SetCurrentContext(name); err != nil {
		return err
	}

	return cm.loader.Save(cm.config)
}

// Rename renames a context
func (cm *ContextManager) Rename(oldName, newName string) error {
	validator := NewValidator()
	if err := validator.ValidateContextName(newName); err != nil {
		return err
	}

	if err := cm.config.RenameContext(oldName, newName); err != nil {
		return err
	}

	return cm.loader.Save(cm.config)
}

// List returns all contexts with details
func (cm *ContextManager) List() []ContextInfo {
	contexts := make([]ContextInfo, 0, len(cm.config.Contexts))

	for name, ctx := range cm.config.Contexts {
		info := ContextInfo{
			Name:         name,
			Organization: ctx.Organization,
			APIURL:       ctx.APIURL,
			Current:      name == cm.config.CurrentContext,
			CreatedAt:    ctx.CreatedAt,
			UpdatedAt:    ctx.UpdatedAt,
			LastUpdate:   ctx.LastUpdate,
		}

		if ctx.Metadata.UserEmail != "" {
			info.UserEmail = ctx.Metadata.UserEmail
		}

		contexts = append(contexts, info)
	}

	// Sort by name
	sort.Slice(contexts, func(i, j int) bool {
		return contexts[i].Name < contexts[j].Name
	})

	return contexts
}

// GetCurrent returns the current context
func (cm *ContextManager) GetCurrent() (*Context, error) {
	return cm.config.GetCurrentContext()
}

// GetContext returns a specific context
func (cm *ContextManager) GetContext(name string) (*Context, error) {
	ctx, exists := cm.config.Contexts[name]
	if !exists {
		return nil, fmt.Errorf("context '%s' not found", name)
	}
	return &ctx, nil
}

// ValidateContext checks if a context is valid
func (cm *ContextManager) ValidateContext(name string) error {
	ctx, exists := cm.config.Contexts[name]
	if !exists {
		return fmt.Errorf("context '%s' not found", name)
	}

	if ctx.APIKey == "" {
		return fmt.Errorf("context '%s' has no API key", name)
	}

	if ctx.Organization == "" {
		return fmt.Errorf("context '%s' has no organization", name)
	}

	return nil
}

// RefreshMetadata updates context metadata (called after successful API calls)
func (cm *ContextManager) RefreshMetadata(name string, metadata Metadata) error {
	ctx, exists := cm.config.Contexts[name]
	if !exists {
		return fmt.Errorf("context '%s' not found", name)
	}

	ctx.Metadata = metadata
	ctx.UpdatedAt = time.Now()
	ctx.LastUpdate = time.Now()
	cm.config.Contexts[name] = ctx

	return cm.loader.Save(cm.config)
}

// ContextInfo provides display information about a context
type ContextInfo struct {
	Name         string
	Organization string
	APIURL       string
	UserEmail    string
	Current      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastUpdate   time.Time
}

// String returns a formatted string representation
func (ci ContextInfo) String() string {
	current := ""
	if ci.Current {
		current = " (current)"
	}

	parts := []string{
		fmt.Sprintf("Name: %s%s", ci.Name, current),
		fmt.Sprintf("Organization: %s", ci.Organization),
	}

	if ci.UserEmail != "" {
		parts = append(parts, fmt.Sprintf("User: %s", ci.UserEmail))
	}

	if ci.APIURL != "https://api.dotenv.com" {
		parts = append(parts, fmt.Sprintf("API URL: %s", ci.APIURL))
	}

	parts = append(parts, fmt.Sprintf("Last updated: %s", ci.LastUpdate.Format("2006-01-02 15:04:05")))

	return strings.Join(parts, "\n")
}
