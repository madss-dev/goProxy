package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type DomainTemplate struct {
	Patterns       []string `json:"patterns"`
	Origin         string   `json:"origin"`
	Referer        string   `json:"referer"`
	SecFetchSite   string   `json:"sec_fetch_site"`
	UseCacheHeaders bool     `json:"use_cache_headers"`
}

type Config struct {
	DefaultHeaders  map[string]string  `json:"default_headers"`
	DomainTemplates []DomainTemplate  `json:"domain_templates"`
}

var (
	config     *Config
	httpClient = &http.Client{}
	m3u8Types  = []string{
		"application/vnd.apple.mpegurl",
		"application/x-mpegurl",
		"audio/x-mpegurl",
		"audio/mpegurl",
		"video/x-mpegurl",
		"application/mpegurl",
		"application/x-hls",
		"application/x-apple-hls",
	}
	videoTypes = []string{
		"video/mp4",
		"video/webm",
		"video/ogg",
		"video/quicktime",
		"video/MP2T",
		"application/mp4",
		"video/x-m4v",
	}
	compiledPatterns = make(map[string]*regexp.Regexp)
	patternMutex     sync.RWMutex
)

func init() {
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	exPath := filepath.Dir(ex)
	possiblePaths := []string{
		"src/domains/templates.json",
		filepath.Join(exPath, "src/domains/templates.json"),
		"../src/domains/templates.json",
		"domains/templates.json",
		"templates.json",
	}

	var data []byte
	var loadErr error
	for _, path := range possiblePaths {
		data, loadErr = os.ReadFile(path)
		if loadErr == nil {
			log.Printf("Loaded templates from: %s", path)
			break
		}
	}

	if loadErr != nil {
		log.Fatalf("Error reading templates.json we are cooked %v", loadErr)
	}

	config = &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	for _, template := range config.DomainTemplates {
		for _, pattern := range template.Patterns {
			if re, err := regexp.Compile(pattern); err == nil {
				patternMutex.Lock()
				compiledPatterns[pattern] = re
				patternMutex.Unlock()
			}
		}
	}
}

func findMatchingTemplate(urlStr string) *DomainTemplate {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	hostname := parsedURL.Hostname()
	patternMutex.RLock()
	defer patternMutex.RUnlock()

	for _, template := range config.DomainTemplates {
		for _, pattern := range template.Patterns {
			if re, ok := compiledPatterns[pattern]; ok {
				if re.MatchString(hostname) {
					return &template
				}
			}
		}
	}
	return nil
}



func generateHeaders(urlStr string, template *DomainTemplate) http.Header {
	headers := make(http.Header)

	for key, value := range config.DefaultHeaders {
		headers.Set(key, value)
	}

	if template != nil {
		if template.Origin != "" {
			headers.Set("Origin", template.Origin)
		}
		if template.Referer != "" {
			headers.Set("Referer", template.Referer)
		}
		if template.SecFetchSite != "" {
			headers.Set("Sec-Fetch-Site", template.SecFetchSite)
		}
	}

	return headers
}

func isM3U8(contentType string) bool {
	contentType = strings.ToLower(contentType)
	for _, m3u8Type := range m3u8Types {
		if strings.Contains(contentType, m3u8Type) {
			return true
		}
	}
	return strings.HasSuffix(strings.ToLower(contentType), ".m3u8")
}

func processM3U8Content(content string, baseURL *url.URL, originalHeaders string) (string, error) {
	var processedLines []string
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "#EXT-X-KEY") || strings.HasPrefix(line, "#EXT-X-MAP") {
				line = processSpecialTag(line, baseURL, originalHeaders)
			}
			processedLines = append(processedLines, line)
			continue
		}

		if strings.TrimSpace(line) != "" {
			processedURL := processURL(line, baseURL, originalHeaders)
			processedLines = append(processedLines, processedURL)
		} else {
			processedLines = append(processedLines, line)
		}
	}

	return strings.Join(processedLines, "\n"), scanner.Err()
}

func processSpecialTag(line string, baseURL *url.URL, originalHeaders string) string {
	if strings.Contains(line, "URI=\"") {
		uriStart := strings.Index(line, "URI=\"") + 5
		uriEnd := strings.Index(line[uriStart:], "\"") + uriStart
		uri := line[uriStart:uriEnd]

		resolvedURL, err := baseURL.Parse(uri)
		if err != nil {
			return line
		}

		encodedURL := base64.URLEncoding.EncodeToString([]byte(resolvedURL.String()))
		newURI := fmt.Sprintf("/anime/%s", encodedURL)
		if originalHeaders != "" {
			newURI += "?headers=" + url.QueryEscape(originalHeaders)
		}

		return line[:uriStart] + newURI + line[uriEnd:]
	}
	return line
}

func processURL(urlStr string, baseURL *url.URL, originalHeaders string) string {
	if urlStr == "" {
		return urlStr
	}

	resolvedURL, err := baseURL.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	encodedURL := base64.URLEncoding.EncodeToString([]byte(resolvedURL.String()))
	newURL := fmt.Sprintf("/anime/%s", encodedURL)
	if originalHeaders != "" {
		newURL += "?headers=" + url.QueryEscape(originalHeaders)
	}

	return newURL
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	urlParam := strings.TrimPrefix(r.URL.Path, "/anime/")
	if urlParam == "" {
		http.Error(w, "Missing URL parameter", http.StatusBadRequest)
		return
	}

	decodedBytes, err := base64.URLEncoding.DecodeString(urlParam)
	if err != nil {
		http.Error(w, "Invalid base64 URL", http.StatusBadRequest)
		return
	}
	targetURL := string(decodedBytes)

	template := findMatchingTemplate(targetURL)
	headers := generateHeaders(targetURL, template)

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		headers.Set("Range", rangeHeader)
	}

	proxyReq, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}
	proxyReq.Header = headers

	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		http.Error(w, "Error fetching content", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

	for k, v := range resp.Header {
		if k == "Content-Length" || k == "Content-Type" || k == "Content-Range" || 
		   k == "Accept-Ranges" || k == "Cache-Control" || k == "Last-Modified" || 
		   k == "ETag" {
			w.Header()[k] = v
		}
	}

	contentType := resp.Header.Get("Content-Type")
	if isM3U8(contentType) {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Error reading M3U8 content", http.StatusInternalServerError)
			return
		}

		parsedURL, err := url.Parse(targetURL)
		if err != nil {
			http.Error(w, "Error parsing URL", http.StatusInternalServerError)
			return
		}

		processedContent, err := processM3U8Content(string(body), parsedURL, r.URL.Query().Get("headers"))
		if err != nil {
			http.Error(w, "Error processing M3U8 content", http.StatusInternalServerError)
			return
		}

		processedBytes := []byte(processedContent)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(processedBytes)))
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(resp.StatusCode)
		w.Write(processedBytes)
		return
	}
	w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(w, resp.Body, buf)
	if err != nil {
		log.Printf("Error streaming response: %v", err)
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/anime/", handleProxy)

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
