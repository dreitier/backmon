package storage

import "time"

type File struct {
	Name      *string
	Timestamp *time.Time
	Size      *int64
}