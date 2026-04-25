package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func loadDotEnvFiles() error {
	paths := []string{".env"}

	if executablePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executablePath), ".env"))
	}

	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, ok := seen[absolutePath]; ok {
			continue
		}
		seen[absolutePath] = struct{}{}

		if err := loadDotEnvFile(absolutePath); err != nil {
			return err
		}
	}

	return nil
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++

		key, value, ok, err := parseDotEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if !ok {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func parseDotEnvLine(line string) (string, string, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}

	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false, fmt.Errorf("invalid dotenv line")
	}

	key := strings.TrimSpace(parts[0])
	if !isValidEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid env key %q", key)
	}

	value, err := parseDotEnvValue(parts[1])
	if err != nil {
		return "", "", false, err
	}

	return key, value, true, nil
}

func parseDotEnvValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if strings.HasPrefix(value, `"`) {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value: %w", err)
		}
		return parsed, nil
	}

	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'"), nil
	}

	return stripDotEnvInlineComment(value), nil
}

func stripDotEnvInlineComment(value string) string {
	for index, char := range value {
		if char == '#' && index > 0 && unicode.IsSpace(rune(value[index-1])) {
			return strings.TrimSpace(value[:index])
		}
	}

	return strings.TrimSpace(value)
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}

	for index, char := range key {
		if char == '_' || unicode.IsLetter(char) || (index > 0 && unicode.IsDigit(char)) {
			continue
		}
		return false
	}

	return true
}
