package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/storage"
)

func GetDisks(w http.ResponseWriter) {
	disks := storage.GetDisks()

	writeData(w, disks)
}

func GetDirectories(
	w http.ResponseWriter,
	diskName string,
) {
	disk := findDisk(w, diskName)
	if disk == nil {
		return
	}

	writeData(w, disk.Definition)
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
		groupNotFound(w, variation)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")

	_, err = io.Copy(w, data)
	_ = data.Close()

	if err != nil {
		// headers and parts of the body have already been sent, so the status code can't be changed anymore
		log.Errorf("Failed to stream file %s to client: %v", fileName, err)
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

func findDisk(
	w http.ResponseWriter,
	diskName string,
) *storage.DiskData {
	disk := storage.FindDisk(diskName)
	if disk == nil {
		diskNotFound(w, diskName)
	}
	return disk
}

func findDirectory(
	w http.ResponseWriter,
	diskName string,
	directoryName string,
) *backup.Directory {
	disk := findDisk(w, diskName)
	if disk == nil {
		return nil
	}
	for _, dir := range disk.Definition.Directories {
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
