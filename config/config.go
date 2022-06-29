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
	environments []*EnvironmentConfiguration
	global       *GlobalConfiguration
	downloads 	 *DownloadsConfiguration
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

func (c *configuration) Global() *GlobalConfiguration {
	return c.global
}

func (c *configuration) Environments() []*EnvironmentConfiguration {
	return c.environments
}

func (c *configuration) Downloads() *DownloadsConfiguration {
	return c.downloads
}

func initConfig() {
	var file *os.File = nil
	var err error = nil
	
	flag.Parse()

	if hasGlobalDebugEnabled {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug log level enabled")
	}

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

	instance.global = parseGlobalSection(cfg)
	instance.downloads = parseDownloadsSection(cfg.Sub("downloads"))
	instance.environments = parseEnvironmentsSection(cfg.Sub("environments"))
}

func parseDownloadsSection(cfg Raw) *DownloadsConfiguration {
	var r *DownloadsConfiguration

	const paramEnabled = "enabled" 

	enabled := false
	if (cfg.Has(paramEnabled)) {
		enabled = cfg.Bool(paramEnabled)
	}

	log.Infof("Downloads enabled: %t", enabled)

	r = &DownloadsConfiguration{
		Enabled: enabled,
	}

	return r
}

func parseGlobalSection(cfg Raw) *GlobalConfiguration {
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
	if cfg.Has("ignore_disks") {
		for _, disk := range cfg.StringSlice("ignore_disks") {
			log.Debugf("Ignoring disk %s", disk)
			ignore[disk] = struct{}{}
		}
	}

	return &GlobalConfiguration{
		logLevel: logLevel, 
		httpPort: httpPort, 
		updateInterval: updateInterval, 
		ignored: ignore,
	}
}

func parseEnvironmentsSection(cfg Raw) []*EnvironmentConfiguration {
	var envs []*EnvironmentConfiguration

	for envName := range cfg {
		envCfg := cfg.Sub(envName)

		env, err := parseEnvironmentSection(envCfg, envName)

		if err != nil {
			log.Errorf("Environment '%s' could not be parsed: %s", envName, err)
			continue
		}

		envs = append(envs, env)
	}

	if len(envs) == 0 {
		log.Fatalf("No valid environments defined in configuration file. Did you miss the `environments` section?")
	}

	return envs
}

func parseEnvironmentSection(cfg Raw, envName string) (*EnvironmentConfiguration, error) {
	if envName == "" {
		return nil, errors.New("Missing environment name")
	}

	if cfg == nil || envName == "" {
		return nil, errors.New("Missing environment configuration entries")
	}

	var c *ClientConfiguration
	const paramRegion = "region"
	const paramForcePathStyle = "force_path_style"
	const paramAccessKeyId = "access_key_id"
	const paramSecretAccessKey = "secret_access_key"
	const paramEndpoint = "endpoint"
	const paramToken = "token"

	// check if local env oder S3 env
	if cfg.Has("path") {
		path := cfg.String("path")
		if path == "" {
			return nil, errors.New("Parameter 'path' has been set, but is empty")
		}

		c = &ClientConfiguration{
			Directory: path, 
			EnvName: envName,
		}
	} else {
		region := "eu-central-1"
		if cfg.Has(paramRegion) {
			region = cfg.String(paramRegion)
		}

		forcePathStyle := false
		if (cfg.Has(paramForcePathStyle)) {
			forcePathStyle = cfg.Bool(paramForcePathStyle)
		}

		c = &ClientConfiguration{
			EnvName:        envName,
			Region:         region,
			ForcePathStyle: forcePathStyle,
			AccessKey:      cfg.String(paramAccessKeyId),
			SecretKey:      cfg.String(paramSecretAccessKey),
			Endpoint:       cfg.String(paramEndpoint),
			Token:          cfg.String(paramToken),
		}
	}

	definitions := "backup_definitions.yaml"
	if cfg.Has("definitions") {
		definitions = cfg.String("definitions")
	}

	return &EnvironmentConfiguration{
		Name: envName, 
		Client: c, 
		Definitions: definitions,
	}, nil
}
