package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// ffprobe -v error -print_format json -show_streams https://tubely-881484.s3.eu-north-1.amazonaws.com/0cd9f1a37ff8d0c72687656c33872cfe.mp4

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Error running ffprobe: %w", err)
	}

	var output struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	err = json.Unmarshal(buf.Bytes(), &output)
	if err != nil {
		return "", fmt.Errorf("Error unmarshaling ffprobe output: %w", err)
	}

	if len(output.Streams) == 0 {
		return "", errors.New("no video streams found")
	}

	width := output.Streams[0].Width
	height := output.Streams[0].Height

	if width == 16*height/9 {
		return "16:9", nil
	} else if height == 16*width/9 {
		return "9:16", nil
	}
	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outPath := filePath + ".processing"

	cmd := exec.Command(
		"ffmpeg", "-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error processing video: %s, %v", stderr.String(), err)
	}

	return outPath, nil
}
