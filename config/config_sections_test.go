package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// rawSub parses a YAML string and returns the sub-section under key.
func rawSub(t *testing.T, yaml string, key string) Raw {
	t.Helper()
	raw, err := ParseFromString(yaml)
	assert.NoError(t, err)
	return raw.Sub(key)
}

func Test_parseBasicAuthConfiguration(t *testing.T) {
	assertion := assert.New(t)

	// both username and password present -> configured
	full := parseBasicAuthConfiguration(rawSub(t, `
basic_auth:
  username: alice
  password: s3cret
`, "basic_auth"))
	assertion.NotNil(full)
	assertion.Equal("alice", full.Username)
	assertion.Equal("s3cret", full.Password)

	// empty password -> nil
	emptyPw := parseBasicAuthConfiguration(rawSub(t, `
basic_auth:
  username: alice
  password: ""
`, "basic_auth"))
	assertion.Nil(emptyPw)

	// missing password key -> nil
	noPw := parseBasicAuthConfiguration(rawSub(t, `
basic_auth:
  username: alice
`, "basic_auth"))
	assertion.Nil(noPw)
}

func Test_parseTlsConfiguration(t *testing.T) {
	assertion := assert.New(t)

	// key + certificate, strict omitted -> IsStrict false
	defaultStrict := parseTlsConfiguration(rawSub(t, `
tls:
  key: /etc/key.pem
  certificate: /etc/cert.pem
`, "tls"))
	assertion.NotNil(defaultStrict)
	assertion.Equal("/etc/key.pem", defaultStrict.PrivateKeyPath)
	assertion.Equal("/etc/cert.pem", defaultStrict.CertificatePath)
	assertion.False(defaultStrict.IsStrict)

	// strict explicitly enabled
	strict := parseTlsConfiguration(rawSub(t, `
tls:
  key: /etc/key.pem
  certificate: /etc/cert.pem
  strict: true
`, "tls"))
	assertion.NotNil(strict)
	assertion.True(strict.IsStrict)

	// missing certificate -> nil
	noCert := parseTlsConfiguration(rawSub(t, `
tls:
  key: /etc/key.pem
`, "tls"))
	assertion.Nil(noCert)

	// empty key -> nil
	emptyKey := parseTlsConfiguration(rawSub(t, `
tls:
  key: ""
  certificate: /etc/cert.pem
`, "tls"))
	assertion.Nil(emptyKey)
}

func Test_parseHttpSection(t *testing.T) {
	assertion := assert.New(t)

	// full http section with both sub-configs
	http := parseHttpSection(rawSub(t, `
http:
  basic_auth:
    username: alice
    password: s3cret
  tls:
    key: /etc/key.pem
    certificate: /etc/cert.pem
`, "http"))
	assertion.NotNil(http)
	assertion.NotNil(http.BasicAuth)
	assertion.NotNil(http.Tls)

	// empty http section -> both nil
	empty := parseHttpSection(rawSub(t, `
http: {}
`, "http"))
	assertion.NotNil(empty)
	assertion.Nil(empty.BasicAuth)
	assertion.Nil(empty.Tls)
}

func Test_parseDownloadsSection(t *testing.T) {
	assertion := assert.New(t)

	enabled := parseDownloadsSection(rawSub(t, `
downloads:
  enabled: true
`, "downloads"))
	assertion.NotNil(enabled)
	assertion.True(enabled.Enabled)

	// missing enabled key -> defaults to false
	def := parseDownloadsSection(rawSub(t, `
downloads: {}
`, "downloads"))
	assertion.NotNil(def)
	assertion.False(def.Enabled)
}

func Test_parseEnvironmentSection_errors(t *testing.T) {
	assertion := assert.New(t)

	_, err := parseEnvironmentSection(Raw{}, "")
	assertion.Error(err)

	_, err = parseEnvironmentSection(nil, "default")
	assertion.Error(err)

	// path present but empty
	emptyPath := rawSub(t, `
default:
  path: ""
`, "default")
	_, err = parseEnvironmentSection(emptyPath, "default")
	assertion.Error(err)

	// neither path nor s3
	noStorage := rawSub(t, `
default:
  definitions: foo.yaml
`, "default")
	_, err = parseEnvironmentSection(noStorage, "default")
	assertion.Error(err)
}

func Test_parseEnvironmentSection_localPath(t *testing.T) {
	assertion := assert.New(t)

	cfg := rawSub(t, `
default:
  path: /mnt/backup
  definitions: custom_definitions.yaml
`, "default")
	env, err := parseEnvironmentSection(cfg, "default")
	assertion.NoError(err)
	assertion.NotNil(env)
	assertion.Equal("default", env.Name)
	assertion.Equal("custom_definitions.yaml", env.Definitions)
	assertion.NotNil(env.Client)
	assertion.Equal("/mnt/backup", env.Client.Directory)
	assertion.Equal("default", env.Client.EnvName)
}

func Test_parseEnvironmentSection_s3Defaults(t *testing.T) {
	assertion := assert.New(t)

	cfg := rawSub(t, `
prod:
  s3:
    access_key_id: AKIA
    secret_access_key: shhh
`, "prod")
	env, err := parseEnvironmentSection(cfg, "prod")
	assertion.NoError(err)
	assertion.NotNil(env)
	// default definitions filename when not overridden
	assertion.Equal("backup_definitions.yaml", env.Definitions)
	assertion.NotNil(env.Client)
	assertion.Equal("eu-central-1", env.Client.Region)
	assertion.True(env.Client.AutoDiscoverDisks)
	assertion.False(env.Client.ForcePathStyle)
	assertion.Equal("AKIA", env.Client.AccessKey)
	assertion.Equal("shhh", env.Client.SecretKey)
}

func Test_Configuration_accessors(t *testing.T) {
	assertion := assert.New(t)

	raw, err := ParseFromString(`
port: 8080
http:
  basic_auth:
    username: alice
    password: s3cret
downloads:
  enabled: true
environments:
  default:
    path: /mnt/backup
`)
	assertion.NoError(err)

	sut := NewConfigurationInstance(raw)
	assertion.NotNil(sut)

	assertion.NotNil(sut.Global())
	assertion.Equal(8080, sut.Global().HttpPort())

	assertion.NotNil(sut.Http())
	assertion.NotNil(sut.Http().BasicAuth)

	assertion.NotNil(sut.Downloads())
	assertion.True(sut.Downloads().Enabled)

	assertion.Equal(float64(1), sut.TotalEnvironments())
}

func Test_Configuration_TotalEnvironments_nil(t *testing.T) {
	assertion := assert.New(t)
	sut := &Configuration{}
	assertion.Equal(float64(0), sut.TotalEnvironments())
}
