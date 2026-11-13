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
	AssumeRoleArn     string
	Endpoint          string
	TLSSkipVerify     bool
	Proxy             string
	ForcePathStyle    bool
	Token             string
	AutoDiscoverDisks bool
	Disks             *DisksConfiguration
	// GCS is set for Google Cloud Storage environments only; Endpoint, Proxy, AutoDiscoverDisks and Disks apply as well
	GCS *GCSConfiguration
}

// GCSConfiguration contains the settings specific to the `gcs:` section of an environment
type GCSConfiguration struct {
	// ProjectID is the project whose buckets are listed during auto-discovery
	ProjectID string
	// CredentialsFile is the path to a service account key file; mutually exclusive with CredentialsJSON
	CredentialsFile string
	// CredentialsJSON is the content of a service account key file
	CredentialsJSON string
	// Anonymous disables authentication, e.g. for public buckets or emulators
	Anonymous bool
}
