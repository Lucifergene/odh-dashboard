package models

import (
	"encoding/json"
	"log/slog"
)

// ExternalVectorDBConfig represents a single external vector database provider
// parsed from the gen-ai-external-vector-dbs ConfigMap.
// Each key in the ConfigMap becomes the Name (provider_id), and the value is
// a JSON object containing provider_type, config, vector_store_id, embedding_model,
// and embedding_dimension.
type ExternalVectorDBConfig struct {
	Name               string                 `json:"name"`
	ProviderType       string                 `json:"provider_type"`
	VectorStoreID      string                 `json:"vector_store_id"`
	Config             map[string]interface{} `json:"config"`
	EmbeddingModel     string                 `json:"embedding_model"`
	EmbeddingDimension int                    `json:"embedding_dimension"`
}

// ExternalVectorDBsListData is the response data for the GET external-vector-dbs endpoint.
type ExternalVectorDBsListData struct {
	Databases              []ExternalVectorDBConfig `json:"databases"`
	CurrentDefaultProvider string                   `json:"current_default_provider"`
	CurrentVectorStoreID   string                   `json:"current_vector_store_id"`
}

// ParseExternalVectorDBsFromConfigMapData parses external vector database configurations
// from a ConfigMap's Data field. Each key becomes the provider name, and each value
// is expected to be a JSON object with provider_type, vector_store_id, config, and
// optional embedding_model and embedding_dimension fields.
//
// Entries with invalid JSON or missing required fields (provider_type, vector_store_id) are skipped.
// Default embedding values are applied when not specified.
// If logger is provided, warnings are logged for skipped entries.
func ParseExternalVectorDBsFromConfigMapData(
	data map[string]string,
	defaultEmbeddingModel string,
	defaultEmbeddingDimension int,
	logger *slog.Logger,
) []ExternalVectorDBConfig {
	databases := make([]ExternalVectorDBConfig, 0, len(data))

	for providerID, configJSON := range data {
		var parsed struct {
			ProviderType       string                 `json:"provider_type"`
			VectorStoreID      string                 `json:"vector_store_id"`
			Config             map[string]interface{} `json:"config"`
			EmbeddingModel     string                 `json:"embedding_model"`
			EmbeddingDimension int                    `json:"embedding_dimension"`
		}
		if err := json.Unmarshal([]byte(configJSON), &parsed); err != nil {
			if logger != nil {
				logger.Warn("failed to parse external vector DB entry, skipping", "providerID", providerID, "error", err)
			}
			continue
		}

		// Validate required fields
		if parsed.ProviderType == "" {
			if logger != nil {
				logger.Warn("external vector DB entry missing provider_type, skipping", "providerID", providerID)
			}
			continue
		}
		if parsed.VectorStoreID == "" {
			if logger != nil {
				logger.Warn("external vector DB entry missing vector_store_id, skipping", "providerID", providerID)
			}
			continue
		}

		// Backward-compatible defaults for embedding metadata
		embeddingModel := parsed.EmbeddingModel
		if embeddingModel == "" {
			embeddingModel = defaultEmbeddingModel
		}
		embeddingDimension := parsed.EmbeddingDimension
		if embeddingDimension == 0 {
			embeddingDimension = defaultEmbeddingDimension
		}

		databases = append(databases, ExternalVectorDBConfig{
			Name:               providerID,
			ProviderType:       parsed.ProviderType,
			VectorStoreID:      parsed.VectorStoreID,
			Config:             parsed.Config,
			EmbeddingModel:     embeddingModel,
			EmbeddingDimension: embeddingDimension,
		})
	}

	return databases
}
