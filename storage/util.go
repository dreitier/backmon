package storage

import (
	fs "github.com/dreitier/backmon/storage/fs"
)

// This method is no longer used.
// @deprecated
func getLatestFileInSlice(files []*fs.FileInfo) *fs.FileInfo {
	var latestFile *fs.FileInfo
	for _, file := range files {
		if latestFile == nil {
			latestFile = file
		} else {
			if latestFile.ModifiedAt.Before(file.ModifiedAt) {
				latestFile = file
			}
		}
	}

	return latestFile
}
