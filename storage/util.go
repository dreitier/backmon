package storage

import (
	storage "github.com/dreitier/cloudmon/storage/abstraction"
)

func getLatestFileInSlice(files []*storage.File) *storage.File{
	var latestFile *storage.File
	for _, file := range files {
		if latestFile == nil {
			latestFile = file
		} else {
			if latestFile.Timestamp.Before(*file.Timestamp) {
				latestFile = file
			}
		}
	}

	return latestFile
}