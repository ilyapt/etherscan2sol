package solc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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

type solcBuildList struct {
	Builds []solcBuild `json:"builds"`
}

type solcBuild struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

func platformSlug() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "macosx-amd64", nil // works on arm64 via Rosetta
	case "linux":
		return "linux-amd64", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// findBuildPath fetches the solc-bin list.json and returns the binary path for the given version.
func findBuildPath(httpClient *http.Client, platform, version string) (string, error) {
	listURL := fmt.Sprintf("https://binaries.soliditylang.org/%s/list.json", platform)

	resp, err := httpClient.Get(listURL)
	if err != nil {
		return "", fmt.Errorf("fetching solc build list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching solc build list: HTTP %d", resp.StatusCode)
	}

	var list solcBuildList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", fmt.Errorf("parsing solc build list: %w", err)
	}

	for i := len(list.Builds) - 1; i >= 0; i-- {
		if list.Builds[i].Version == version {
			return list.Builds[i].Path, nil
		}
	}

	return "", fmt.Errorf("solc v%s not found in %s builds", version, platform)
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

	platform, err := platformSlug()
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{Timeout: 5 * time.Minute}

	buildPath, err := findBuildPath(httpClient, platform, version)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://binaries.soliditylang.org/%s/%s", platform, buildPath)
	fmt.Fprintf(os.Stderr, "Downloading solc v%s from %s\n", version, url)

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
