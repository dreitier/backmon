package config

type HttpConfiguration struct {
	BasicAuth      *BasicAuthConfiguration
}

type BasicAuthConfiguration struct {
	Username      string
	Password      string
}