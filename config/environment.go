package config

type Environment struct {
	Name        string
	Definitions string
	Client      *Client
}

type Client struct {
	EnvName        string
	Directory      string
	Region         string
	AccessKey      string
	SecretKey      string
	Endpoint       string
	ForcePathStyle bool
	Token          string
}
