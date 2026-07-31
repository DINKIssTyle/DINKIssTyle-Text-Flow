package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxModelsResponseBytes = 8 * 1024 * 1024

// ModelInfo is the normalized model metadata used by the settings model picker.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Loaded      bool   `json:"loaded"`
	InstanceID  string `json:"instanceId,omitempty"`
}

type rawModelList struct {
	Data   []rawModel `json:"data"`
	Models []rawModel `json:"models"`
}

type rawModel struct {
	ID              string            `json:"id"`
	Key             string            `json:"key"`
	Name            string            `json:"name"`
	DisplayName     string            `json:"display_name"`
	DisplayNameAlt  string            `json:"displayName"`
	Loaded          bool              `json:"loaded"`
	IsLoaded        bool              `json:"is_loaded"`
	IsLoadedAlt     bool              `json:"isLoaded"`
	Active          bool              `json:"active"`
	CurrentlyLoaded bool              `json:"currently_loaded"`
	State           string            `json:"state"`
	Status          string            `json:"status"`
	LoadState       string            `json:"load_state"`
	LoadedInstances []json.RawMessage `json:"loaded_instances"`
}

func modelAPIBase(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	for _, suffix := range []string{
		"/api/v1/models/unload",
		"/api/v1/models/load",
		"/api/v1/models",
		"/api/v1/chat",
		"/api/v1",
		"/v1/models/unload",
		"/v1/models/load",
		"/v1/models",
		"/v1/chat/completions",
		"/chat/completions",
		"/v1",
	} {
		if strings.HasSuffix(endpoint, suffix) {
			return strings.TrimSuffix(endpoint, suffix)
		}
	}
	return endpoint
}

func ModelsEndpoint(provider, endpoint string) string {
	base := modelAPIBase(endpoint)
	if strings.EqualFold(strings.TrimSpace(provider), ProviderLMStudio) {
		return base + "/api/v1/models"
	}
	return base + "/v1/models"
}

func ModelUnloadEndpoint(provider, endpoint string) string {
	base := modelAPIBase(endpoint)
	if strings.EqualFold(strings.TrimSpace(provider), ProviderLMStudio) {
		return base + "/api/v1/models/unload"
	}
	return base + "/v1/models/unload"
}

func FetchModels(provider, endpoint, apiKey string) ([]ModelInfo, error) {
	if strings.EqualFold(strings.TrimSpace(provider), ProviderAppleIntelligence) {
		return nil, errors.New("Apple Intelligence does not expose a model list")
	}
	req, err := newModelsRequest(provider, endpoint, apiKey)
	if err != nil {
		return nil, err
	}

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch model list: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelsResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read model list: %w", err)
	}
	if len(body) > maxModelsResponseBytes {
		return nil, errors.New("model list response is too large")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("model server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	models, err := parseModelList(body)
	if err != nil {
		return nil, fmt.Errorf("parse model list: %w", err)
	}
	return models, nil
}

func UnloadModel(provider, endpoint, apiKey, instanceID string) error {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return errors.New("model instance ID is required")
	}
	req, err := newUnloadModelRequest(provider, endpoint, apiKey, instanceID)
	if err != nil {
		return err
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("unload model: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelsResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read model unload response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("model server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func newModelsRequest(provider, endpoint, apiKey string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, ModelsEndpoint(provider, endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("create model list request: %w", err)
	}
	setModelAPIHeaders(req, provider, apiKey)
	return req, nil
}

func newUnloadModelRequest(
	provider, endpoint, apiKey, instanceID string,
) (*http.Request, error) {
	payload, err := json.Marshal(map[string]string{"instance_id": instanceID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(
		http.MethodPost,
		ModelUnloadEndpoint(provider, endpoint),
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create model unload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setModelAPIHeaders(req, provider, apiKey)
	return req, nil
}

func setModelAPIHeaders(req *http.Request, provider, apiKey string) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return
	}
	if strings.EqualFold(strings.TrimSpace(provider), ProviderLMStudio) {
		req.Header.Set("Authorization", "Bearer lm-studio")
	}
}

func parseModelList(body []byte) ([]ModelInfo, error) {
	var list rawModelList
	if err := json.Unmarshal(body, &list); err != nil {
		var direct []rawModel
		if directErr := json.Unmarshal(body, &direct); directErr != nil {
			return nil, err
		}
		list.Data = direct
	}
	models := list.Data
	if len(models) == 0 {
		models = list.Models
	}
	result := make([]ModelInfo, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		id := firstNonEmpty(model.ID, model.Key, model.Name)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		displayName := firstNonEmpty(model.DisplayName, model.DisplayNameAlt, model.Name, id)
		instanceID := firstLoadedInstanceID(model.LoadedInstances)
		state := strings.ToLower(firstNonEmpty(model.State, model.Status, model.LoadState))
		loaded := instanceID != "" ||
			model.Loaded ||
			model.IsLoaded ||
			model.IsLoadedAlt ||
			model.Active ||
			model.CurrentlyLoaded ||
			state == "loaded" ||
			state == "active" ||
			state == "ready" ||
			state == "resident"
		result = append(result, ModelInfo{
			ID:          id,
			DisplayName: displayName,
			Loaded:      loaded,
			InstanceID:  instanceID,
		})
	}
	return result, nil
}

func firstLoadedInstanceID(instances []json.RawMessage) string {
	for _, raw := range instances {
		var stringID string
		if err := json.Unmarshal(raw, &stringID); err == nil {
			if stringID = strings.TrimSpace(stringID); stringID != "" {
				return stringID
			}
		}
		var instance struct {
			InstanceID string `json:"instance_id"`
			ID         string `json:"id"`
			Key        string `json:"key"`
		}
		if err := json.Unmarshal(raw, &instance); err == nil {
			if id := firstNonEmpty(instance.InstanceID, instance.ID, instance.Key); id != "" {
				return id
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
