package storage

import (
	fs "github.com/dreitier/cloudmon/storage/fs"
)

// This method is no longer used.
// @deprecated
func getLatestFileInSlice(files []*fs.FileInfo) *fs.FileInfo{
	var latestFile *fs.FileInfo
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