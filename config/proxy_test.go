package config

import "testing"

func parseS3Environment(t *testing.T, yaml string) (*EnvironmentConfiguration, error) {
	t.Helper()

	raw, err := ParseFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	return parseEnvironmentSection(raw.Sub("env"), "env")
}

func Test_parseEnvironmentSection_s3Proxy(t *testing.T) {
	env, err := parseS3Environment(t, `
env:
  s3:
    endpoint: http://minio:9000
    proxy: http://proxy.example.com:3128
`)
	if err != nil {
		t.Fatal(err)
	}

	if env.Client.Proxy != "http://proxy.example.com:3128" {
		t.Errorf("got proxy %q", env.Client.Proxy)
	}
}

func Test_parseEnvironmentSection_s3WithoutProxy(t *testing.T) {
	env, err := parseS3Environment(t, `
env:
  s3:
    endpoint: http://minio:9000
`)
	if err != nil {
		t.Fatal(err)
	}

	if env.Client.Proxy != "" {
		t.Errorf("got proxy %q, want none", env.Client.Proxy)
	}
}

func Test_parseEnvironmentSection_s3InvalidProxy_returnsError(t *testing.T) {
	for _, proxy := range []string{"proxy.example.com:3128", "http://", "not a url"} {
		_, err := parseS3Environment(t, "env:\n  s3:\n    proxy: \""+proxy+"\"\n")

		if err == nil {
			t.Errorf("proxy %q: expected an error", proxy)
		}
	}
}
