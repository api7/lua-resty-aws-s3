package main

import (
	"C"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)
import (
	"bytes"
	"context"
	"io"
	"unsafe"
)

//export getObject
func getObject(cBucket *C.char, cKey *C.char, cRegion *C.char, cAccessKey *C.char, cSecretKey *C.char, cCustomEndpoint *C.char) (unsafe.Pointer, unsafe.Pointer) {
	region := C.GoString(cRegion)
	accessKey := C.GoString(cAccessKey)
	secretKey := C.GoString(cSecretKey)
	customEndpoint := C.GoString(cCustomEndpoint)
	bucket := C.GoString(cBucket)
	key := C.GoString(cKey)

	var cfg aws.Config
	if accessKey == "" || secretKey == "" {
		c, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			return nil, unsafe.Pointer(C.CString(err.Error()))
		}
		cfg = c
	} else {
		creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
		cfg = aws.Config{
			Region:      region,
			Credentials: creds,
		}
	}

	if customEndpoint != "" {
		cfg.BaseEndpoint = &customEndpoint
	}
	cli := s3.NewFromConfig(cfg)

	out, err := cli.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, unsafe.Pointer(C.CString(err.Error()))
	}

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, unsafe.Pointer(C.CString(err.Error()))
	}

	return unsafe.Pointer(C.CString(string(body))), nil
}

//export putObject
func putObject(cBucket *C.char, cKey *C.char, cContent *C.char, cRegion *C.char, cAccessKey *C.char, cSecretKey *C.char, cCustomEndpoint *C.char) unsafe.Pointer {
	region := C.GoString(cRegion)
	accessKey := C.GoString(cAccessKey)
	secretKey := C.GoString(cSecretKey)
	customEndpoint := C.GoString(cCustomEndpoint)
	bucket := C.GoString(cBucket)
	key := C.GoString(cKey)
	content := C.GoString(cContent)

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	cfg := aws.Config{
		Region:      region,
		Credentials: creds,
	}
	if customEndpoint != "" {
		cfg.BaseEndpoint = &customEndpoint
	}
	cli := s3.NewFromConfig(cfg)

	_, err := cli.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})

	if err != nil {
		return unsafe.Pointer(C.CString(err.Error()))
	}

	return nil
}

func main() {
}
