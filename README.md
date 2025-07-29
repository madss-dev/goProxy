# goProxy - HLS/M3U8 Proxy Server

A Go-based proxy server for handling HLS (HTTP Live Streaming) content, specifically designed to proxy m3u8 playlists and ts segments. The proxy handles CORS, header management, and URL rewriting for seamless streaming.

## Features

- Proxies M3U8 playlists and video segments
- Automatic URL rewriting in M3U8 files
- CORS support
- Custom header management based on domain templates
- Support for relative and absolute URLs in playlists
- Range request support for video segments
- Base64 URL and header encoding for security

## Setup

1. Install Go (1.21 or later)
2. Clone the repository
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Run the server:
   ```bash
   go run src/main.go
   ```

The server will start on `http://localhost:8080`.

## API Usage

### Proxy Endpoint

```
GET /proxy?url=<base64_encoded_url>&headers=<base64_encoded_headers>
```

Parameters:
- `url`: Base64 encoded URL of the m3u8 or video segment
- `headers` (optional): Base64 encoded JSON string of headers to send with the request

Example:
```javascript
// Encode URL and headers
const url = 'https://example.com/video.m3u8';
const headers = {
    'Referer': 'https://example.com',
    'Origin': 'https://example.com'
};

const encodedUrl = btoa(url);
const encodedHeaders = btoa(JSON.stringify(headers));

// Make request
fetch(`http://localhost:8080/proxy?url=${encodeURIComponent(encodedUrl)}&headers=${encodeURIComponent(encodedHeaders)}`);
```

## Domain Templates

The proxy uses domain templates from `templates.json` to automatically set appropriate headers for different domains. Templates include:
- Origin
- Referer
- Sec-Fetch-Site
- Cache header settings

## Content Types

Supported content types:
- M3U8 playlists
- Video segments (MP4, TS, etc.)
- Other streaming formats

## Error Handling

The proxy returns appropriate HTTP status codes and error messages:
- 400: Bad Request (invalid URL or headers)
- 403: Forbidden (CORS violation)
- 500: Internal Server Error (proxy errors)

## Security Considerations

- URLs and headers are base64 encoded to prevent injection
- CORS is enabled by default
- Headers are validated before forwarding
- Error messages are sanitized

## License

MIT License 