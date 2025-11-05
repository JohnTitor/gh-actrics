package util

import (
	"fmt"
	"strings"
)

// ParseRepo converts OWNER/REPO string into components.
func ParseRepo(input string) (owner string, repo string, err error) {
	parts := strings.Split(strings.TrimSpace(input), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo must be in OWNER/REPO format")
	}
	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("repo must be in OWNER/REPO format")
	}
	return owner, repo, nil
}
