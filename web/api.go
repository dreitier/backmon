package web

import (
	"github.com/dreitier/cloudmon/backup"
	"github.com/dreitier/cloudmon/storage"
	"encoding/json"
	"io"
	"net/http"
)

func GetBuckets(w http.ResponseWriter) {
	buckets := storage.GetBuckets()

	writeData(w, buckets)
}

func GetDirectories(
	w http.ResponseWriter,
	bucketName string,
) {
	bucket := findBucket(w, bucketName)
	if bucket == nil {
		return
	}

	writeData(w, bucket.Definition)
}

func GetFiles(
	w http.ResponseWriter,
	bucketName string,
	directoryName string,
) {
	dir := findDirectory(w, bucketName, directoryName)
	if dir == nil {
		return
	}

	writeData(w, dir.Files)
}

func GetVariations(
	w http.ResponseWriter,
	bucketName string,
	directoryName string,
	fileName string,
) {
	filenames := storage.GetFilenames(bucketName, directoryName, fileName)
	if filenames == nil {
		fileNotFound(w, fileName)
		return
	}

	writeData(w, filenames)
}

func Download(
	w http.ResponseWriter,
	bucketName string,
	directoryName string,
	fileName string,
	variation string,
) {
	data, err := storage.Download(bucketName, directoryName, fileName, variation)
	if err != nil {
		groupNotFound(w, variation)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachmernt; filename=\""+ fileName + "\"")
	_, err = io.Copy(w, data)
	if err != nil && err != io.EOF{
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_ = data.Close()
}

func writeData(w http.ResponseWriter, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errStr := err.Error()
		_, _ = w.Write([]byte(errStr))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(b)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func findBucket(
	w http.ResponseWriter,
	bucketName string,
) *storage.BucketData {
	bucket := storage.FindBucket(bucketName)
	if bucket == nil {
		bucketNotFound(w, bucketName)
	}
	return bucket
}

func findDirectory(
	w http.ResponseWriter,
	bucketName string,
	directoryName string,
) *backup.Directory {
	bucket := findBucket(w, bucketName)
	if bucket == nil {
		return nil
	}
	for _, dir := range bucket.Definition {
		if dir.Alias == directoryName {
			return dir
		}
	}
	directoryNotFound(w, directoryName)
	return nil
}

func findFile(
	w http.ResponseWriter,
	bucketName string,
	directoryName string,
	fileName string,
) *backup.File {
	dir := findDirectory(w, bucketName, directoryName)
	if dir == nil {
		return nil
	}
	for _, file := range dir.Files {
		if file.Alias == fileName {
			return file
		}
	}
	fileNotFound(w, fileName)
	return nil
}
