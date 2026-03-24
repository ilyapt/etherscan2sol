package solc

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const maxSolcSize = 100 * 1024 * 1024 // 100 MB

var versionRegex = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)

// ParseVersion extracts the semver portion from a solc compiler version string.
func ParseVersion(compilerVersion string) (string, error) {
	matches := versionRegex.FindStringSubmatch(compilerVersion)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse version from %q", compilerVersion)
	}
	return matches[1], nil
}

// EnsureCompiler returns the path to a solc binary for the given version,
// downloading it if necessary.
func EnsureCompiler(compilerVersion string) (string, error) {
	version, err := ParseVersion(compilerVersion)
	if err != nil {
		return "", fmt.Errorf("parsing compiler version: %w", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "."
	}
	solcDir := filepath.Join(cacheDir, "etherscan_to_sol", "solidity")
	solcPath := filepath.Join(solcDir, fmt.Sprintf("solc-v%s", version))

	info, statErr := os.Stat(solcPath)
	if statErr == nil && info.Mode()&0111 != 0 {
		return solcPath, nil
	}

	if err := os.MkdirAll(solcDir, 0755); err != nil {
		return "", fmt.Errorf("creating solidity directory: %w", err)
	}

	url := fmt.Sprintf("https://github.com/argotorg/solidity/releases/download/v%s/solc-macos", version)
	fmt.Fprintf(os.Stderr, "Downloading solc v%s from %s\n", version, url)

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading solc v%s: %w", version, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading solc v%s: HTTP %d", version, resp.StatusCode)
	}

	tmpPath := solcPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("creating solc binary: %w", err)
	}

	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxSolcSize)); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing solc binary: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("closing solc binary: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("setting solc permissions: %w", err)
	}

	if err := os.Rename(tmpPath, solcPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("installing solc binary: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Downloaded solc v%s to %s\n", version, solcPath)
	return solcPath, nil
}
