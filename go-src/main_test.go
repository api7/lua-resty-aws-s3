package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestUsePathStyleWithEndpointPrefix(t *testing.T) {
	const (
		prefix = "/fallback-cp-config"
		bucket = "resource-bucket"
		key    = "gwid"
		body   = "payload"
	)

	for _, usePathStyle := range []bool{false, true} {
		t.Run("use_path_style_"+map[bool]string{false: "false", true: "true"}[usePathStyle], func(t *testing.T) {
			var mu sync.Mutex
			var requests []recordedRequest
			var stored string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, recordedRequest{method: r.Method, host: r.Host, path: r.URL.Path})
				mu.Unlock()
				if r.URL.Path != prefix+"/"+bucket+"/"+key {
					http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
					return
				}

				switch r.Method {
				case http.MethodPut:
					data, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					mu.Lock()
					stored = string(data)
					mu.Unlock()
					w.WriteHeader(http.StatusOK)
				case http.MethodGet:
					mu.Lock()
					body := stored
					mu.Unlock()
					_, _ = w.Write([]byte(body))
				default:
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()

			serverAddr := strings.TrimPrefix(server.URL, "http://")
			_, port, err := net.SplitHostPort(serverAddr)
			if err != nil {
				t.Fatal(err)
			}

			endpoint := "http://minio.test:" + port + prefix
			cli := newS3Client(aws.Config{
				Region:       "us-east-1",
				Credentials:  credentials.NewStaticCredentialsProvider("access", "secret", ""),
				BaseEndpoint: &endpoint,
				HTTPClient: &http.Client{
					Transport: &http.Transport{
						DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
							var d net.Dialer
							return d.DialContext(ctx, network, serverAddr)
						},
					},
					Timeout: 5 * time.Second,
				},
				Retryer: func() aws.Retryer {
					return retry.NewStandard(func(o *retry.StandardOptions) {
						o.MaxAttempts = 1
					})
				},
			}, usePathStyle)

			_, err = cli.PutObject(context.Background(), &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   strings.NewReader(body),
			})
			if usePathStyle {
				if err != nil {
					t.Fatal(err)
				}

				out, err := cli.GetObject(context.Background(), &s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(key),
				})
				if err != nil {
					t.Fatal(err)
				}
				defer out.Body.Close()
				got, err := io.ReadAll(out.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != body {
					t.Fatalf("expected body %q, got %q", body, string(got))
				}
				want := []recordedRequest{
					{method: "PUT", host: "minio.test:" + port, path: "/fallback-cp-config/resource-bucket/gwid"},
					{method: "GET", host: "minio.test:" + port, path: "/fallback-cp-config/resource-bucket/gwid"},
				}
				mu.Lock()
				gotRequests := append([]recordedRequest(nil), requests...)
				mu.Unlock()
				if strings.Join(formatRequests(gotRequests), "\n") != strings.Join(formatRequests(want), "\n") {
					t.Fatalf("expected requests %v, got %v", want, requests)
				}
				return
			}

			if err == nil {
				t.Fatal("expected PutObject to fail without path-style")
			}
			want := []recordedRequest{
				{method: "PUT", host: "resource-bucket.minio.test:" + port, path: "/fallback-cp-config/gwid"},
			}
			mu.Lock()
			gotRequests := append([]recordedRequest(nil), requests...)
			mu.Unlock()
			if strings.Join(formatRequests(gotRequests), "\n") != strings.Join(formatRequests(want), "\n") {
				t.Fatalf("expected requests %v, got %v", want, requests)
			}
		})
	}
}

func TestPutObjectSendsNoChecksumHeaders(t *testing.T) {
	const (
		bucket = "resource-bucket"
		key    = "gwid"
		body   = "payload"
	)

	t.Setenv("AWS_ACCESS_KEY_ID", "env-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "env-secret")

	for _, tc := range []struct {
		name      string
		accessKey string
		secretKey string
	}{
		{name: "static_credentials", accessKey: "access", secretKey: "secret"},
		{name: "default_credential_chain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var checksumHeaders []string
			var received string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				mu.Lock()
				received = string(data)
				for name := range r.Header {
					if strings.HasPrefix(strings.ToLower(name), "x-amz-checksum-") {
						checksumHeaders = append(checksumHeaders, name)
					}
				}
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg, err := getConfiguration("us-east-1", tc.accessKey, tc.secretKey, server.URL)
			if err != nil {
				t.Fatal(err)
			}

			_, err = newS3Client(*cfg, true).PutObject(context.Background(), &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   strings.NewReader(body),
			})
			if err != nil {
				t.Fatal(err)
			}

			mu.Lock()
			defer mu.Unlock()
			if len(checksumHeaders) != 0 {
				t.Fatalf("expected no x-amz-checksum-* headers, got %v", checksumHeaders)
			}
			if received != body {
				t.Fatalf("expected body %q to be sent verbatim, got %q", body, received)
			}
		})
	}
}

type recordedRequest struct {
	method string
	host   string
	path   string
}

func formatRequests(requests []recordedRequest) []string {
	out := make([]string, 0, len(requests))
	for _, req := range requests {
		out = append(out, req.method+" "+req.host+req.path)
	}
	return out
}
