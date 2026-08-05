package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"context"
	"io"
	"unsafe"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

//export getObject
func getObject(cBucket *C.char, cKey *C.char, cRegion *C.char, cAccessKey *C.char, cSecretKey *C.char, cCustomEndpoint *C.char, cUsePathStyle C.int) (unsafe.Pointer, unsafe.Pointer) {
	region := C.GoString(cRegion)
	accessKey := C.GoString(cAccessKey)
	secretKey := C.GoString(cSecretKey)
	customEndpoint := C.GoString(cCustomEndpoint)
	bucket := C.GoString(cBucket)
	key := C.GoString(cKey)
	usePathStyle := cUsePathStyle != 0

	cfg, err := getConfiguration(region, accessKey, secretKey, customEndpoint)
	if err != nil {
		return nil, unsafe.Pointer(C.CString(err.Error()))
	}
	cli := newS3Client(*cfg, usePathStyle)

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
func putObject(cBucket *C.char, cKey *C.char, cContent *C.char, cRegion *C.char, cAccessKey *C.char, cSecretKey *C.char, cCustomEndpoint *C.char, cUsePathStyle C.int) unsafe.Pointer {
	region := C.GoString(cRegion)
	accessKey := C.GoString(cAccessKey)
	secretKey := C.GoString(cSecretKey)
	customEndpoint := C.GoString(cCustomEndpoint)
	bucket := C.GoString(cBucket)
	key := C.GoString(cKey)
	content := C.GoString(cContent)
	usePathStyle := cUsePathStyle != 0

	cfg, err := getConfiguration(region, accessKey, secretKey, customEndpoint)
	if err != nil {
		return unsafe.Pointer(C.CString(err.Error()))
	}
	cli := newS3Client(*cfg, usePathStyle)

	_, err = cli.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})

	if err != nil {
		return unsafe.Pointer(C.CString(err.Error()))
	}

	return nil
}

func getConfiguration(region, accessKey, secretKey, customEndpoint string) (*aws.Config, error) {
	var cfg aws.Config
	if accessKey == "" || secretKey == "" {
		c, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
		if err != nil {
			return nil, err
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
	return &cfg, nil
}

func newS3Client(cfg aws.Config, usePathStyle bool) *s3.Client {
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
		// The backend is often an S3-compatible store rather than AWS S3, so opt
		// out of the SDK's default of attaching x-amz-checksum-* to every request
		// it can. Pinning it here also keeps both getConfiguration branches
		// consistent: only LoadDefaultConfig populates these two fields.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
}

//export freeCString
func freeCString(ptr unsafe.Pointer) {
	C.free(ptr)
}

func main() {
}
