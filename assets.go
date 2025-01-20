package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	id := make([]byte, 32)
	_, err := rand.Read(id)
	if err != nil {
		panic("failed to generate random bytes")
	}
	videoIDStr := base64.StdEncoding.EncodeToString(id)

	ext := mediaTypeToExt(mediaType)
	path := videoIDStr + "." + ext

	return path
}

func (cfg *apiConfig) getAssetDiskPath(assetPath string) string {
	diskPath := filepath.Join(cfg.assetsRoot, assetPath)

	return diskPath
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func makeS3VideoKey() string {
	// Create a byte slice to hold our random bytes
	bytes := make([]byte, 16) // 16 bytes will give us 32 hex chars

	// Read random bytes
	_, err := rand.Read(bytes)
	if err != nil {
		panic("failed to generate random bytes")
	}

	// Convert to hex string
	return fmt.Sprintf("%x.mp4", bytes)
}
