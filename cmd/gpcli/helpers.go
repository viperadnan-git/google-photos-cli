package main

import (
	"fmt"
	"strings"
)

var configPath string
var authOverride string
var cfgManager *ConfigManager

func loadConfig() error {
	var err error
	cfgManager, err = NewConfigManager(configPath)
	return err
}

// getAuthData returns the auth data string based on authOverride or selected config
func getAuthData(cfg Config) string {
	if authOverride != "" {
		return authOverride
	}
	// Find credentials for selected account
	for _, cred := range cfg.Credentials {
		params, err := ParseAuthString(cred)
		if err != nil {
			continue
		}
		if params.Get("Email") == cfg.Selected {
			return cred
		}
	}
	// Return first credential if no match
	if len(cfg.Credentials) > 0 {
		return cfg.Credentials[0]
	}
	return ""
}

// resolveEmailFromArg resolves an email from either an index number (1-based) or email string
func resolveEmailFromArg(arg string, credentials []string) (string, error) {
	// Try to parse as number first
	if num, err := fmt.Sscanf(arg, "%d", new(int)); err == nil && num == 1 {
		var idx int
		fmt.Sscanf(arg, "%d", &idx)
		if idx < 1 || idx > len(credentials) {
			return "", fmt.Errorf("invalid index %d: must be between 1 and %d", idx, len(credentials))
		}
		params, err := ParseAuthString(credentials[idx-1])
		if err != nil {
			return "", fmt.Errorf("invalid credential at index %d", idx)
		}
		return params.Get("Email"), nil
	}

	// Otherwise treat as email - try exact match first
	for _, cred := range credentials {
		params, err := ParseAuthString(cred)
		if err != nil {
			continue
		}
		email := params.Get("Email")
		if email == arg {
			return email, nil
		}
	}

	// Try fuzzy matching
	var candidates []string
	for _, cred := range credentials {
		params, err := ParseAuthString(cred)
		if err != nil {
			continue
		}
		email := params.Get("Email")
		if containsSubstring(email, arg) {
			candidates = append(candidates, email)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no authentication found matching '%s'", arg)
	} else if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", fmt.Errorf("multiple accounts match '%s': %v - please be more specific", arg, candidates)
}

func containsSubstring(str, substr string) bool {
	strLower := strings.ToLower(str)
	substrLower := strings.ToLower(substr)
	return strings.Contains(strLower, substrLower)
}
