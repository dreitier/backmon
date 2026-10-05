package web

import (
	"encoding/json"
	"fmt"
	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/storage"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func GetDisks(w http.ResponseWriter) {
	disks := storage.GetDisks()

	writeData(w, disks)
}

func GetDirectories(
	w http.ResponseWriter,
	diskName string,
) {
	definition := findDefinition(w, diskName)
	if definition == nil {
		return
	}

	writeData(w, definition)
}

func GetFiles(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
) {
	dir := findDirectory(w, diskName, directoryName)
	if dir == nil {
		return
	}

	writeData(w, dir.Files)
}

func GetVariations(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
	fileName string,
) {
	filenames := storage.GetFilenames(diskName, directoryName, fileName)
	if filenames == nil {
		fileNotFound(w, fileName)
		return
	}

	writeData(w, filenames)
}

func Download(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
	fileName string,
	variation string,
) {
	data, length, contentType, err := storage.Download(diskName, directoryName, fileName, variation)
	if err != nil {
		log.Debugf("Download of %s/%s/%s/%s failed: %s", diskName, directoryName, fileName, variation, err)
		groupNotFound(w, variation)
		return
	}

	defer func() {
		_ = data.Close()
	}()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")

	// headers have already been sent at this point, so an error can only be logged
	if _, err = io.Copy(w, data); err != nil {
		log.Errorf("Failed to stream %s/%s/%s/%s to client: %s", diskName, directoryName, fileName, variation, err)
	}
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

// findDefinition returns the backup definitions of a disk or writes a 404 response
func findDefinition(
	w http.ResponseWriter,
	diskName string,
) *backup.Definition {
	definition, found := storage.GetDefinition(diskName)
	if !found {
		diskNotFound(w, diskName)
		return nil
	}
	if definition == nil {
		definitionsNotFound(w, diskName)
	}
	return definition
}

func findDirectory(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
) *backup.Directory {
	definition := findDefinition(w, diskName)
	if definition == nil {
		return nil
	}
	for _, dir := range definition.Directories {
		if dir.Alias == directoryName {
			return dir
		}
	}
	directoryNotFound(w, directoryName)
	return nil
}

func findFile(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
	fileName string,
) *backup.FileDefinition {
	dir := findDirectory(w, diskName, directoryName)
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
