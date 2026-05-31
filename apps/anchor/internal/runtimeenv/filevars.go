package runtimeenv

import (
	"fmt"
	"os"
	"strings"
)

const fileSuffix = "_FILE"

// HydrateFileBackedEnv loads VAR contents from VAR_FILE when VAR is not already set.
func HydrateFileBackedEnv() error {
	for _, envEntry := range os.Environ() {
		name, filePath, ok := strings.Cut(envEntry, "=")
		if !ok || !strings.HasSuffix(name, fileSuffix) || strings.TrimSpace(filePath) == "" {
			continue
		}

		targetName := strings.TrimSuffix(name, fileSuffix)
		if _, exists := os.LookupEnv(targetName); exists {
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s from %s: %w", name, filePath, err)
		}

		setErr := os.Setenv(targetName, strings.TrimRight(string(data), "\r\n"))
		if setErr != nil {
			return fmt.Errorf("set %s from %s: %w", targetName, name, setErr)
		}
	}

	return nil
}
