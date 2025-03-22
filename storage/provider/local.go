package provider

import (
	"errors"
	"fmt"
	fs "github.com/dreitier/backmon/storage/fs"
	dotstat "github.com/dreitier/backmon/storage/fs/dotstat"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type LocalClient struct {
	Directory string
	EnvName   string
}

func (c *LocalClient) GetFileNames(diskName string, maxDepth uint64) (*fs.DirectoryInfo, error) {
	if diskName != c.Directory {
		return nil, errors.New(fmt.Sprintf("disk %#q does not exist", diskName))
	}

	return scanDir(diskName, "", "", maxDepth)
}

func scanDir(root string, fullSubdirectoryPath string, directoryName string, maxDepth uint64) (*fs.DirectoryInfo, error) {
	currentSubdirectoryPath := filepath.Join(fullSubdirectoryPath, directoryName)
	absoluteSubdirectoryPath := filepath.Join(root, currentSubdirectoryPath)
	dirEntries, err := os.ReadDir(absoluteSubdirectoryPath)

	if err != nil {
		log.Errorf("Failed to scan directory %s, %v", absoluteSubdirectoryPath, err)
		return nil, err
	}

	directoryContainer := &fs.DirectoryInfo{
		Name:    directoryName,
		SubDirs: make(map[string]*fs.DirectoryInfo),
	}

	dotStatFiles := make(map[string]string)

	for _, dirEntry := range dirEntries {
		// if current item is a directory, scanning it recursively
		if dirEntry.IsDir() {
			if maxDepth < 1 {
				continue
			}

			subDir, subErr := scanDir(root, currentSubdirectoryPath, dirEntry.Name(), maxDepth-1)

			if subErr == nil {
				directoryContainer.SubDirs[subDir.Name] = subDir
			}
		} else if dotstat.IsStatFile(dirEntry.Name()) {
			pathToStatFile := absoluteSubdirectoryPath + "/" + dirEntry.Name()
			pathToNonStatFile := dotstat.RemoveDotStatSuffix(pathToStatFile)
			// .stat files are registered for later examination
			dotStatFiles[pathToNonStatFile] = pathToStatFile
			log.Debugf("Adding .stat file %s for %s", pathToStatFile, pathToNonStatFile)
		} else {
			fileInfo, err := dirEntry.Info()

			if err != nil {
				log.Errorf("Failed to get file info for %s, %v", dirEntry.Name(), err)
				continue
			}

			file := &fs.FileInfo{
				Name:       dirEntry.Name(),
				Parent:     absoluteSubdirectoryPath,
				BornAt:     fileInfo.ModTime(),
				ModifiedAt: fileInfo.ModTime(),
				ArchivedAt: fileInfo.ModTime(),
				Size:       fileInfo.Size(),
			}

			directoryContainer.Files = append(directoryContainer.Files, file)
		}
	}

	dotstat.ApplyDotStatValues(dotStatFiles, directoryContainer.Files)

	return directoryContainer, nil
}

func (c *LocalClient) GetDiskNames() ([]string, error) {
	diskName := c.Directory
	diskNames := make([]string, 1, 1)
	diskNames[0] = diskName

	return diskNames, nil
}

func (c *LocalClient) Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, err error) {
	if disk != c.Directory {
		return nil, errors.New(fmt.Sprintf("disk %#q does not exist", disk))
	}
	fileName := filepath.Join(disk, file.Parent, file.Name)

	bytes, err = os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for reading: %s", err)
	}

	return bytes, nil
}

func (c *LocalClient) Delete(disk string, file *fs.FileInfo) error {
	if disk != c.Directory {
		return fmt.Errorf("disk %#q does not exist", disk)
	}
	filePath := filepath.Join(disk, file.Parent, file.Name)

	err := os.Remove(filePath)

	// remove a belonging .stat file if it is existent
	possibleDotStatFilePath := dotstat.ToDotStatPath(filePath)
	dotStatExists, _ := fs.IsFilePathValid(possibleDotStatFilePath)

	if dotStatExists {
		// don't throw any errors
		_ = os.Remove(possibleDotStatFilePath)
	}

	return err
}

func (c *LocalClient) findDisk(diskName *string) (*string, error) {
	names, err := c.GetDiskNames()

	if err != nil {
		return nil, fmt.Errorf("could not get available diskName names: %s", err)
	}

	if runtime.GOOS != "windows" && !strings.HasPrefix(*diskName, string(os.PathSeparator)) {
		*diskName = string(os.PathSeparator) + *diskName
	}

	diskFound := false

	for _, name := range names {
		if name == *diskName {
			diskFound = true
		}
	}

	if !diskFound {
		return nil, fmt.Errorf("unknown diskName %#q", *diskName)
	}

	return diskName, nil
}

func (c *LocalClient) getDiskName() string {
	trimmedString := strings.TrimLeft(c.Directory, "/")
	normalizedDiskName := strings.Replace(trimmedString, "/", "_", -1)
	return normalizedDiskName
}
