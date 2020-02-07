package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	osBase := os.Getenv("OE_BASE")
	if osBase == "" {
		log.Fatalln("OE_BASE must be set")
	}

	s3Key := os.Getenv("S3_KEY")
	if s3Key == "" {
		log.Fatalln("S3_KEY must be set")
	}

	s3Secret := os.Getenv("S3_SECRET")
	if s3Secret == "" {
		log.Fatalln("S3_SECRET must be set")
	}

	bucket := aws.String("yoe-feed")
	key := aws.String("wasabi-testobject2")

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3Key, s3Secret, ""),
		Endpoint:         aws.String("https://s3.wasabisys.com"),
		Region:           aws.String("us-east-1"),
		S3ForcePathStyle: aws.Bool(true),
	}
	newSession := session.New(s3Config)

	s3Client := s3.New(newSession)

	cparams := &s3.CreateBucketInput{
		Bucket: bucket, // Required
	}
	_, err := s3Client.CreateBucket(cparams)
	if err != nil {
		// Print if any error.
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("Successfully created bucket %s\n", *bucket)

	// Upload a new object "wasabi-testobject" with the string "Wasabi Hot storage"
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Body:   strings.NewReader("wasabi hot storage"),
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		fmt.Printf("Failed to upload object %s/%s, %s\n", *bucket, *key, err.Error())
		return
	}
	fmt.Printf("Successfully uploaded key %s\n", *key)

	//Get Object
	_, err = s3Client.GetObject(&s3.GetObjectInput{
		Bucket: bucket,
		Key:    key,
	})
	if err != nil {
		fmt.Println("Failed to download file", err)
		return
	}
	fmt.Printf("Successfully Downloaded key %s\n", *key)
}
