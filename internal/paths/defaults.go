package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appDirName    = "lazyssm"
	configName    = "config.yaml"
	logFileName   = "lazyssm.log"
	processLogDir = "process"
)

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
			return filepath.Join(configDir, appDirName, configName)
		}
		return filepath.Join("~", "AppData", "Roaming", appDirName, configName)
	}

	return filepath.Join("~", ".config", appDirName, configName)
}

func DefaultLogFilePath() string {
	return filepath.Join(defaultStateDir(), logFileName)
}

func DefaultProcessLogDir() string {
	return filepath.Join(defaultStateDir(), processLogDir)
}

func ExpandHome(path string) (string, error) {
	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, path[2:]), nil
	}

	return filepath.Clean(path), nil
}

func defaultStateDir() string {
	if runtime.GOOS == "windows" {
		if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
			return filepath.Join(cacheDir, appDirName)
		}
		if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
			return filepath.Join(configDir, appDirName)
		}
		return filepath.Join("~", "AppData", "Local", appDirName)
	}

	return filepath.Join("~", ".local", "state", appDirName)
}
