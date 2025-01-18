package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// upload logic
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)

	data, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing form file", err)
		return
	}
	defer data.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid content-type", err)
		return
	}
	if mediaType != "image/jpg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "invalid media-type", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user is not owner of video", err)
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video from database", err)
		return
	}

	// file saving
	ext, _ := strings.CutPrefix(mediaType, "image/")
	fileExt := videoIDString + "." + ext
	path := filepath.Join(cfg.assetsRoot, fileExt)
	dst, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating file", err)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, data)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying file", err)
		return
	}

	tnURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileExt)
	video.ThumbnailURL = &tnURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
