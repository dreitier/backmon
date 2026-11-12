package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3Client_httpClient_defaultsToSdkClient(t *testing.T) {
	client, err := (&S3Client{}).httpClient()

	if err != nil || client != nil {
		t.Errorf("got client %v and error %v, want neither without proxy and tls_skip_verify", client, err)
	}
}

func TestS3Client_httpClient_rejectsInvalidProxy(t *testing.T) {
	for _, proxy := range []string{"://missing-scheme", "not a url", "http://"} {
		c := &S3Client{Proxy: proxy, Region: "eu-central-1", AccessKey: "key", SecretKey: "secret"}

		if _, err := c.httpClient(); err == nil {
			t.Errorf("proxy %q: expected an error from httpClient", proxy)
		}

		if _, err := getClient(c); err == nil {
			t.Errorf("proxy %q: expected an error from getClient", proxy)
		}
	}
}

// Requests to the S3 endpoint must be sent to the configured proxy instead of the endpoint itself
func TestS3Client_sendsRequestsThroughProxy(t *testing.T) {
	var mu sync.Mutex
	var proxiedHosts []string

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		proxiedHosts = append(proxiedHosts, r.URL.Host)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Buckets></Buckets></ListAllMyBucketsResult>`))
	}))
	defer proxy.Close()

	c := &S3Client{
		Proxy:          proxy.URL,
		Region:         "eu-central-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		ForcePathStyle: true,
		// .invalid never resolves, so the request can only succeed through the proxy
		Endpoint: "http://s3.invalid",
	}

	client, err := getClient(c)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.ListBuckets(context.Background(), &s3.ListBucketsInput{}); err != nil {
		t.Fatalf("request through the proxy failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(proxiedHosts) == 0 || proxiedHosts[0] != "s3.invalid" {
		t.Errorf("proxy saw requests for %v, want one for s3.invalid", proxiedHosts)
	}
}

// `tls_skip_verify` must keep working with the customized HTTP client
func TestS3Client_tlsSkipVerify_acceptsSelfSignedCertificate(t *testing.T) {
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Buckets></Buckets></ListAllMyBucketsResult>`))
	}))
	defer endpoint.Close()

	for _, skipVerify := range []bool{true, false} {
		c := &S3Client{
			TLSSkipVerify:  skipVerify,
			Region:         "eu-central-1",
			AccessKey:      "key",
			SecretKey:      "secret",
			ForcePathStyle: true,
			Endpoint:       endpoint.URL,
		}

		client, err := getClient(c)
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.ListBuckets(context.Background(), &s3.ListBucketsInput{}, func(o *s3.Options) {
			o.RetryMaxAttempts = 1
		})

		if skipVerify && err != nil {
			t.Errorf("tls_skip_verify: request failed: %v", err)
		}

		if !skipVerify && err == nil {
			t.Error("expected the self-signed certificate to be rejected without tls_skip_verify")
		}
	}
}

// With static keys and assume_role_arn, the AssumeRole request to STS must also go through the proxy
func TestS3Client_assumeRoleWithStaticKeys_usesProxy(t *testing.T) {
	var mu sync.Mutex
	var proxiedHosts []string

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		proxiedHosts = append(proxiedHosts, r.Host)
		mu.Unlock()

		http.Error(w, "test proxy", http.StatusBadGateway)
	}))
	defer proxy.Close()

	c := &S3Client{
		Proxy:          proxy.URL,
		Region:         "eu-central-1",
		AccessKey:      "key",
		SecretKey:      "secret",
		AssumeRoleArn:  "arn:aws:iam::123456789012:role/backmon",
		ForcePathStyle: true,
		Endpoint:       "http://s3.invalid",
	}

	client, err := getClient(c)
	if err != nil {
		t.Fatal(err)
	}

	// the request fails at the test proxy; only the first attempt matters, so don't wait for the SDK's retries
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = client.ListBuckets(ctx, &s3.ListBucketsInput{})

	mu.Lock()
	defer mu.Unlock()

	for _, host := range proxiedHosts {
		if strings.HasPrefix(host, "sts.") {
			return
		}
	}

	t.Errorf("the AssumeRole request to STS did not go through the proxy, the proxy saw %v", proxiedHosts)
}
