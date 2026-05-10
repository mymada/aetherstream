package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportAndImport(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	configPath := filepath.Join(tmpDir, "config.yaml")
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// Create dummy files
	require.NoError(t, os.WriteFile(dbPath, []byte("dummy db content"), 0644))
	require.NoError(t, os.WriteFile(configPath, []byte("server:\n  port: 8096\n"), 0644))

	// Export
	backupPath, err := Export(dbPath, configPath, backupDir)
	require.NoError(t, err)
	assert.FileExists(t, backupPath)

	// Import to new paths
	newDB := filepath.Join(tmpDir, "restored.db")
	newCfg := filepath.Join(tmpDir, "restored.yaml")
	require.NoError(t, Import(backupPath, newDB, newCfg))

	// Verify
	dbBytes, err := os.ReadFile(newDB)
	require.NoError(t, err)
	assert.Equal(t, "dummy db content", string(dbBytes))

	cfgBytes, err := os.ReadFile(newCfg)
	require.NoError(t, err)
	assert.Contains(t, string(cfgBytes), "port: 8096")
}

func TestImport_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	badZip := filepath.Join(tmpDir, "bad.zip")
	f, err := os.Create(badZip)
	require.NoError(t, err)
	f.Close()

	err = Import(badZip, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open backup")
}

func TestExportJSON(t *testing.T) {
	cfg := map[string]interface{}{"port": 8096}
	data, err := ExportJSON(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(data), "8096")
}

func TestImportJSON_Valid(t *testing.T) {
	data := []byte(`{"port": 8096}`)
	var target map[string]interface{}
	err := ImportJSON(data, &target)
	require.NoError(t, err)
	assert.Equal(t, float64(8096), target["port"])
}

func TestImportJSON_Invalid(t *testing.T) {
	var target map[string]interface{}
	err := ImportJSON([]byte("not json"), &target)
	assert.Error(t, err)
}

func TestImportJSON_Empty(t *testing.T) {
	var target map[string]interface{}
	err := ImportJSON([]byte{}, &target)
	assert.Error(t, err)
}
