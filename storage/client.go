package storage

import (
	"github.com/dreitier/cloudmon/config"
	"io"
)

type Client interface {
	GetDiskNames() ([]string, error)
	GetFileNames(disk string, maxDepth uint) (*DirectoryInfo, error)
	Download(disk string, file *FileInfo) (bytes io.ReadCloser, err error)
	Delete(disk string, file *FileInfo) error
}

func NewClient(config *config.Client) Client {
	if config.Directory == "" {
		return &S3Client{
			EnvName:        config.EnvName,
			Region:         config.Region,
			AccessKey:      config.AccessKey,
			SecretKey:      config.SecretKey,
			Endpoint:       config.Endpoint,
			ForcePathStyle: config.ForcePathStyle,
			Token:          config.Token,
		}
	} else {
		return &LocalClient{
			EnvName:   config.EnvName,
			Directory: config.Directory,
		}
	}
}
