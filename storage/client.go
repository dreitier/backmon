package storage

import (
	"github.com/dreitier/backmon/config"
	fs "github.com/dreitier/backmon/storage/fs"
	provider "github.com/dreitier/backmon/storage/provider"
	"io"
)

type Client interface {
	GetDiskNames() ([]string, error)
	GetFileNames(disk string, maxDepth uint) (*fs.DirectoryInfo, error)
	Download(disk string, file *fs.FileInfo) (bytes io.ReadCloser, err error)
	Delete(disk string, file *fs.FileInfo) error
}

func NewClient(config *config.ClientConfiguration) Client {
	if config.Directory == "" {
		return &provider.S3Client{
			EnvName:           config.EnvName,
			Region:            config.Region,
			AccessKey:         config.AccessKey,
			SecretKey:         config.SecretKey,
			Endpoint:          config.Endpoint,
			ForcePathStyle:    config.ForcePathStyle,
			Token:             config.Token,
			AutoDiscoverDisks: config.AutoDiscoverDisks,
		}
	}

	// fall back to local client
	return &provider.LocalClient{
		EnvName:   config.EnvName,
		Directory: config.Directory,
	}
}
