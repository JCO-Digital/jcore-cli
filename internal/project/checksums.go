package project

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/JCO-Digital/jcore/internal/constants"
)

// LoadChecksums loads the checksums from the project's checksum file
func LoadChecksums(projectDir string) (map[string]string, error) {
	checksumPath := filepath.Join(projectDir, constants.ChecksumFile)
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var checksums map[string]string
	if err := json.Unmarshal(data, &checksums); err != nil {
		return nil, err
	}
	return checksums, nil
}

// SaveChecksums saves the checksums to the project's checksum file
func SaveChecksums(projectDir string, checksums map[string]string) error {
	checksumPath := filepath.Join(projectDir, constants.ChecksumFile)
	data, err := json.MarshalIndent(checksums, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(checksumPath, data, 0644)
}

// CalculateChecksum calculates the SHA256 checksum of a file
func CalculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CompareChecksum compares a file's current checksum against its stored checksum.
// If strict is false and no checksum is stored for the file, it is treated as unchanged.
func CompareChecksum(projectDir string, relPath string, strict bool) (bool, error) {
	checksums, err := LoadChecksums(projectDir)
	if err != nil {
		return false, err
	}

	stored, exists := checksums[relPath]
	if !strict && !exists {
		return true, nil
	}

	current, err := CalculateChecksum(filepath.Join(projectDir, relPath))
	if err != nil {
		return false, err
	}

	return current == stored, nil
}

// UpdateChecksum updates the checksum for a specific file in the project
func UpdateChecksum(projectDir string, relPath string) error {
	checksums, err := LoadChecksums(projectDir)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(projectDir, relPath)
	checksum, err := CalculateChecksum(fullPath)
	if err != nil {
		return err
	}

	checksums[relPath] = checksum
	return SaveChecksums(projectDir, checksums)
}
