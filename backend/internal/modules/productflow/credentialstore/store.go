package credentialstore

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Store resolves a secret only inside a provider adapter. Implementations can be
// replaced by KMS or a cloud secret manager without changing provider configs.
type Store interface {
	Inspect(context.Context, string) (Metadata, error)
	Resolve(context.Context, string) ([]byte, error)
}

type Metadata struct {
	Configured bool   `json:"configured"`
	Hint       string `json:"hint,omitempty"`
}

type EnvironmentStore struct{}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`)

func envReference(reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if !strings.HasPrefix(reference, "env:") {
		return "", fmt.Errorf("unsupported credential reference")
	}
	name := strings.TrimPrefix(reference, "env:")
	if !envName.MatchString(name) {
		return "", fmt.Errorf("invalid environment credential reference")
	}
	return name, nil
}

func (EnvironmentStore) Inspect(_ context.Context, reference string) (Metadata, error) {
	if strings.TrimSpace(reference) == "" {
		return Metadata{}, nil
	}
	name, err := envReference(reference)
	if err != nil {
		return Metadata{}, err
	}
	value, ok := os.LookupEnv(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return Metadata{}, nil
	}
	hint := "******"
	runes := []rune(value)
	if len(runes) >= 4 {
		hint += string(runes[len(runes)-4:])
	}
	return Metadata{Configured: true, Hint: hint}, nil
}

func (EnvironmentStore) Resolve(_ context.Context, reference string) ([]byte, error) {
	name, err := envReference(reference)
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil, fmt.Errorf("credential is not configured")
	}
	return []byte(value), nil
}
