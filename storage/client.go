package storage

import (
	"github.com/dreitier/cloudmon/config"
	storage "github.com/dreitier/cloudmon/storage/abstraction"
	provider "github.com/dreitier/cloudmon/storage/provider"
	"io"
)

type Client interface {
	GetDiskNames() ([]string, error)
	GetFileNames(disk string, maxDepth uint) (*storage.DirectoryInfo, error)
	Download(disk string, file *storage.FileInfo) (bytes io.ReadCloser, err error)
	Delete(disk string, file *storage.FileInfo) error
}

func NewClient(config *config.ClientConfiguration) Client {
	if config.Directory == "" {
		return &provider.S3Client{
			EnvName:        config.EnvName,
			Region:         config.Region,
			AccessKey:      config.AccessKey,
			SecretKey:      config.SecretKey,
			Endpoint:       config.Endpoint,
			ForcePathStyle: config.ForcePathStyle,
			Token:          config.Token,
		}
	} else {
		return &provider.LocalClient{
			EnvName:   config.EnvName,
			Directory: config.Directory,
		}
	}
}
