package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// set an upload limit
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

	// extract video id from url
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error parsing videoID", err)
		return
	}

	// authentication
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

	// get video metadata from db
	video, err := cfg.db.GetVideo(videoID)
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "user is not owner of video", err)
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video from database", err)
		return
	}

	// parse video file from form data
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing form file", err)
		return
	}
	defer file.Close()

	// validate uploaded file
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid content-type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid media-type", err)
		return
	}

	// save uploaded file to a tmp on disk
	tmpFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating tmp file", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error copying to tmp file", err)
		return
	}

	// reset tmp file's pointer to the beginning
	tmpFile.Seek(0, io.SeekStart)

	// moov atom processing
	procPath, err := processVideoForFastStart(tmpFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error during process via ffmpeg", err)
		return
	}
	procFile, err := os.Open(procPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening processed file", err)
		return
	}

	defer os.Remove(procFile.Name())
	defer procFile.Close()

	// aspect ratio logic
	dir := ""
	ratio, err := getVideoAspectRatio(procFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}
	switch ratio {
	case "16:9":
		dir = "landscape"
	case "9:16":
		dir = "portrait"
	default:
		dir = "other"
	}

	// s3 file action
	key := makeS3VideoKey()
	key = filepath.Join(dir, key)
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        procFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error during S3 put", err)
		return
	}

	// update the video url
	// https://d2f12oh3itvuwy.cloudfront.net/portrait/5793afa9bd3fa515129aad0ee73e070e.mp4
	vidURLString := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, key)
	video.VideoURL = &vidURLString

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
