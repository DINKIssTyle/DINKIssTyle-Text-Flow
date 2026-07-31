package ai

import (
	"encoding/json"
	"testing"
)

func TestParseModelListNormalizesLMStudioMetadata(t *testing.T) {
	models, err := parseModelList([]byte(`{
		"models": [
			{
				"key": "qwen3.6-35b-a3b",
				"display_name": "Qwen3.6 35B A3B",
				"loaded_instances": [{"instance_id": "instance-1"}]
			},
			{"id": "gemma-4-e4b", "name": "Gemma 4 E4B", "state": "available"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].ID != "qwen3.6-35b-a3b" ||
		models[0].DisplayName != "Qwen3.6 35B A3B" ||
		!models[0].Loaded ||
		models[0].InstanceID != "instance-1" {
		t.Fatalf("unexpected loaded model: %#v", models[0])
	}
	if models[1].Loaded {
		t.Fatalf("available model reported as loaded: %#v", models[1])
	}
}

func TestNewModelsRequestUsesProviderEndpointAndAuthorization(t *testing.T) {
	request, err := newModelsRequest(
		ProviderLMStudio,
		"http://localhost:1234/api/v1/chat",
		"secret",
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.URL.String() != "http://localhost:1234/api/v1/models" {
		t.Fatalf("request URL = %q", request.URL)
	}
	if authorization := request.Header.Get("Authorization"); authorization != "Bearer secret" {
		t.Fatalf("authorization = %q", authorization)
	}
}

func TestNewUnloadModelRequestPostsInstanceID(t *testing.T) {
	request, err := newUnloadModelRequest(
		ProviderLMStudio,
		"http://localhost:1234",
		"",
		"instance-7",
	)
	if err != nil {
		t.Fatal(err)
	}
	if request.URL.String() != "http://localhost:1234/api/v1/models/unload" {
		t.Fatalf("request URL = %q", request.URL)
	}
	var payload map[string]string
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["instance_id"] != "instance-7" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
