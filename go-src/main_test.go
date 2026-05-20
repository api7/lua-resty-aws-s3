package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
			var requests []string
			var stored string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.Host+r.URL.RequestURI())
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
					stored = string(data)
					w.WriteHeader(http.StatusOK)
				case http.MethodGet:
					_, _ = w.Write([]byte(stored))
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
				got, err := io.ReadAll(out.Body)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != body {
					t.Fatalf("expected body %q, got %q", body, string(got))
				}
				want := []string{
					"PUT minio.test:" + port + "/fallback-cp-config/resource-bucket/gwid?x-id=PutObject",
					"GET minio.test:" + port + "/fallback-cp-config/resource-bucket/gwid?x-id=GetObject",
				}
				if strings.Join(requests, "\n") != strings.Join(want, "\n") {
					t.Fatalf("expected requests %v, got %v", want, requests)
				}
				return
			}

			if err == nil {
				t.Fatal("expected PutObject to fail without path-style")
			}
			want := []string{
				"PUT resource-bucket.minio.test:" + port + "/fallback-cp-config/gwid?x-id=PutObject",
			}
			if strings.Join(requests, "\n") != strings.Join(want, "\n") {
				t.Fatalf("expected requests %v, got %v", want, requests)
			}
		})
	}
}
