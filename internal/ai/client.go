package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu       sync.Mutex
	activeID int64
	cancel   context.CancelFunc
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) MakeRequest(endpoint string, headers map[string]string, body string) (string, error) {
	ctx, cancel, requestID := c.beginRequest()
	defer cancel()
	defer c.finishRequest(requestID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "", context.Canceled
		}
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(respBody), fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

func (c *Client) Cancel() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.activeID++
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (c *Client) beginRequest() (context.Context, context.CancelFunc, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.activeID++
	c.cancel = cancel
	return ctx, cancel, c.activeID
}

func (c *Client) finishRequest(requestID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeID == requestID {
		c.cancel = nil
	}
}
