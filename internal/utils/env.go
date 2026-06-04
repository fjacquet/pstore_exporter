package utils

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fjacquet/pstore_exporter/internal/models"
)

var envRefPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandEnv replaces ${VAR} references with the value of the environment variable VAR.
// It returns an error if a referenced variable is not set, so misconfiguration fails
// loudly at startup rather than silently authenticating with an empty secret.
func ExpandEnv(s string) (string, error) {
	var missing []string
	out := envRefPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := envRefPattern.FindStringSubmatch(match)[1]
		val, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return ""
		}
		return val
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable(s) referenced in config but not set: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// ResolveSecrets expands ${ENV} references in array endpoint/password fields and
// loads passwords from passwordFile when set. Mutates cfg in place.
func ResolveSecrets(cfg *models.Config) error {
	for i := range cfg.Arrays {
		a := &cfg.Arrays[i]

		endpoint, err := ExpandEnv(a.Endpoint)
		if err != nil {
			return fmt.Errorf("array %q endpoint: %w", a.Name, err)
		}
		a.Endpoint = endpoint

		if a.Password == "" && a.PasswordFile != "" {
			data, err := os.ReadFile(a.PasswordFile)
			if err != nil {
				return fmt.Errorf("array %q passwordFile: %w", a.Name, err)
			}
			a.Password = strings.TrimSpace(string(data))
			continue
		}

		password, err := ExpandEnv(a.Password)
		if err != nil {
			return fmt.Errorf("array %q password: %w", a.Name, err)
		}
		a.Password = password
	}
	return nil
}
