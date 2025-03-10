package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Configuration struct {
	global       *GlobalConfiguration
	http         *HttpConfiguration
	downloads    *DownloadsConfiguration
	environments []*EnvironmentConfiguration
}

var (
	instance                    *Configuration
	once                        sync.Once
	configSearchDirectories     []string
	hasGlobalDebugEnabled       bool
	isRunningInBackgroundForced bool
)

const (
	CfgFileName = "config.yaml"
	PathLocal   = "."
	PathGlobal  = "/etc/backmon"
)

func init() {
	flag.BoolVar(&hasGlobalDebugEnabled, "debug", false, "Enable debug log; overwrites any configuration file loglevel")
	flag.BoolVar(&isRunningInBackgroundForced, "background", false, "Run in background; no interactive terminal")

	configSearchDirectories = append(configSearchDirectories, PathLocal)

	userHome, err := os.UserHomeDir()

	if err == nil {
		userHome = fmt.Sprintf("%s%c%s", userHome, os.PathSeparator, ".backmon")
		configSearchDirectories = append(configSearchDirectories, userHome)
	}

	configSearchDirectories = append(configSearchDirectories, PathGlobal)
}

func IsRunningInBackgroundForced() bool {
	return isRunningInBackgroundForced
}

func HasGlobalDebugEnabled() bool {
	return hasGlobalDebugEnabled
}

func GetInstance() *Configuration {
	once.Do(func() {
		instance = CreateFromConfigurationFiles()
	})
	return instance
}

func (c *Configuration) Global() *GlobalConfiguration {
	return c.global
}

func (c *Configuration) Environments() []*EnvironmentConfiguration {
	return c.environments
}

func (c *Configuration) TotalEnvironments() float64 {
	if nil == c.Environments() {
		return float64(0)
	}

	return float64(len(c.Environments()))
}

func (c *Configuration) Downloads() *DownloadsConfiguration {
	return c.downloads
}

func (c *Configuration) Http() *HttpConfiguration {
	return c.http
}

// CreateFromConfigurationFiles Create a new configuration from default configuration files
func CreateFromConfigurationFiles() *Configuration {
	var file *os.File = nil
	var err error = nil

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

	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	cfg, err := Parse(file)
	if err != nil {
		log.Fatalf("Failed to parse configuration file: %s", err)
	}

	return NewConfigurationInstance(cfg)
}

// NewConfigurationInstance Parse all section
func NewConfigurationInstance(cfg Raw) *Configuration {
	var r *Configuration

	var globalConfiguration = parseGlobalSection(cfg)
	var httpConfiguration = parseHttpSection(cfg.Sub("http"))
	var downloadsConfiguration = parseDownloadsSection(cfg.Sub("downloads"))
	var environmentsConfiguration = parseEnvironmentsSection(cfg.Sub("environments"))

	r = &Configuration{
		global:       globalConfiguration,
		http:         httpConfiguration,
		downloads:    downloadsConfiguration,
		environments: environmentsConfiguration,
	}

	return r
}

// ParseDisksSection Parses `disks:` section
func ParseDisksSection(cfg Raw) *DisksConfiguration {
	var r *DisksConfiguration

	const paramInclude = "include"
	const paramExclude = "exclude"
	const paramAllOthers = "all_others"
	// possible values for `all_others`
	const paramAllOthersValueInclude = paramInclude
	const paramAllOthersValueExclude = paramExclude

	// #5:UC2: include is the default behaviour
	allOthers := DisksBehaviourInclude

	if cfg.Has(paramAllOthers) {
		rawAllOthers := cfg.String(paramAllOthers)

		switch rawAllOthers {
		case paramAllOthersValueInclude:
			allOthers = DisksBehaviourInclude
			break
		case paramAllOthersValueExclude:
			allOthers = DisksBehaviourExclude
			break
		default:
			log.Warnf("Unknown value for %s. Using 'include' as default value", paramAllOthers)
		}
	}

	includeDisks, includeRegExps := parseIncludeExcludeSection(cfg.StringSlice(paramInclude))
	excludeDisks, excludeRegExps := parseIncludeExcludeSection(cfg.StringSlice(paramExclude))

	r = &DisksConfiguration{
		behaviourForAllOthers: allOthers,
		include:               includeDisks,
		includeRegExps:        includeRegExps,
		exclude:               excludeDisks,
		excludeRegExps:        excludeRegExps,
	}

	return r
}

// Parses `disks.include` and `disks.exclude`
func parseIncludeExcludeSection(rawDiskNames []string) ( /*diskNames */ map[string]SingleDiskConfiguration /* diskRegExps */, []SingleDiskConfiguration) {
	var diskNames = make(map[string]SingleDiskConfiguration)
	var diskRegExps []SingleDiskConfiguration

	for _, diskName := range rawDiskNames {
		singleDiskConfiguration, err := NewSingleDiskConfiguration(diskName)

		if err != nil {
			log.Warnf("Ignoring disk '%s': %s", diskName, err)
			continue
		}

		// put it in the correct bucket
		if singleDiskConfiguration.IsRegularExpression {
			diskRegExps = append(diskRegExps, *singleDiskConfiguration)
		} else {
			diskNames[singleDiskConfiguration.Name] = *singleDiskConfiguration
		}
	}

	return diskNames, diskRegExps
}

func parseHttpSection(cfg Raw) *HttpConfiguration {
	var r *HttpConfiguration

	const paramBasicAuth = "basic_auth"
	const paramTls = "tls"

	var basicAuthConfiguration *BasicAuthConfiguration
	var tlsConfiguration *TlsConfiguration

	if cfg.Has(paramBasicAuth) {
		basicAuthConfiguration = parseBasicAuthConfiguration(cfg.Sub(paramBasicAuth))
	}

	log.Infof("Using HTTP Basic auth: %t", basicAuthConfiguration != nil)

	if cfg.Has(paramTls) {
		tlsConfiguration = parseTlsConfiguration(cfg.Sub(paramTls))
	}

	log.Infof("Using TLS: %t", tlsConfiguration != nil)

	r = &HttpConfiguration{
		BasicAuth: basicAuthConfiguration,
		Tls:       tlsConfiguration,
	}

	return r
}

func parseBasicAuthConfiguration(cfg Raw) *BasicAuthConfiguration {
	var r *BasicAuthConfiguration

	const paramUsername = "username"
	const paramPassword = "password"

	if cfg.Has(paramUsername) && cfg.Has(paramPassword) {
		username := cfg.String(paramUsername)
		password := cfg.String(paramPassword)

		if username != "" && password != "" {
			r = &BasicAuthConfiguration{
				Username: cfg.String(paramUsername),
				Password: cfg.String(paramPassword),
			}
		}
	}

	return r
}

func parseTlsConfiguration(cfg Raw) *TlsConfiguration {
	var r *TlsConfiguration

	const paramKey = "key"
	const paramCertificate = "certificate"
	const paramIsStrict = "strict"

	if cfg.Has(paramKey) && cfg.Has(paramCertificate) {
		key := cfg.String(paramKey)
		certificate := cfg.String(paramCertificate)
		isStrict := false

		if cfg.Has(paramIsStrict) {
			isStrict = cfg.Bool(paramIsStrict)
		}

		if key != "" && certificate != "" {
			r = &TlsConfiguration{
				CertificatePath: cfg.String(paramCertificate),
				PrivateKeyPath:  cfg.String(paramKey),
				IsStrict:        isStrict,
			}
		}
	}

	return r
}

func parseDownloadsSection(cfg Raw) *DownloadsConfiguration {
	var r *DownloadsConfiguration

	const paramEnabled = "enabled"

	enabled := false
	if cfg.Has(paramEnabled) {
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

	// if debug log level has not been enabled, set log level to info
	if hasGlobalDebugEnabled {
		logLevel = log.DebugLevel
	}

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

	return &GlobalConfiguration{
		logLevel:       logLevel,
		httpPort:       httpPort,
		updateInterval: updateInterval,
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
		return nil, errors.New("missing environment name")
	}

	if cfg == nil || envName == "" {
		return nil, errors.New("missing environment configuration entries")
	}

	var c *ClientConfiguration
	const paramRegion = "region"
	const paramForcePathStyle = "force_path_style"
	const paramInsecure = "insecure"
	const paramTLSSkipVerify = "tls_skip_verify"
	const paramAccessKeyId = "access_key_id"
	const paramSecretAccessKey = "secret_access_key"
	const paramEndpoint = "endpoint"
	const paramToken = "token"
	const paramAutoDiscoverDisks = "auto_discover_disks"

	// check if local env oder S3 env
	if cfg.Has("path") {
		path := cfg.String("path")
		if path == "" {
			return nil, errors.New("parameter 'path' has been set, but is empty")
		}

		var disks = ParseDisksSection(cfg.Sub("disks"))

		c = &ClientConfiguration{
			Directory: path,
			EnvName:   envName,
			Disks:     disks,
		}
	} else {
		region := "eu-central-1"
		if cfg.Has(paramRegion) {
			region = cfg.String(paramRegion)
		}

		forcePathStyle := false
		if cfg.Has(paramForcePathStyle) {
			forcePathStyle = cfg.Bool(paramForcePathStyle)
		}

		insecure := false
		if cfg.Has(paramInsecure) {
			insecure = cfg.Bool(paramInsecure)
		}

		tlsSkipVerify := false
		if cfg.Has(paramTLSSkipVerify) {
			tlsSkipVerify = cfg.Bool(paramTLSSkipVerify)
		}

		autoDiscoverDisks := true
		var disks = ParseDisksSection(cfg.Sub("disks"))

		if cfg.Has(paramAutoDiscoverDisks) {
			autoDiscoverDisks = cfg.Bool(paramAutoDiscoverDisks)
		}

		c = &ClientConfiguration{
			EnvName:           envName,
			Region:            region,
			ForcePathStyle:    forcePathStyle,
			Insecure:          insecure,
			TLSSkipVerify:     tlsSkipVerify,
			AccessKey:         cfg.String(paramAccessKeyId),
			SecretKey:         cfg.String(paramSecretAccessKey),
			Endpoint:          cfg.String(paramEndpoint),
			Token:             cfg.String(paramToken),
			AutoDiscoverDisks: autoDiscoverDisks,
			Disks:             disks,
		}
	}

	definitions := "backup_definitions.yaml"
	if cfg.Has("definitions") {
		definitions = cfg.String("definitions")
	}

	return &EnvironmentConfiguration{
		Name:        envName,
		Client:      c,
		Definitions: definitions,
	}, nil
}
