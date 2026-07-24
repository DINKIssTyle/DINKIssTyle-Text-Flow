//go:build darwin

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const appleIntelligenceHelperName = "DKSTAppleIntelligenceHelper"

type appleIntelligenceProcessClient struct {
	mu       sync.Mutex
	activeID int64
	cancel   context.CancelFunc
}

type appleIntelligenceHelperRequest struct {
	Mode         string `json:"mode"`
	Instructions string `json:"instructions,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
}

type appleIntelligenceHelperResponse struct {
	Available bool   `json:"available"`
	State     string `json:"state"`
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

func NewAppleIntelligenceClient() AppleIntelligenceClient {
	return &appleIntelligenceProcessClient{}
}

func (c *appleIntelligenceProcessClient) Generate(instructions string, prompt string) (string, error) {
	response, err := c.invoke(appleIntelligenceHelperRequest{
		Mode:         "generate",
		Instructions: instructions,
		Prompt:       prompt,
	})
	if err != nil {
		return "", err
	}
	if !response.Available {
		return "", appleIntelligenceAvailabilityError(response)
	}
	if detail := strings.TrimSpace(response.Error); detail != "" {
		return "", errors.New(detail)
	}
	content := strings.TrimSpace(response.Content)
	if content == "" {
		return "", errors.New("Apple Intelligence returned an empty response")
	}
	return content, nil
}

func (c *appleIntelligenceProcessClient) Status() AppleIntelligenceStatus {
	response, err := c.invokeUntracked(appleIntelligenceHelperRequest{Mode: "availability"})
	if err != nil {
		state := AppleIntelligenceStateUnavailable
		if errors.Is(err, os.ErrNotExist) {
			state = AppleIntelligenceStateHelperUnavailable
		}
		return AppleIntelligenceStatus{
			Available: false,
			State:     state,
			Detail:    err.Error(),
		}
	}
	return AppleIntelligenceStatus{
		Available: response.Available,
		State:     normalizeAppleIntelligenceState(response.State),
		Detail:    strings.TrimSpace(response.Error),
	}
}

func (c *appleIntelligenceProcessClient) Cancel() {
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.activeID++
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *appleIntelligenceProcessClient) invoke(request appleIntelligenceHelperRequest) (appleIntelligenceHelperResponse, error) {
	ctx, cancel, requestID := c.beginRequest()
	defer c.finishRequest(requestID, cancel)
	return invokeAppleIntelligenceHelper(ctx, request)
}

func (c *appleIntelligenceProcessClient) invokeUntracked(request appleIntelligenceHelperRequest) (appleIntelligenceHelperResponse, error) {
	return invokeAppleIntelligenceHelper(context.Background(), request)
}

func (c *appleIntelligenceProcessClient) beginRequest() (context.Context, context.CancelFunc, int64) {
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

func (c *appleIntelligenceProcessClient) finishRequest(requestID int64, cancel context.CancelFunc) {
	cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activeID == requestID {
		c.cancel = nil
	}
}

func invokeAppleIntelligenceHelper(ctx context.Context, request appleIntelligenceHelperRequest) (appleIntelligenceHelperResponse, error) {
	helperPath, err := findAppleIntelligenceHelper()
	if err != nil {
		return appleIntelligenceHelperResponse{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return appleIntelligenceHelperResponse{}, err
	}

	cmd := exec.CommandContext(ctx, helperPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return appleIntelligenceHelperResponse{}, context.Canceled
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return appleIntelligenceHelperResponse{}, fmt.Errorf("Apple Intelligence helper failed: %s", detail)
	}

	var response appleIntelligenceHelperResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return appleIntelligenceHelperResponse{}, fmt.Errorf("invalid Apple Intelligence helper response: %w", err)
	}
	response.State = normalizeAppleIntelligenceState(response.State)
	return response, nil
}

func findAppleIntelligenceHelper() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("DKST_APPLE_INTELLIGENCE_HELPER")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("%w: Apple Intelligence helper at %s", os.ErrNotExist, configured)
	}

	var candidates []string
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), appleIntelligenceHelperName))
	}
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(workingDirectory, "bin", appleIntelligenceHelperName),
			filepath.Join(workingDirectory, appleIntelligenceHelperName),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: %s is not bundled with the app", os.ErrNotExist, appleIntelligenceHelperName)
}

func appleIntelligenceAvailabilityError(response appleIntelligenceHelperResponse) error {
	detail := strings.TrimSpace(response.Error)
	if detail == "" {
		detail = "Apple Intelligence is unavailable"
	}
	return fmt.Errorf("%s (%s)", detail, normalizeAppleIntelligenceState(response.State))
}

func normalizeAppleIntelligenceState(state string) string {
	switch strings.TrimSpace(state) {
	case AppleIntelligenceStateAvailable,
		AppleIntelligenceStateDeviceNotEligible,
		AppleIntelligenceStateNotEnabled,
		AppleIntelligenceStateModelNotReady,
		AppleIntelligenceStateOSUnsupported,
		AppleIntelligenceStateSDKUnavailable,
		AppleIntelligenceStateHelperUnavailable,
		AppleIntelligenceStateUnavailable,
		AppleIntelligenceStateChecking:
		return strings.TrimSpace(state)
	default:
		return AppleIntelligenceStateUnavailable
	}
}
