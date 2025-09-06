package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
)

// HFResponse represents the HuggingFace API response for parquet file discovery
type HFResponse struct {
	Default map[string][]string `json:"default"`
}

// Metadata holds information about the downloaded dataset
type Metadata struct {
	SHA256       string    `json:"sha256"`
	DownloadedAt time.Time `json:"downloaded_at"`
	ETag         string    `json:"etag,omitempty"`
	Size         int64     `json:"size"`
}

// Manager handles dataset downloading and metadata management
type Manager struct {
	parquetURL   string
	parquetPath  string
	metadataPath string
	lockPath     string
	log          *slog.Logger
	config       *config.Config
}

// NewManager creates a new dataset manager
func NewManager(parquetURL, parquetPath, metadataPath, lockPath string, cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{
		parquetURL:   parquetURL,
		parquetPath:  parquetPath,
		metadataPath: metadataPath,
		lockPath:     lockPath,
		log:          logger,
		config:       cfg,
	}
}

// discoverDownloadURL discovers the actual download URL from HuggingFace API
func (m *Manager) discoverDownloadURL(ctx context.Context) (string, error) {
	// If the URL doesn't look like a HuggingFace dataset URL, use it directly
	if !strings.Contains(m.parquetURL, "huggingface.co/datasets/") {
		return m.parquetURL, nil
	}

	// For OpenFoodFacts, we know the structure - use the direct food.parquet file
	downloadURL := "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/food.parquet"

	m.log.Debug("Using direct download URL for OpenFoodFacts dataset", "download_url", downloadURL)
	return downloadURL, nil
} // EnsureDataset ensures the dataset is available and up-to-date
func (m *Manager) EnsureDataset(ctx context.Context) error {
	start := time.Now()
	m.log.Info("Ensuring dataset is available", "parquet_path", m.parquetPath)

	// Check if file exists
	if _, err := os.Stat(m.parquetPath); err == nil {
		// File exists, check if we should skip remote checks
		if m.config.DisableRemoteCheck {
			m.log.Info("Remote checks disabled, using local dataset", "duration", time.Since(start))
			return nil
		}

		// File exists, check if up-to-date
		upToDate, err := m.isUpToDate(ctx)
		if err != nil {
			m.log.Warn("Failed to verify dataset freshness", "error", err)
		}
		if upToDate {
			m.log.Info("Dataset is up-to-date", "duration", time.Since(start))
			return nil
		}
	}

	// Need to download
	if err := m.downloadWithLock(ctx); err != nil {
		return fmt.Errorf("failed to download dataset: %w", err)
	}

	m.log.Info("Dataset ensured", "duration", time.Since(start))
	return nil
}

// isUpToDate checks if the local dataset is up-to-date with the remote
func (m *Manager) isUpToDate(ctx context.Context) (bool, error) {
	start := time.Now()
	m.log.Debug("Checking if dataset is up-to-date")

	// Load local metadata
	localMeta, err := m.loadMetadata()
	if err != nil {
		m.log.Debug("No local metadata found", "error", err)
		return false, nil
	}

	// Get remote metadata
	remoteMeta, err := m.getRemoteMetadata(ctx)
	if err != nil {
		m.log.Warn("Failed to get remote metadata", "error", err)
		return false, err
	}

	// Compare ETag if available
	if remoteMeta.ETag != "" && localMeta.ETag != "" {
		upToDate := remoteMeta.ETag == localMeta.ETag
		m.log.Debug("ETag comparison", "local", localMeta.ETag, "remote", remoteMeta.ETag, "up_to_date", upToDate, "duration", time.Since(start))
		return upToDate, nil
	}

	// Fallback to size comparison
	upToDate := remoteMeta.Size == localMeta.Size
	m.log.Debug("Size comparison", "local", localMeta.Size, "remote", remoteMeta.Size, "up_to_date", upToDate, "duration", time.Since(start))
	return upToDate, nil
}

// getRemoteMetadata fetches metadata from the remote URL using HEAD request
func (m *Manager) getRemoteMetadata(ctx context.Context) (*Metadata, error) {
	start := time.Now()

	// Discover the actual download URL
	downloadURL, err := m.discoverDownloadURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover download URL: %w", err)
	}

	m.log.Debug("Fetching remote metadata", "url", downloadURL)

	req, err := http.NewRequestWithContext(ctx, "HEAD", downloadURL, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HEAD request failed with status: %d", resp.StatusCode)
	}

	meta := &Metadata{
		ETag: resp.Header.Get("ETag"),
		Size: resp.ContentLength,
	}

	m.log.Debug("Remote metadata fetched", "etag", meta.ETag, "size", meta.Size, "duration", time.Since(start))
	return meta, nil
}

// downloadWithLock downloads the dataset with file locking
func (m *Manager) downloadWithLock(ctx context.Context) error {
	start := time.Now()
	m.log.Info("Attempting to acquire download lock", "lock_path", m.lockPath)

	// Check if IGNORE_LOCK is enabled and forcefully remove lock file if it exists
	if m.config.IgnoreLock {
		if _, err := os.Stat(m.lockPath); err == nil {
			m.log.Warn("IGNORE_LOCK enabled, forcefully removing existing lock file", "lock_path", m.lockPath)
			if err := os.Remove(m.lockPath); err != nil {
				m.log.Warn("Failed to remove lock file", "error", err)
			}
		}
	}

	// Try to acquire lock
	lockFile, err := acquireLock(m.lockPath)
	if err != nil {
		if m.config.IgnoreLock {
			m.log.Warn("IGNORE_LOCK enabled but still failed to acquire lock, proceeding anyway", "error", err)
			// Continue with download without lock
		} else {
			m.log.Info("Another instance is downloading, waiting", "lock_path", m.lockPath)
			return m.waitForDownload(ctx)
		}
	}

	// Only defer lock release if we successfully acquired it
	if lockFile != nil {
		defer releaseLock(lockFile, m.lockPath)
	}

	m.log.Info("Lock acquired, starting download", "duration", time.Since(start))

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(m.parquetPath), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Download to temporary file in tmp-data directory to avoid volume constraints
	// Use tmp-data directory relative to current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	tmpDataDir := filepath.Join(cwd, "tmp-data")
	// Ensure tmp-data directory exists
	if err := os.MkdirAll(tmpDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp-data directory: %w", err)
	}
	tmpPath := filepath.Join(tmpDataDir, "product-database.parquet.tmp")
	if err := m.downloadFile(ctx, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Compute SHA256
	sha, err := computeSHA256(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to compute SHA256: %w", err)
	}

	// Get file size
	stat, err := os.Stat(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Get remote metadata for ETag
	remoteMeta, _ := m.getRemoteMetadata(ctx)
	etag := ""
	if remoteMeta != nil {
		etag = remoteMeta.ETag
	}

	// Save metadata
	meta := &Metadata{
		SHA256:       sha,
		DownloadedAt: time.Now().UTC(),
		ETag:         etag,
		Size:         stat.Size(),
	}
	if err := m.saveMetadata(meta); err != nil {
		m.log.Warn("Failed to save metadata", "error", err)
	}

	// Atomic file replacement: remove old file and copy new one
	// This minimizes the time window where the file doesn't exist
	if _, err := os.Stat(m.parquetPath); err == nil {
		// Remove existing file first
		if err := os.Remove(m.parquetPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to remove existing file: %w", err)
		}
	}

	// Copy from temp location to final location
	if err := m.copyFile(tmpPath, m.parquetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Clean up temporary file
	os.Remove(tmpPath)

	m.log.Info("Dataset downloaded successfully", "size", stat.Size(), "sha256", sha[:16]+"...", "duration", time.Since(start))
	return nil
}

// downloadFile downloads the file from the remote URL
func (m *Manager) downloadFile(ctx context.Context, filePath string) error {
	start := time.Now()

	// Discover the actual download URL
	downloadURL, err := m.discoverDownloadURL(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover download URL: %w", err)
	}

	m.log.Info("Downloading dataset", "url", downloadURL, "path", filePath)

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy with progress logging
	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	m.log.Info("Download completed", "bytes", written, "duration", time.Since(start))
	return nil
}

// waitForDownload waits for another instance to complete the download
func (m *Manager) waitForDownload(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for download by other instance")
		case <-ticker.C:
			if _, err := os.Stat(m.parquetPath); err == nil {
				m.log.Info("Dataset now available after other instance completed")
				return nil
			}
		}
	}
}

// loadMetadata loads metadata from the metadata file
func (m *Manager) loadMetadata() (*Metadata, error) {
	data, err := os.ReadFile(m.metadataPath)
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// saveMetadata saves metadata to the metadata file
func (m *Manager) saveMetadata(meta *Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.metadataPath, data, 0644)
}

// acquireLock attempts to acquire an exclusive lock
func acquireLock(lockPath string) (*os.File, error) {
	// Ensure the directory exists for the lock file
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// O_CREATE|O_EXCL will fail if file exists
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// releaseLock releases the lock file
func releaseLock(f *os.File, lockPath string) {
	f.Close()
	os.Remove(lockPath)
}

// copyFile copies a file from src to dst
func (m *Manager) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// computeSHA256 computes the SHA256 hash of a file
func computeSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
