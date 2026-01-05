package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	CurrentVersion = "1.0.0"
	GitHubRepo     = "HarshalPatel1972/momentum"
	UpdateCheckURL = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
)

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Assets     []Asset `json:"assets"`
	HTMLURL    string `json:"html_url"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdates queries GitHub for the latest release
func (a *App) CheckForUpdates() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	resp, err := client.Get(UpdateCheckURL)
	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %v", err)
	}

	// Remove 'v' prefix if present
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	
	if latestVersion != CurrentVersion {
		return latestVersion, nil
	}

	return "", nil // No update available
}

// DownloadUpdate downloads the new version
func (a *App) DownloadUpdate(version string) error {
	// Get latest release info
	client := &http.Client{Timeout: 30 * time.Second}
	
	resp, err := client.Get(UpdateCheckURL)
	if err != nil {
		return fmt.Errorf("failed to fetch update info: %v", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release: %v", err)
	}

	// Find the .exe asset
	var downloadURL string
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".exe") && !strings.Contains(asset.Name, "Setup") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no executable found in release")
	}

	// Download to temp location
	tempPath := filepath.Join(os.TempDir(), "Momentum-update.exe")
	
	resp, err = client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save update: %v", err)
	}

	// Apply update (replace current exe)
	if err := a.applyUpdate(tempPath); err != nil {
		return fmt.Errorf("failed to apply update: %v", err)
	}

	return nil
}

// applyUpdate replaces the current executable with the new one
func (a *App) applyUpdate(newExePath string) error {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// On Windows, we need to rename old exe and copy new one
	oldPath := exePath + ".old"
	
	// Rename current exe
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("failed to rename old executable: %v", err)
	}

	// Copy new exe to current location
	if err := copyFile(newExePath, exePath); err != nil {
		// Restore old exe if copy failed
		os.Rename(oldPath, exePath)
		return fmt.Errorf("failed to copy new executable: %v", err)
	}

	// Schedule old file deletion and restart
	if err := a.scheduleRestartAndCleanup(oldPath); err != nil {
		return err
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// scheduleRestartAndCleanup creates a batch script to clean up and restart
func (a *App) scheduleRestartAndCleanup(oldExePath string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-update only supported on Windows")
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// Create batch script
	batchContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak > nul
del "%s" > nul 2>&1
start "" "%s"
del "%%~f0"
`, oldExePath, exePath)

	batchPath := filepath.Join(os.TempDir(), "momentum-update.bat")
	if err := os.WriteFile(batchPath, []byte(batchContent), 0755); err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}

	// Launch batch script
	cmd := exec.Command("cmd", "/c", "start", "/min", batchPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch update script: %v", err)
	}

	// App will be closed by the batch script
	os.Exit(0)
	
	return nil
}

// AutoCheckForUpdates runs in background on startup
func (a *App) AutoCheckForUpdates() {
	go func() {
		time.Sleep(3 * time.Second) // Wait a bit before checking

		latestVersion, err := a.CheckForUpdates()
		if err != nil {
			// Silent fail - don't bother user with update check errors
			return
		}

		if latestVersion != "" {
			// Show notification in system tray
			a.showUpdateNotification(latestVersion)
		}
	}()
}

// showUpdateNotification shows a tray notification about available update
func (a *App) showUpdateNotification(version string) {
	// Notification will be handled by system tray menu check
	// User can manually check for updates via tray menu
}
