package backup

import (
	"github.com/dreitier/cloudmon/config"
	"fmt"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
	"io"
	"time"
)

type RawDefinition map[string]*RawDirectory

type RawDirectory struct {
	Alias    string
	FuseVars []string
	Defaults *Defaults
	Files    map[string]*RawFile
}

type Defaults struct {
	Schedule       *cronexpr.Expression
	Sort           string
	RetentionCount uint64
	RetentionAge   time.Duration
	Purge          bool
}

type RawFile struct {
	Alias          string
	Schedule       *cronexpr.Expression
	Sort           string
	RetentionCount uint64
	RetentionAge   time.Duration
	Purge          bool
}

func ParseRawDefinitions(definitionsReader io.Reader) (RawDefinition, error) {
	cfg, err := config.Parse(definitionsReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse definitions file: %v", err)
	}

	parsed := make(RawDefinition)
	for dirName := range cfg {
		dirConfig := cfg.Sub(dirName)
		dir, err := parseRawDirectory(dirConfig, dirName)
		if err != nil {
			return nil, err
		}
		parsed[dirName] = dir
	}

	return parsed, nil
}

func parseRawDirectory(cfg config.Raw, name string) (*RawDirectory, error) {
	defaults, err := parseDefaults(cfg.Sub("defaults"))
	if err != nil {
		return nil, err
	}

	files := make(map[string]*RawFile)
	fileConfigs := cfg.Sub("files")
	if fileConfigs == nil {
		log.Warnf("directory %s does not contain any files", name)
	} else {
		for fileName := range fileConfigs {
			fileConfig := fileConfigs.Sub(fileName)
			file, err := parseRawFile(fileConfig, defaults)
			if err != nil {
				return nil, err
			}
			files[fileName] = file
		}
	}

	return &RawDirectory{
		Alias:    cfg.String("alias"),
		FuseVars: cfg.StringSlice("fuse"),
		Defaults: defaults,
		Files:    files,
	}, nil
}

func parseRawFile(cfg config.Raw, defaults *Defaults) (*RawFile, error) {
	file := &RawFile{
		Alias: cfg.String("alias"),
	}

	if defaults != nil {
		file.Schedule = defaults.Schedule
		file.Sort = defaults.Sort
		file.Purge = defaults.Purge
		file.RetentionCount = defaults.RetentionCount
		file.RetentionAge = defaults.RetentionAge
	}

	if cfg.Has("schedule") {
		schedule, err := cronexpr.Parse(cfg.String("schedule"))
		if err != nil {
			return nil, nil
		}
		file.Schedule = schedule
	}

	if cfg.Has("sort") {
		file.Sort = cfg.String("sort")
	}

	if cfg.Has("purge") {
		file.Purge = cfg.Bool("purge")
	}

	if cfg.Has("retention-count") {
		file.RetentionCount = cfg.Uint64("retention-count")
	}

	if cfg.Has("retention-age") {
		file.RetentionAge = cfg.Duration("retention-age")
	}

	return file, nil
}

func parseDefaults(cfg config.Raw) (*Defaults, error) {
	if cfg == nil {
		return nil, nil
	}

	schedule, err := cronexpr.Parse(cfg.String("schedule"))
	if err != nil {
		return nil, nil
	}

	defaults := &Defaults{
		Schedule:       schedule,
		Sort:           cfg.String("sort"),
		RetentionCount: cfg.Uint64("retention-count"),
		RetentionAge:   cfg.Duration("retention-age"),
		Purge:          cfg.Bool("purge"),
	}

	return defaults, nil
}
