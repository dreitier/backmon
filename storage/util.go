package storage

func getLatestFileInSlice(files []*File) *File{
	var latestFile *File
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