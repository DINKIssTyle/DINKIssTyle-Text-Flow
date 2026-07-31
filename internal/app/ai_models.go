package app

import "dkst-text-flow/internal/ai"

func (a *App) ListAIModels(provider, endpoint, apiKey string) ([]ai.ModelInfo, error) {
	return ai.FetchModels(provider, endpoint, apiKey)
}

func (a *App) UnloadAIModel(provider, endpoint, apiKey, instanceID string) error {
	if err := ai.UnloadModel(provider, endpoint, apiKey, instanceID); err != nil {
		return err
	}
	if a.aiHistory != nil {
		a.aiHistory.Reset()
	}
	return nil
}
