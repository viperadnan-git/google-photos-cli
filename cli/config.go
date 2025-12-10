package cli

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Config represents the persistent configuration
type Config struct {
	Credentials                   []string `json:"credentials" koanf:"credentials"`
	Selected                      string   `json:"selected" koanf:"selected"`
	Proxy                         string   `json:"proxy" koanf:"proxy"`
	UseQuota                      bool     `json:"useQuota" koanf:"use_quota"`
	Quality                       string   `json:"quality" koanf:"quality"` // "original" or "storage-saver"
	Recursive                     bool     `json:"recursive" koanf:"recursive"`
	ForceUpload                   bool     `json:"forceUpload" koanf:"force_upload"`
	UploadThreads                 int      `json:"uploadThreads" koanf:"upload_threads"`
	DeleteFromHost                bool     `json:"deleteFromHost" koanf:"delete_from_host"`
	DisableUnsupportedFilesFilter bool     `json:"disableUnsupportedFilesFilter" koanf:"disable_unsupported_files_filter"`
}

// DefaultConfig returns the default configuration values
func DefaultConfig() Config {
	return Config{
		UploadThreads: 3,
		Quality:       "original",
	}
}

// ConfigManager manages configuration loading and saving
type ConfigManager struct {
	config     Config
	configPath string
}

// NewConfigManager creates a new ConfigManager and loads the configuration from the given path
func NewConfigManager(configPath string) (*ConfigManager, error) {
	if configPath == "" {
		configPath = "gpcli.config"
	}

	m := &ConfigManager{
		configPath: configPath,
	}

	// Load config from file, or use defaults if file doesn't exist
	data, _ := os.ReadFile(configPath)
	if len(data) == 0 {
		m.config = DefaultConfig()
	} else {
		m.config = m.loadFromFile()
	}

	return m, nil
}

// GetConfig returns the current configuration
func (m *ConfigManager) GetConfig() Config {
	return m.config
}

// GetConfigPath returns the path to the config file
func (m *ConfigManager) GetConfigPath() string {
	return m.configPath
}

// Save persists the current configuration to disk
func (m *ConfigManager) Save() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(m.config, "koanf"), nil)
	if err != nil {
		slog.Error("failed to load config struct", "error", err)
		return err
	}
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		slog.Error("failed to marshal config", "error", err)
		return err
	}

	err = os.WriteFile(m.configPath, b, 0644)
	if err != nil {
		slog.Error("failed to write config file", "error", err)
		return err
	}

	return nil
}

// loadFromFile loads config from the config file
func (m *ConfigManager) loadFromFile() Config {
	var c Config
	k := koanf.New(".")
	if err := k.Load(file.Provider(m.configPath), yaml.Parser()); err != nil {
		slog.Error("error parsing app config", "error", err)
		return DefaultConfig()
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		slog.Error("error unmarshaling app config", "error", err)
		return DefaultConfig()
	}

	if c.UploadThreads < 1 {
		c.UploadThreads = DefaultConfig().UploadThreads
	}

	// Set default quality if not set
	if c.Quality == "" {
		c.Quality = DefaultConfig().Quality
	}

	return c
}

// ParseAuthString parses an auth string and returns url.Values (exported for CLI use)
func ParseAuthString(authString string) (url.Values, error) {
	return url.ParseQuery(authString)
}

// SetProxy updates the proxy setting
func (m *ConfigManager) SetProxy(proxy string) {
	m.config.Proxy = proxy
	m.Save()
}

// SetSelected updates the selected email
func (m *ConfigManager) SetSelected(email string) {
	m.config.Selected = email
	m.Save()
}

// SetUseQuota updates the use quota setting
func (m *ConfigManager) SetUseQuota(useQuota bool) {
	m.config.UseQuota = useQuota
	m.Save()
}

// SetQuality updates the quality setting
func (m *ConfigManager) SetQuality(quality string) {
	if quality == "original" || quality == "storage-saver" {
		m.config.Quality = quality
		m.Save()
	}
}

// SetRecursive updates the recursive setting
func (m *ConfigManager) SetRecursive(recursive bool) {
	m.config.Recursive = recursive
	m.Save()
}

// SetForceUpload updates the force upload setting
func (m *ConfigManager) SetForceUpload(forceUpload bool) {
	m.config.ForceUpload = forceUpload
	m.Save()
}

// SetDeleteFromHost updates the delete from host setting
func (m *ConfigManager) SetDeleteFromHost(deleteFromHost bool) {
	m.config.DeleteFromHost = deleteFromHost
	m.Save()
}

// SetDisableUnsupportedFilesFilter updates the filter setting
func (m *ConfigManager) SetDisableUnsupportedFilesFilter(disableUnsupportedFilesFilter bool) {
	m.config.DisableUnsupportedFilesFilter = disableUnsupportedFilesFilter
	m.Save()
}

// SetUploadThreads updates the upload threads setting
func (m *ConfigManager) SetUploadThreads(uploadThreads int) {
	if uploadThreads < 1 {
		return
	}
	m.config.UploadThreads = uploadThreads
	m.Save()
}

// AddCredentials adds a new credential to the config
func (m *ConfigManager) AddCredentials(newAuthString string) error {
	// Required fields that must be present in the auth string
	requiredFields := []string{
		"androidId",
		"app",
		"client_sig",
		"Email",
		"Token",
		"lang",
		"service",
	}

	// Parse the auth string
	params, err := url.ParseQuery(newAuthString)
	if err != nil {
		return fmt.Errorf("invalid auth string format: %v", err)
	}

	// Validate required fields
	var missingFields []string
	for _, field := range requiredFields {
		if params.Get(field) == "" {
			missingFields = append(missingFields, field)
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("auth string missing required fields: %v", missingFields)
	}

	// Get and validate email
	email := params.Get("Email")
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Check for duplicate email in existing credentials
	for _, cred := range m.config.Credentials {
		existingParams, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}
		if existingParams.Get("Email") == email {
			return fmt.Errorf("auth string with email %s already exists", email)
		}
	}

	// If validation passed, add the new credentials
	m.config.Credentials = append(m.config.Credentials, newAuthString)
	m.config.Selected = email
	m.Save()
	return nil
}

// RemoveCredentials removes a credential by email
func (m *ConfigManager) RemoveCredentials(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	// Find and remove the credential with matching email
	found := false
	var updatedCredentials []string

	for _, cred := range m.config.Credentials {
		params, err := url.ParseQuery(cred)
		if err != nil {
			continue // skip malformed entries
		}

		if params.Get("Email") == email {
			found = true
			continue // skip this credential (effectively removing it)
		}

		updatedCredentials = append(updatedCredentials, cred)
	}

	if !found {
		return fmt.Errorf("no credentials found for email %s", email)
	}

	// Update the configuration
	m.config.Credentials = updatedCredentials

	// If we're removing the currently selected credential, clear the selection
	if m.config.Selected == email {
		m.config.Selected = ""
	}

	m.Save()
	return nil
}
