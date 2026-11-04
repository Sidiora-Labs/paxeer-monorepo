package reconciliation

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type ListRequest struct {
	Bucket, Prefix, Token string
	MaxKeys               int
}
type Object struct {
	Body        []byte
	ContentType string
}

// Objects is a loopback S3 protocol fixture. Lists and reads use bytes published
// by the real SDK PutObject path, with opaque continuation tokens.
type Objects struct {
	mu      sync.Mutex
	objects map[string]map[string]Object
	lists   []ListRequest
	gets    []string
}

func NewObjects() *Objects { return &Objects{objects: map[string]map[string]Object{}} }
func (s *Objects) Requests() ([]ListRequest, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ListRequest(nil), s.lists...), append([]string(nil), s.gets...)
}
func (s *Objects) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
		maximum, err := strconv.Atoi(r.URL.Query().Get("max-keys"))
		if err != nil || maximum < 1 || maximum > 1000 {
			http.Error(w, "invalid max-keys", 400)
			return
		}
		prefix, token := r.URL.Query().Get("prefix"), r.URL.Query().Get("continuation-token")
		s.lists = append(s.lists, ListRequest{bucket, prefix, token, maximum})
		var after string
		if token != "" {
			decoded, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				http.Error(w, "invalid continuation-token", 400)
				return
			}
			after = string(decoded)
		}
		var keys []string
		for k := range s.objects[bucket] {
			if strings.HasPrefix(k, prefix) && k > after {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		truncated := len(keys) > maximum
		if truncated {
			keys = keys[:maximum]
		}
		type entry struct {
			Key  string
			Size int
		}
		result := struct {
			XMLName               xml.Name `xml:"ListBucketResult"`
			XMLNS                 string   `xml:"xmlns,attr"`
			Name, Prefix          string
			MaxKeys, KeyCount     int
			IsTruncated           bool
			NextContinuationToken string `xml:",omitempty"`
			Contents              []entry
		}{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Name: bucket, Prefix: prefix, MaxKeys: maximum, KeyCount: len(keys), IsTruncated: truncated}
		for _, k := range keys {
			result.Contents = append(result.Contents, entry{k, len(s.objects[bucket][k].Body)})
		}
		if truncated {
			result.NextContinuationToken = base64.StdEncoding.EncodeToString([]byte(keys[len(keys)-1]))
		}
		w.Header().Set("Content-Type", "application/xml")
		if err := xml.NewEncoder(w).Encode(result); err != nil {
			panic(err)
		}
		return
	}
	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, (32<<20)+1))
		if err != nil || len(body) > 32<<20 {
			http.Error(w, "invalid body", 400)
			return
		}
		if s.objects[bucket] == nil {
			s.objects[bucket] = map[string]Object{}
		}
		s.objects[bucket][key] = Object{append([]byte(nil), body...), r.Header.Get("Content-Type")}
		w.Header().Set("ETag", `"fixture-object"`)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		object, exists := s.objects[bucket][key]
		if !exists {
			http.Error(w, "not found", 404)
			return
		}
		s.gets = append(s.gets, key)
		w.Header().Set("Content-Type", object.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(object.Body)))
		w.Header().Set("ETag", `"fixture-object"`)
		_, _ = w.Write(object.Body)
	default:
		http.Error(w, fmt.Sprintf("unsupported method %s", r.Method), 405)
	}
}
