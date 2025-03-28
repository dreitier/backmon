package config

type EnvironmentConfiguration struct {
	Name        string
	Definitions string
	Client      *ClientConfiguration
}

type ClientConfiguration struct {
	EnvName           string
	Directory         string
	Region            string
	AccessKey         string
	SecretKey         string
	Endpoint          string
	Insecure          bool
	TLSSkipVerify     bool
	ForcePathStyle    bool
	Token             string
	AutoDiscoverDisks bool
	Disks             *DisksConfiguration
}
