package function

import (
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type HttpRequestEvent struct {
	Version               string                  `json:"version"`
	RouteKey              string                  `json:"routeKey"`
	RawPath               string                  `json:"rawPath"`
	RawQueryString        string                  `json:"rawQueryString"`
	Cookies               []string                `json:"cookies,omitempty"`
	Headers               map[string]string       `json:"headers"`
	QueryStringParameters map[string]string       `json:"queryStringParameters,omitempty"`
	PathParameters        map[string]string       `json:"pathParameters,omitempty"`
	RequestContext        HttpRequestEventContext `json:"requestContext"`
	StageVariables        map[string]string       `json:"stageVariables,omitempty"`
	Body                  string                  `json:"body,omitempty"`
	IsBase64Encoded       bool                    `json:"isBase64Encoded"`
}

type HttpRequestEventContext struct {
	RouteKey     string                                 `json:"routeKey"`
	RequestID    string                                 `json:"requestId"`
	DomainName   string                                 `json:"domainName"`
	DomainPrefix string                                 `json:"domainPrefix"`
	Time         string                                 `json:"time"`
	TimeEpoch    int64                                  `json:"timeEpoch"`
	HTTP         HttpRequestEventContextHTTPDescription `json:"http"`
}

type HttpRequestEventContextHTTPDescription struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
	SourceIP  string `json:"sourceIp"`
	UserAgent string `json:"userAgent"`
}

func FromRequest(r *http.Request) HttpRequestEvent {
	evt := HttpRequestEvent{
		Version:        "2.0",
		RouteKey:       "$default",
		RawPath:        r.URL.EscapedPath(),
		RawQueryString: r.URL.RawQuery,
	}

	// Cookies: lift Cookie header values into a separate array (AWS removes them from headers).
	if cookies := r.Cookies(); len(cookies) > 0 {
		evt.Cookies = make([]string, 0, len(cookies))
		for _, c := range cookies {
			evt.Cookies = append(evt.Cookies, c.Name+"="+c.Value)
		}
	}

	// Headers: lowercased keys, multi-values joined with ",", Cookie stripped.
	evt.Headers = make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		if strings.EqualFold(k, "Cookie") {
			continue
		}
		evt.Headers[strings.ToLower(k)] = strings.Join(vs, ",")
	}

	// Query: parsed map with multi-values joined the same way.
	if q := r.URL.Query(); len(q) > 0 {
		evt.QueryStringParameters = make(map[string]string, len(q))
		for k, vs := range q {
			evt.QueryStringParameters[k] = strings.Join(vs, ",")
		}
	}

	// Not derivable from a raw HTTP request.
	evt.PathParameters = nil
	evt.StageVariables = nil

	// Body: cap size, then decide UTF-8 vs base64.
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 6<<20)) // 6 MiB
	if err != nil {
		// No error return on this factory — surface the failure via logs.
		// See note below: consider changing the signature to return (HttpRequestEvent, error).
		log.Printf("FromRequest: body read failed: %v", err)
		body = nil
	}
	if utf8.Valid(body) {
		evt.Body = string(body)
		evt.IsBase64Encoded = false
	} else {
		evt.Body = base64.StdEncoding.EncodeToString(body)
		evt.IsBase64Encoded = true
	}

	now := time.Now().UTC()
	domainPrefix := ""
	if i := strings.IndexByte(r.Host, '.'); i > 0 {
		domainPrefix = r.Host[:i]
	}

	evt.RequestContext = HttpRequestEventContext{
		RouteKey:     "$default",
		RequestID:    uuid.NewString(),
		DomainName:   r.Host,
		DomainPrefix: domainPrefix,
		Time:         now.Format("02/Jan/2006:15:04:05 -0700"),
		TimeEpoch:    now.UnixMilli(),
		HTTP: HttpRequestEventContextHTTPDescription{
			Method:    r.Method,
			Path:      r.URL.Path,
			Protocol:  r.Proto,
			SourceIP:  clientIP(r),
			UserAgent: r.UserAgent(),
		},
	}

	return evt
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xrip := r.Header.Get("X-Real-Ip"); xrip != "" {
		return xrip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

type HttpResponseEvent struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded,omitempty"`
	Cookies           []string            `json:"cookies"`
}
