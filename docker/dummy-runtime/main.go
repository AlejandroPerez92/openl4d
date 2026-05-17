package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type nextEvent struct {
	RawPath               string            `json:"rawPath"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	RequestContext        struct {
		RequestID string `json:"requestId"`
	} `json:"requestContext"`
}

type responseEvent struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
	Cookies         []string          `json:"cookies"`
}

type invocationErrorEvent struct {
	ErrorMessage string   `json:"errorMessage"`
	ErrorType    string   `json:"errorType"`
	StackTrace   []string `json:"stackTrace"`
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func main() {
	processBaseURL := envOrDefault("PROCESS_BASE_URL", "http://host.docker.internal:8081")
	nextPath := envOrDefault("NEXT_PATH", "/2018-06-01/runtime/invocation/next")
	logPrefix := envOrDefault("LOG_PREFIX", "dummy-runtime")
	forceErrorOnPath := strings.TrimSpace(os.Getenv("FORCE_ERROR_ON_PATH"))
	responseQueryKey := envOrDefault("RESPONSE_QUERY_KEY", "echo")
	logEvery, err := intFromEnv("LOG_EVERY", 1000)
	if err != nil || logEvery <= 0 {
		log.Fatalf("invalid LOG_EVERY: %v", err)
	}
	blockingDuration, err := blockingDurationFromEnv()
	if err != nil {
		log.Fatalf("invalid BLOCKING_TIME_MS: %v", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	nextURL := strings.TrimRight(processBaseURL, "/") + nextPath

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   256,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 35 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stdout, logPrefix+" ", log.LstdFlags)
	logger.Printf("starting: host=%s next=%s blocking=%s responseQueryKey=%s logEvery=%d", hostname, nextURL, blockingDuration, responseQueryKey, logEvery)

	var processed uint64
	var postedSuccess uint64
	var postedErrors uint64

	for {
		select {
		case <-ctx.Done():
			logger.Printf("stopping")
			return
		default:
		}

		status, body, err := getNext(ctx, client, nextURL)
		if err != nil {
			logger.Printf("next request failed: %v", err)
			continue
		}

		switch status {
		case http.StatusOK:
			var evt nextEvent
			if err := json.Unmarshal(body, &evt); err != nil {
				logger.Printf("invalid next payload: %v", err)
				continue
			}

			requestID := strings.TrimSpace(evt.RequestContext.RequestID)
			if requestID == "" {
				logger.Printf("received 200 without requestId")
				continue
			}

			if !sleepIfNeeded(ctx, blockingDuration) {
				logger.Printf("stopping")
				return
			}

			if forceErrorOnPath != "" && strings.Contains(evt.RawPath, forceErrorOnPath) {
				err = postInvocationError(ctx, client, processBaseURL, requestID, fmt.Sprintf("forced error for path %s", evt.RawPath))
				if err == nil {
					postedErrors++
				}
			} else {
				echoValue := ""
				if evt.QueryStringParameters != nil {
					echoValue = evt.QueryStringParameters[responseQueryKey]
				}
				err = postSuccess(ctx, client, processBaseURL, requestID, evt.RawPath, responseQueryKey, echoValue)
				if err == nil {
					postedSuccess++
				}
			}
			if err != nil {
				logger.Printf("post failed for requestId=%s: %v", requestID, err)
				continue
			}

			processed++
			if processed%uint64(logEvery) == 0 {
				logger.Printf("host=%s processed=%d success=%d error=%d", hostname, processed, postedSuccess, postedErrors)
			}
		case http.StatusNoContent, 499:
			continue
		default:
			logger.Printf("next returned status %d", status)
		}
	}
}

func intFromEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func blockingDurationFromEnv() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("BLOCKING_TIME_MS"))
	if raw == "" {
		return 0, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if ms < 0 {
		return 0, errors.New("must be >= 0")
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func sleepIfNeeded(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func getNext(parent context.Context, client *http.Client, nextURL string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(parent, http.MethodGet, nextURL, nil)
	if err != nil {
		return 0, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

func postSuccess(parent context.Context, client *http.Client, processBaseURL, requestID, rawPath, echoKey, echoValue string) error {
	bodyBytes, err := json.Marshal(map[string]any{
		"ok":        true,
		"requestId": requestID,
		"path":      rawPath,
		echoKey:     echoValue,
	})
	if err != nil {
		return err
	}

	payload := responseEvent{
		StatusCode: 200,
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body:            string(bodyBytes),
		IsBase64Encoded: false,
		Cookies:         []string{},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := strings.TrimRight(processBaseURL, "/") + "/runtime/invocation/" + requestID + "/response"
	return doJSONPost(parent, client, url, b)
}

func postInvocationError(parent context.Context, client *http.Client, processBaseURL, requestID, message string) error {
	payload := invocationErrorEvent{
		ErrorMessage: message,
		ErrorType:    "DummyRuntimeError",
		StackTrace:   []string{},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := strings.TrimRight(processBaseURL, "/") + "/runtime/invocation/" + requestID + "/error"
	return doJSONPost(parent, client, url, b)
}

func doJSONPost(parent context.Context, client *http.Client, url string, body []byte) error {
	req, err := http.NewRequestWithContext(parent, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.New("unexpected status " + resp.Status)
}
