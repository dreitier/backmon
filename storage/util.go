package storage

import (
	storage "github.com/dreitier/cloudmon/storage/abstraction"
)

// This method is no longer used.
// @deprecated
func getLatestFileInSlice(files []*storage.FileInfo) *storage.FileInfo{
	var latestFile *storage.FileInfo
	for _, file := range files {
		if latestFile == nil {
			latestFile = file
		} else {
			if latestFile.Timestamp.Before(file.Timestamp) {
				latestFile = file
			}
		}
	}

	return latestFile
}