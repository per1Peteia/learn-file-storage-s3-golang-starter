package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

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

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "missing content type for thumbnail", err)
		return
	}

	readData, err := io.ReadAll(data)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error reading file data", err)
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

	encString := base64.StdEncoding.EncodeToString(readData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, encString)

	video.ThumbnailURL = &dataURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
