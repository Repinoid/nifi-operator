package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// parseDocsRequestPath parses paths like:
// /docs/<namespace>/<name>/<version>/...  (rest may be empty)
func parseDocsRequestPath(p string) (namespace, name, version, rest string, err error) {
p = strings.TrimPrefix(p, "/")
p = strings.TrimPrefix(p, "docs/")
parts := strings.SplitN(p, "/", 4)
if len(parts) < 3 {
err = fmt.Errorf("invalid docs path: %s", p)
return
}
namespace = parts[0]
name = parts[1]
version = parts[2]
if len(parts) == 3 {
rest = ""
} else {
rest = parts[3]
}
return
}

// docsObjectKey builds the S3 key for a docs object given parsed parts.
func docsObjectKey(namespace, name, version, pathPart string) string {
clean := strings.TrimPrefix(pathPart, "/")
if clean == "" {
return fmt.Sprintf("docs/%s/%s/%s/index.html", namespace, name, version)
}
return fmt.Sprintf("docs/%s/%s/%s/%s", namespace, name, version, clean)
}

// tryCandidateKeys returns a list of keys to attempt for a given request path.
// Order matters: exact path first, then <path>/index.html, then top-level index.
func tryCandidateKeys(namespace, name, version, rest string) []string {
keys := []string{}
if rest == "" {
keys = append(keys, docsObjectKey(namespace, name, version, "index.html"))
return keys
}
// exact
keys = append(keys, docsObjectKey(namespace, name, version, rest))
// if it looks like a directory or has no extension, try index under it
if strings.HasSuffix(rest, "/") || filepath.Ext(rest) == "" {
keys = append(keys, docsObjectKey(namespace, name, version, strings.TrimSuffix(rest, "/")+"/index.html"))
}
// finally, try root index
keys = append(keys, docsObjectKey(namespace, name, version, "index.html"))
return keys
}

// docsHandler serves static documentation files from S3 (public-facing via Ingress).
func docsHandler(w http.ResponseWriter, r *http.Request) {
ns, name, ver, rest, err := parseDocsRequestPath(r.URL.Path)
if err != nil {
http.Error(w, "Bad docs path", http.StatusBadRequest)
return
}

ctx := context.Background()
candidates := tryCandidateKeys(ns, name, ver, rest)
log.Printf("Docs candidates (host=%s): %v", hostname, candidates)

var lastErr error
for _, key := range candidates {
obj, err := s3Client.GetObject(ctx, bucketName, key, minio.GetObjectOptions{})
if err != nil {
lastErr = err
continue
}
stat, err := obj.Stat()
if err != nil {
lastErr = err
_ = obj.Close()
continue
}

// Determine content-type
ext := filepath.Ext(key)
ctype := mime.TypeByExtension(ext)
if ctype == "" {
// fallback for HTML
if ext == ".html" || strings.HasSuffix(key, "index.html") {
ctype = "text/html; charset=utf-8"
} else {
ctype = "application/octet-stream"
}
}

w.Header().Set("Content-Type", ctype)
w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
w.Header().Set("Last-Modified", stat.LastModified.Format(http.TimeFormat))

if _, err := io.Copy(w, obj); err != nil {
log.Printf("Error streaming object %s: %v", key, err)
}
_ = obj.Close()
return
}

log.Printf("Docs not found in candidates: %v, lastErr: %v", candidates, lastErr)
http.Error(w, "Documentation not found", http.StatusNotFound)
}
