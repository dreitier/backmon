package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseGCSEnvironment(t *testing.T, yaml string) (*EnvironmentConfiguration, error) {
	t.Helper()

	raw, err := ParseFromString(yaml)
	require.NoError(t, err)

	return parseEnvironmentSection(raw.Sub("env"), "env")
}

// An empty `gcs:` section is sufficient: Application Default Credentials and auto-discovery
func Test_parseEnvironmentSection_gcsDefaults(t *testing.T) {
	for _, yaml := range []string{"env:\n  gcs:\n", "env:\n  gcs: {}\n"} {
		env, err := parseGCSEnvironment(t, yaml)
		require.NoError(t, err)

		assert.Equal(t, "env", env.Name)
		assert.Equal(t, "backup_definitions.yaml", env.Definitions)
		assert.Equal(t, "env", env.Client.EnvName)
		assert.Empty(t, env.Client.Directory)
		assert.Empty(t, env.Client.Endpoint)
		assert.Empty(t, env.Client.Proxy)
		assert.True(t, env.Client.AutoDiscoverDisks)
		assert.NotNil(t, env.Client.Disks)
		require.NotNil(t, env.Client.GCS)
		assert.Equal(t, GCSConfiguration{}, *env.Client.GCS)
	}
}

func Test_parseEnvironmentSection_gcs(t *testing.T) {
	t.Setenv("BACKMON_TEST_GCS_KEY", `{"type":"service_account"}`)

	env, err := parseGCSEnvironment(t, `
env:
  definitions: custom_definitions.yaml
  gcs:
    project_id: my-project
    credentials_json: __${BACKMON_TEST_GCS_KEY}__
    endpoint: http://localhost:4443
    proxy: http://proxy.example.com:3128
    auto_discover_disks: false
  disks:
    include:
      - my-bucket
    all_others: exclude
`)
	require.NoError(t, err)

	assert.Equal(t, "custom_definitions.yaml", env.Definitions)
	assert.Equal(t, "http://localhost:4443", env.Client.Endpoint)
	assert.Equal(t, "http://proxy.example.com:3128", env.Client.Proxy)
	assert.False(t, env.Client.AutoDiscoverDisks)
	assert.True(t, env.Client.Disks.IsDiskIncluded("my-bucket"))
	assert.False(t, env.Client.Disks.IsDiskIncluded("other-bucket"))

	require.NotNil(t, env.Client.GCS)
	assert.Equal(t, "my-project", env.Client.GCS.ProjectID)
	assert.Equal(t, `{"type":"service_account"}`, env.Client.GCS.CredentialsJSON)
	assert.Empty(t, env.Client.GCS.CredentialsFile)
	assert.False(t, env.Client.GCS.Anonymous)
}

func Test_parseEnvironmentSection_gcsCredentialsFileAndAnonymous(t *testing.T) {
	env, err := parseGCSEnvironment(t, "env:\n  gcs:\n    credentials_file: /etc/backmon/key.json\n")
	require.NoError(t, err)
	assert.Equal(t, "/etc/backmon/key.json", env.Client.GCS.CredentialsFile)

	env, err = parseGCSEnvironment(t, "env:\n  gcs:\n    anonymous: true\n")
	require.NoError(t, err)
	assert.True(t, env.Client.GCS.Anonymous)
}

func Test_parseEnvironmentSection_gcsInvalid_returnsError(t *testing.T) {
	cases := map[string]string{
		"credentials_file and credentials_json": "credentials_file: key.json\n    credentials_json: \"{}\"",
		"anonymous with credentials_file":       "anonymous: true\n    credentials_file: key.json",
		"anonymous with credentials_json":       "anonymous: true\n    credentials_json: \"{}\"",
		"endpoint without scheme":               "endpoint: localhost:4443",
		"endpoint with unsupported scheme":      "endpoint: gs://my-bucket",
		"endpoint without host":                 "endpoint: \"http://\"",
		"proxy without scheme":                  "proxy: proxy.example.com:3128",
	}

	for name, section := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseGCSEnvironment(t, "env:\n  gcs:\n    "+section+"\n")
			assert.Error(t, err)
		})
	}
}

// `path` and `s3` take precedence, as before `gcs` has been introduced
func Test_parseEnvironmentSection_gcsIsIgnoredForOtherStorages(t *testing.T) {
	env, err := parseGCSEnvironment(t, "env:\n  path: /mnt/backup\n  gcs:\n    project_id: my-project\n")
	require.NoError(t, err)
	assert.Nil(t, env.Client.GCS)
	assert.Equal(t, "/mnt/backup", env.Client.Directory)

	env, err = parseGCSEnvironment(t, "env:\n  s3:\n    region: eu-west-1\n  gcs:\n    project_id: my-project\n")
	require.NoError(t, err)
	assert.Nil(t, env.Client.GCS)
	assert.Equal(t, "eu-west-1", env.Client.Region)
}
