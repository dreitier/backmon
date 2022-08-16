package config

type HttpConfiguration struct {
	BasicAuth   *BasicAuthConfiguration
	Tls         *TlsConfiguration
}

type BasicAuthConfiguration struct {
	Username      string
	Password      string
}

type TlsConfiguration struct {
	CertificatePath string
	PrivateKeyPath  string
	IsStrict        bool
}