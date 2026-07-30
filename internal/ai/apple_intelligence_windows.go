//go:build windows

package ai

import "errors"

type unavailableAppleIntelligenceClient struct{}

func NewAppleIntelligenceClient() AppleIntelligenceClient {
	return &unavailableAppleIntelligenceClient{}
}

func (c *unavailableAppleIntelligenceClient) Generate(string, string) (string, error) {
	return "", errors.New("Apple Intelligence is only available on supported macOS devices")
}

func (c *unavailableAppleIntelligenceClient) Status() AppleIntelligenceStatus {
	return AppleIntelligenceStatus{
		Available: false,
		State:     AppleIntelligenceStateOSUnsupported,
	}
}

func (c *unavailableAppleIntelligenceClient) Cancel() {}
