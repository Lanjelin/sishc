package testvars

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	loadedOnce sync.Once
	loadedVars map[string]string
)

func String(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	if value := fileVars()[key]; value != "" {
		return value
	}
	return fallback
}

func Has(key string) bool {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return true
	}
	_, ok := fileVars()[key]
	return ok
}

func Int(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	if value := fileVars()[key]; value != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return n
		}
	}
	return fallback
}

func Bool(key string, fallback bool) bool {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	if value := fileVars()[key]; value != "" {
		if b, err := strconv.ParseBool(strings.TrimSpace(value)); err == nil {
			return b
		}
	}
	return fallback
}

func fileVars() map[string]string {
	loadedOnce.Do(func() {
		loadedVars = map[string]string{}

		_, file, _, ok := runtime.Caller(0)
		if !ok {
			return
		}

		root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
		path := filepath.Join(root, ".test-secrets")
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "export ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if len(value) >= 2 {
				if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
					(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
					value = value[1 : len(value)-1]
				}
			}
			if key != "" {
				loadedVars[key] = value
			}
		}
	})
	return loadedVars
}
