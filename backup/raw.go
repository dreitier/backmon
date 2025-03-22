package backup

import (
	"fmt"
	"github.com/dreitier/backmon/config"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
	"io"
	"time"
)

const (
	keyDirectories = "directories"
	keyQuotas      = "quota"
)

type RawDefinition struct {
	quota       string
	directories map[string]*RawDirectory
}

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

func ParseRawDefinitions(definitionsReader io.Reader) (*RawDefinition, error) {
	cfg, err := config.Parse(definitionsReader)

	if err != nil {
		return nil, fmt.Errorf("failed to parse definitions file: %v", err)
	}

	var parsed RawDefinition
	parsed.directories = make(map[string]*RawDirectory)

	if cfg.Has(keyDirectories) {
		for dirName := range cfg.Sub(keyDirectories) {
			dirConfig := cfg.Sub(keyDirectories).Sub(dirName)
			dir, err := parseDirectorySection(dirConfig, dirName)

			if err != nil {
				return nil, err
			}

			parsed.directories[dirName] = dir
		}
	}

	if cfg.Has(keyQuotas) {
		parsed.quota = cfg.String(keyQuotas)
	}

	return &parsed, nil
}

func parseDirectorySection(cfg config.Raw, name string) (*RawDirectory, error) {
	defaults, err := parseDefaults(cfg.Sub("defaults"))
	const paramAlias = "alias"

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
			file, err := parseFileSection(fileConfig, defaults)

			if err != nil {
				return nil, err
			}

			files[fileName] = file
		}
	}

	var alias = name

	if cfg.Has(paramAlias) {
		alias = cfg.String(paramAlias)
	}

	return &RawDirectory{
		Alias:    alias,
		FuseVars: cfg.StringSlice("fuse"),
		Defaults: defaults,
		Files:    files,
	}, nil
}

func parseFileSection(cfg config.Raw, defaults *Defaults) (*RawFile, error) {
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

	cronExprString := cfg.String("schedule")
	log.Debugf("parsed cron expression is: %s", cronExprString)
	schedule, err := cronexpr.Parse(cronExprString)

	if err != nil {
		log.Errorf("failed to parse cron expression [%s]: %s", cronExprString, err)
		return nil, err
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
