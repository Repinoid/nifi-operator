package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	s3Client   *minio.Client
	bucketName = "terraform-registry" // Default
	hostname   = os.Getenv("REGISTRY_HOSTNAME")
)

// Terraform Registry Protocol Structs
type Discovery struct {
	ProvidersV1 string `json:"providers.v1"`
}

type VersionList struct {
	ID       string    `json:"id"`
	Versions []Version `json:"versions"`
	Warnings []string  `json:"warnings"`
}

type Version struct {
	Version   string     `json:"version"`
	Protocols []string   `json:"protocols"`
	Platforms []Platform `json:"platforms"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type DownloadResponse struct {
	Protocols           []string    `json:"protocols"`
	OS                  string      `json:"os"`
	Arch                string      `json:"arch"`
	Filename            string      `json:"filename"`
	DownloadURL         string      `json:"download_url"`
	ShasumsURL          string      `json:"shasums_url"`
	ShasumsSignatureURL string      `json:"shasums_signature_url"`
	Shasum              string      `json:"shasum"`
	SigningKeys         SigningKeys `json:"signing_keys"`
}

type SigningKeys struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys"`
}

type GPGPublicKey struct {
	KeyID      string `json:"key_id"`
	ASCIIArmor string `json:"ascii_armor"`
}

func main() {
	// 1. Init S3 Connection
	if hostname == "" {
		hostname = "localhost:8080"
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyID := os.Getenv("S3_ACCESS_KEY")
	secretAccessKey := os.Getenv("S3_SECRET_KEY")

	if os.Getenv("S3_BUCKET") != "" {
		bucketName = os.Getenv("S3_BUCKET")
	}

	var err error
	s3Client, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true, // Force secure for cloud S3
	})
	if err != nil {
		log.Fatalln(err)
	}

	// 2. HTTP Handlers
	http.HandleFunc("/.well-known/terraform.json", discoveryHandler)
	http.HandleFunc("/v1/providers/", router)
	http.HandleFunc("/v1/proxy", proxyHandler)
	http.HandleFunc("/docs/", docsHandler)
	http.Handle("/", http.HandlerFunc(rootHandler))

	log.Printf("Starting Registry Service on :8080 (Bucket: %s, Endpoint: %s)\n", bucketName, endpoint)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head><title>Terra Registry</title></head>
<body>
    <h1>Terra Registry & Documentation Server</h1>
    <p>Status: <span style="color: green">ONLINE</span></p>
    <hr>
    <p>Powered by Nubes Cloud S3 Storage</p>
</body>
</html>
`)
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	bucket := r.URL.Query().Get("bucket")
	key := r.URL.Query().Get("key")

	if bucket == "" || key == "" {
		http.Error(w, "Missing bucket or key params", http.StatusBadRequest)
		return
	}

	obj, err := s3Client.GetObject(context.Background(), bucket, key, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("Error getting object %s/%s: %v", bucket, key, err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		log.Printf("Error stating object %s/%s: %v", bucket, key, err)
		http.Error(w, "File not found or not accessible", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Last-Modified", stat.LastModified.Format(http.TimeFormat))

	if _, err := io.Copy(w, obj); err != nil {
		log.Printf("Error streaming object: %v", err)
	}
}

func discoveryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Discovery{ProvidersV1: "/v1/providers/"})
}

func router(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/providers/")
	parts := strings.Split(path, "/")

	if len(parts) == 3 && parts[2] == "versions" {
		listVersions(w, r, parts[0], parts[1])
		return
	}

	if len(parts) == 6 && parts[3] == "download" {
		downloadVersion(w, r, parts[0], parts[1], parts[2], parts[4], parts[5])
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

func listVersions(w http.ResponseWriter, r *http.Request, namespace, pType string) {
	prefix := fmt.Sprintf("%s/%s/%s/", hostname, namespace, pType)
	ctx := context.Background()
	versions := []Version{}
	seenVersions := map[string]*Version{}

	objectCh := s3Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		parts := strings.Split(object.Key, "/")
		if len(parts) < 5 {
			continue
		}
		verStr := parts[3]
		fileName := parts[4]

		if _, ok := seenVersions[verStr]; !ok {
			seenVersions[verStr] = &Version{
				Version:   verStr,
				Protocols: []string{"5.0"},
				Platforms: []Platform{},
			}
		}

		if strings.Contains(fileName, "_linux_amd64.zip") {
			seenVersions[verStr].Platforms = append(seenVersions[verStr].Platforms, Platform{OS: "linux", Arch: "amd64"})
		}
		if strings.Contains(fileName, "_windows_amd64.zip") {
			seenVersions[verStr].Platforms = append(seenVersions[verStr].Platforms, Platform{OS: "windows", Arch: "amd64"})
		}
	}

	for _, v := range seenVersions {
		versions = append(versions, *v)
	}

	resp := VersionList{
		ID:       fmt.Sprintf("%s/%s", namespace, pType),
		Versions: versions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func downloadVersion(w http.ResponseWriter, r *http.Request, namespace, pType, version, osType, arch string) {
	basePath := fmt.Sprintf("%s/%s/%s/%s", hostname, namespace, pType, version)
	filename := fmt.Sprintf("terraform-provider-%s_%s_%s_%s.zip", pType, version, osType, arch)
	fullKey := fmt.Sprintf("%s/%s", basePath, filename)
	shasumsKey := fmt.Sprintf("%s/terraform-provider-%s_%s_SHA256SUMS", basePath, pType, version)
	sigKey := fmt.Sprintf("%s/terraform-provider-%s_%s_SHA256SUMS.sig", basePath, pType, version)

	var shasumValue string
	shasumsObj, err := s3Client.GetObject(context.Background(), bucketName, shasumsKey, minio.GetObjectOptions{})
	if err == nil {
		defer shasumsObj.Close()
		scanner := bufio.NewScanner(shasumsObj)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, filename) {
				fields := strings.Fields(line)
				if len(fields) >= 1 {
					shasumValue = fields[0]
				}
				break
			}
		}
	}

	baseURL := "https://" + hostname
	downloadLink := fmt.Sprintf("%s/v1/proxy?bucket=%s&key=%s", baseURL, bucketName, url.QueryEscape(fullKey))
	shasumsLink := fmt.Sprintf("%s/v1/proxy?bucket=%s&key=%s", baseURL, bucketName, url.QueryEscape(shasumsKey))
	sigLink := fmt.Sprintf("%s/v1/proxy?bucket=%s&key=%s", baseURL, bucketName, url.QueryEscape(sigKey))

	gpgKey := GPGPublicKey{
		KeyID: "866FD93D456DCA800F2448413EC4673EB798238A",
		ASCIIArmor: `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
6cZ0NQ==
=3Ea2
-----END PGP PUBLIC KEY BLOCK-----`,
	}

	resp := DownloadResponse{
		Protocols:           []string{"5.0"},
		OS:                  osType,
		Arch:                arch,
		Filename:            filename,
		DownloadURL:         downloadLink,
		ShasumsURL:          shasumsLink,
		ShasumsSignatureURL: sigLink,
		Shasum:              shasumValue,
		SigningKeys: SigningKeys{
			GPGPublicKeys: []GPGPublicKey{gpgKey},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
