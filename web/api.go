package web

import (
	"encoding/json"
	"github.com/dreitier/backmon/backup"
	"github.com/dreitier/backmon/storage"
	"io"
	"net/http"
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
	data, err := storage.Download(diskName, directoryName, fileName, variation)
	if err != nil {
		groupNotFound(w, variation)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachmernt; filename=\""+fileName+"\"")
	_, err = io.Copy(w, data)
	if err != nil && err != io.EOF {
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
