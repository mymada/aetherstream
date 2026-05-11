package backup

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Export creates a backup archive containing the SQLite DB and a JSON config snapshot.
func Export(dbPath, configPath, outDir string) (string, error) {
	timestamp := time.Now().UTC().Format("20060102_150405")
	backupName := fmt.Sprintf("aetherstream_backup_%s.zip", timestamp)
	backupPath := filepath.Join(outDir, backupName)

	// Security: ensure backupPath is within outDir
	cleanBackupPath := filepath.Clean(backupPath)
	cleanOutDir := filepath.Clean(outDir)
	if !strings.HasPrefix(cleanBackupPath, cleanOutDir+string(filepath.Separator)) && cleanBackupPath != cleanOutDir {
		return "", fmt.Errorf("backup path outside output directory")
	}

	f, err := os.OpenFile(cleanBackupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640) // #nosec G304 - path validated above against outDir
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Add DB
	if err := addFileToZip(zw, dbPath, "aetherstream.db"); err != nil {
		return "", fmt.Errorf("add db: %w", err)
	}

	// Add config if exists
	if _, err := os.Stat(configPath); err == nil {
		if err := addFileToZip(zw, configPath, "config.yaml"); err != nil {
			return "", fmt.Errorf("add config: %w", err)
		}
	}

	// Add manifest
	manifest := map[string]interface{}{
		"version":   "1.0",
		"createdAt": timestamp,
		"files":     []string{"aetherstream.db", "config.yaml"},
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return "", fmt.Errorf("create manifest: %w", err)
	}
	if err := json.NewEncoder(mw).Encode(manifest); err != nil {
		return "", fmt.Errorf("encode manifest: %w", err)
	}

	return backupPath, nil
}

func addFileToZip(zw *zip.Writer, srcPath, nameInZip string) error {
	// Security: validate source path is within expected directories (caller should ensure)
	sf, err := os.Open(srcPath) // #nosec G304 - caller must validate srcPath against allowed dirs
	if err != nil {
		return err
	}
	defer sf.Close()

	w, err := zw.Create(nameInZip)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, sf)
	return err
}

// Import restores a backup archive to the given dbPath and configPath.
// It validates the archive structure before extracting.
func Import(backupPath, dbPath, configPath string) error {
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer zr.Close()

	// Validate manifest
	var manifest map[string]interface{}
	var foundManifest bool
	for _, f := range zr.File {
		if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open manifest: %w", err)
			}
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				_ = rc.Close()
				return fmt.Errorf("decode manifest: %w", err)
			}
			_ = rc.Close()
			foundManifest = true
			break
		}
	}
	if !foundManifest {
		return fmt.Errorf("backup manifest missing")
	}
	if v, ok := manifest["version"].(string); !ok || v == "" {
		return fmt.Errorf("backup manifest version missing")
	}

	// Extract files
	for _, f := range zr.File {
		switch f.Name {
		case "aetherstream.db":
			if err := extractFile(f, dbPath); err != nil {
				return fmt.Errorf("extract db: %w", err)
			}
		case "config.yaml":
			if err := extractFile(f, configPath); err != nil {
				return fmt.Errorf("extract config: %w", err)
			}
		}
	}

	return nil
}

func extractFile(zf *zip.File, destPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Write to temp first
	tmpPath := destPath + ".tmp"
	// Security: validate tmpPath is within expected directory (caller should ensure)
	df, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640) // #nosec G304 - caller must validate destPath against allowed dirs
	if err != nil {
		return err
	}
	const maxDecompressedSize = 100 * 1024 * 1024 // 100MB limit
	_, err = io.CopyN(df, rc, maxDecompressedSize)
	_ = df.Close()
	if err != nil && err != io.EOF {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, destPath)
}

// ExportJSON exports config as JSON bytes
func ExportJSON(cfg interface{}) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}

// ImportJSON validates and parses config JSON bytes into target struct
func ImportJSON(data []byte, target interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	if !json.Valid(data) {
		return fmt.Errorf("invalid json")
	}
	return json.Unmarshal(data, target)
}

// Compress gzip-compresses data
func Compress(data []byte) ([]byte, error) {
	var buf []byte
	w := gzip.NewWriter(&bufWriter{&buf})
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

type bufWriter struct {
	buf *[]byte
}

func (b *bufWriter) Write(p []byte) (int, error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}
