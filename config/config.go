package config

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sync"
	"time"
	"flag"
)

type configuration struct {
	environments []*Environment
	global       *global
}

var (
	instance *configuration
	once     sync.Once
	configSearchDirectories []string
	hasGlobalDebugEnabled bool
)

const (
	CfgFileName = "config.yaml"
	PathLocal   = "."
	PathGlobal  = "/etc/cloudmon"

)

func init() {
	flag.BoolVar(&hasGlobalDebugEnabled, "debug", false, "Enable debug log; overwrites any configuration file loglevel")
	flag.Parse()

	if hasGlobalDebugEnabled {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug log level enabled")
	}
	
	configSearchDirectories = append(configSearchDirectories, PathLocal)

	userHome, err := os.UserHomeDir()

	if  err == nil{
		userHome = fmt.Sprintf("%s%c%s", userHome, os.PathSeparator, ".cloudmon")
		configSearchDirectories = append(configSearchDirectories, userHome)
	}

	configSearchDirectories = append(configSearchDirectories, PathGlobal)
}

func HasGlobalDebugEnabled() bool {
	return hasGlobalDebugEnabled
}

func GetInstance() *configuration {
	once.Do(func() {
		instance = &configuration{}
		initConfig()
	})
	return instance
}

func (c *configuration) Global() *global {
	return c.global
}

func (c *configuration) Environments() []*Environment {
	return c.environments
}

func initConfig() {
	var file *os.File = nil
	var err error = nil

	for _, directory := range configSearchDirectories {
		var possibleConfigPath = filepath.Join(directory, CfgFileName)
		log.Debugf("Checking for configuration file at %s", possibleConfigPath)

		file, err = os.Open(possibleConfigPath)

		if err == nil {
			log.Infof("Found configuration file at location %s", possibleConfigPath)
			break
		}
	}

	if file == nil {
		log.Fatal("Could not find any configuration file")
	}

	defer file.Close()

	cfg, err := Parse(file)
	if err != nil {
		log.Fatalf("Failed to parse configuration file: %s", err)
	}

	instance.global = parseGlobal(cfg)
	instance.environments = parseEnvironments(cfg.Sub("environments"))
}

func parseGlobal(cfg Raw) *global {
	logLevel := log.InfoLevel
	if cfg.Has("log_level") {
		parsedLevel, err := log.ParseLevel(cfg.String("log_level"))
		if err == nil {
			logLevel = parsedLevel
		} else {
			log.Warnf("Cannot parse log level, defaulting to 'info': %s", err)
		}
	}

	httpPort := 80
	if cfg.Has("port") {
		httpPort = int(cfg.Int64("port"))
	}

	updateInterval := time.Hour
	if cfg.Has("update_interval") {
		updateInterval = cfg.Duration("update_interval")
	}

	if updateInterval < time.Minute {
		log.Warn("Update interval must not be less than 1 minute, defaulting to 1 hour.")
		updateInterval = time.Hour
	}

	ignore := make(map[string]struct{})
	if cfg.Has("ignore_buckets") {
		for _, bucket := range cfg.StringSlice("ignore_buckets") {
			ignore[bucket] = struct{}{}
		}
	}

	return &global{logLevel: logLevel, httpPort: httpPort, updateInterval: updateInterval, ignored: ignore}
}

func parseEnvironments(cfg Raw) []*Environment {
	var envs []*Environment

	for envName := range cfg {
		envCfg := cfg.Sub(envName)

		env, err := parseEnvironment(envCfg, envName)

		if err != nil {
			log.Errorf("environment '%s' could not be parsed, skipping", err)
			continue
		}

		envs = append(envs, env)
	}

	return envs
}

func parseEnvironment(cfg Raw, envName string) (*Environment, error) {

	if cfg == nil || envName == "" {
		return nil, errors.New("failed to parse environment")
	}

	var c *Client

	// check if local env oder S3 env
	if cfg.Has("path") {
		path := cfg.String("path")

		if path == "" {
			return nil, errors.New("failed to parse environment, path or S3 credentials must be given")
		}

		c = &Client{Directory: path, EnvName: envName}
	} else {
		region := "eu-central-1"
		if cfg.Has("region") {
			region = cfg.String("region")
		}
		c = &Client{
			EnvName:        envName,
			Region:         region,
			AccessKey:      cfg.String("access_key_id"),
			SecretKey:      cfg.String("secret_access_key"),
			Endpoint:       cfg.String("endpoint"),
			ForcePathStyle: cfg.Bool("force_path_style"),
			Token:          cfg.String("token"),
		}
	}

	definitions := "backup_definitions.yaml"
	if cfg.Has("definitions") {
		definitions = cfg.String("definitions")
	}

	return &Environment{Name: envName, Client: c, Definitions: definitions}, nil
}
