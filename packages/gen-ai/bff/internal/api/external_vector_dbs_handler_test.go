package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	lsdapi "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/opendatahub-io/gen-ai/internal/config"
	"github.com/opendatahub-io/gen-ai/internal/constants"
	"github.com/opendatahub-io/gen-ai/internal/integrations"
	k8s "github.com/opendatahub-io/gen-ai/internal/integrations/kubernetes"
	"github.com/opendatahub-io/gen-ai/internal/integrations/kubernetes/k8smocks"
	"github.com/opendatahub-io/gen-ai/internal/integrations/mcp/mcpmocks"
	"github.com/opendatahub-io/gen-ai/internal/models"
	"github.com/opendatahub-io/gen-ai/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testExternalVectorDBNamespace = "dora-namespace"

func setupExternalVectorDBTestApp(t *testing.T) *App {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mockMCPFactory := mcpmocks.NewMockedMCPClientFactory(
		config.EnvConfig{MockK8sClient: true},
		logger,
	)

	mockK8sFactory, err := k8smocks.NewTokenClientFactory(testK8sClient, testCfg, logger)
	require.NoError(t, err)

	return &App{
		config: config.EnvConfig{
			Port:       4000,
			AuthMethod: "user_token",
		},
		logger:                  logger,
		repositories:            repositories.NewRepositoriesWithMCP(mockMCPFactory, logger),
		kubernetesClientFactory: mockK8sFactory,
		mcpClientFactory:        mockMCPFactory,
		dashboardNamespace:      "opendatahub",
	}
}

func TestExternalVectorDBsListHandler(t *testing.T) {
	t.Run("should return 400 when namespace is missing from context", func(t *testing.T) {
		app := setupExternalVectorDBTestApp(t)
		rr := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/gen-ai/api/v1/external-vector-dbs", nil)
		require.NoError(t, err)

		reqCtx := context.WithValue(req.Context(), constants.RequestIdentityKey, &integrations.RequestIdentity{
			Token: "FAKE_BEARER_TOKEN",
		})
		req = req.WithContext(reqCtx)

		app.ExternalVectorDBsListHandler(rr, req, nil)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("should return 401 when identity is missing from context", func(t *testing.T) {
		app := setupExternalVectorDBTestApp(t)
		rr := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/gen-ai/api/v1/external-vector-dbs", nil)
		require.NoError(t, err)

		reqCtx := context.WithValue(req.Context(), constants.NamespaceQueryParameterKey, testExternalVectorDBNamespace)
		req = req.WithContext(reqCtx)

		app.ExternalVectorDBsListHandler(rr, req, nil)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("should return 404 when ConfigMap does not exist", func(t *testing.T) {
		app := setupExternalVectorDBTestAppWithClient(t, &configMapMockClient{
			configMapErr: fmt.Errorf("configmaps %q not found", constants.ExternalVectorDBsConfigMapName),
		})

		rr := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/gen-ai/api/v1/external-vector-dbs", nil)
		require.NoError(t, err)

		reqCtx := context.WithValue(req.Context(), constants.NamespaceQueryParameterKey, "bella-namespace")
		reqCtx = context.WithValue(reqCtx, constants.RequestIdentityKey, &integrations.RequestIdentity{
			Token: "FAKE_BEARER_TOKEN",
		})
		req = req.WithContext(reqCtx)

		app.ExternalVectorDBsListHandler(rr, req, nil)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("should return list of external vector databases", func(t *testing.T) {
		app := setupExternalVectorDBTestAppWithClient(t, &configMapMockClient{
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.ExternalVectorDBsConfigMapName,
					Namespace: testExternalVectorDBNamespace,
				},
				Data: map[string]string{
					"milvus-remote": `{"provider_type": "remote::milvus", "vector_store_id": "vs_milvus_products", "config": {"url": "http://milvus.example.com:19530"}}`,
					"qdrant-prod":   `{"provider_type": "remote::qdrant", "vector_store_id": "vs_qdrant_support", "config": {"api_key": "test-key", "url": "http://qdrant.example.com:6334"}}`,
				},
			},
		})

		rr := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/gen-ai/api/v1/external-vector-dbs", nil)
		require.NoError(t, err)

		reqCtx := context.WithValue(req.Context(), constants.NamespaceQueryParameterKey, testExternalVectorDBNamespace)
		reqCtx = context.WithValue(reqCtx, constants.RequestIdentityKey, &integrations.RequestIdentity{
			Token: "FAKE_BEARER_TOKEN",
		})
		req = req.WithContext(reqCtx)

		app.ExternalVectorDBsListHandler(rr, req, nil)
		assert.Equal(t, http.StatusOK, rr.Code)

		body, err := io.ReadAll(rr.Result().Body)
		require.NoError(t, err)
		defer rr.Result().Body.Close()

		var response ExternalVectorDBsListEnvelope
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)

		assert.Len(t, response.Data.Databases, 2)

		dbByName := make(map[string]models.ExternalVectorDBConfig)
		for _, db := range response.Data.Databases {
			dbByName[db.Name] = db
			assert.NotEmpty(t, db.ProviderType)
			assert.NotEmpty(t, db.VectorStoreID)
			assert.NotNil(t, db.Config)
		}
		assert.Contains(t, dbByName, "milvus-remote")
		assert.Contains(t, dbByName, "qdrant-prod")
		assert.Equal(t, "vs_milvus_products", dbByName["milvus-remote"].VectorStoreID)
		assert.Equal(t, "vs_qdrant_support", dbByName["qdrant-prod"].VectorStoreID)
	})
}

// configMapMockClient is a minimal mock that wraps TokenKubernetesClientMock
// but overrides GetConfigMap and GetLlamaStackDistributions to return test-specific data.
type configMapMockClient struct {
	k8smocks.TokenKubernetesClientMock
	configMap    *corev1.ConfigMap
	configMapErr error
}

func (m *configMapMockClient) GetConfigMap(_ context.Context, _ *integrations.RequestIdentity, _, _ string) (*corev1.ConfigMap, error) {
	if m.configMapErr != nil {
		return nil, m.configMapErr
	}
	return m.configMap, nil
}

func (m *configMapMockClient) GetLlamaStackDistributions(_ context.Context, _ *integrations.RequestIdentity, _ string) (*lsdapi.LlamaStackDistributionList, error) {
	return &lsdapi.LlamaStackDistributionList{Items: []lsdapi.LlamaStackDistribution{}}, nil
}

// configMapMockFactory is a factory that returns a configMapMockClient.
type configMapMockFactory struct {
	client *configMapMockClient
}

func (f *configMapMockFactory) GetClient(_ context.Context) (k8s.KubernetesClientInterface, error) {
	return f.client, nil
}

func (f *configMapMockFactory) ExtractRequestIdentity(_ http.Header) (*integrations.RequestIdentity, error) {
	return &integrations.RequestIdentity{Token: "FAKE_BEARER_TOKEN"}, nil
}

func (f *configMapMockFactory) ValidateRequestIdentity(identity *integrations.RequestIdentity) error {
	if identity == nil || identity.Token == "" {
		return fmt.Errorf("token is required")
	}
	return nil
}

func setupExternalVectorDBTestAppWithClient(t *testing.T, mockClient *configMapMockClient) *App {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mockMCPFactory := mcpmocks.NewMockedMCPClientFactory(
		config.EnvConfig{MockK8sClient: true},
		logger,
	)
	return &App{
		config: config.EnvConfig{
			Port:       4000,
			AuthMethod: "user_token",
		},
		logger:                  logger,
		repositories:            repositories.NewRepositoriesWithMCP(mockMCPFactory, logger),
		kubernetesClientFactory: &configMapMockFactory{client: mockClient},
		mcpClientFactory:        mockMCPFactory,
		dashboardNamespace:      "opendatahub",
	}
}

func TestParseExternalVectorDBsConfigMap_EmbeddingFields(t *testing.T) {
	identity := &integrations.RequestIdentity{Token: "FAKE"}

	t.Run("should parse embedding_model and embedding_dimension from ConfigMap entries", func(t *testing.T) {
		mockClient := &configMapMockClient{
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.ExternalVectorDBsConfigMapName, Namespace: "test-ns"},
				Data: map[string]string{
					"milvus-ext": `{"provider_type": "remote::milvus", "vector_store_id": "vs_001", "config": {"url": "http://milvus:19530"}, "embedding_model": "custom-model", "embedding_dimension": 512}`,
				},
			},
		}

		dbs, err := parseExternalVectorDBsConfigMap(mockClient, context.Background(), identity, "test-ns", nil)
		require.NoError(t, err)
		require.Len(t, dbs, 1)
		assert.Equal(t, "custom-model", dbs[0].EmbeddingModel)
		assert.Equal(t, 512, dbs[0].EmbeddingDimension)
	})

	t.Run("should use defaults when embedding fields are missing", func(t *testing.T) {
		mockClient := &configMapMockClient{
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.ExternalVectorDBsConfigMapName, Namespace: "test-ns"},
				Data: map[string]string{
					"qdrant-ext": `{"provider_type": "remote::qdrant", "vector_store_id": "vs_002", "config": {"url": "http://qdrant:6334"}}`,
				},
			},
		}

		dbs, err := parseExternalVectorDBsConfigMap(mockClient, context.Background(), identity, "test-ns", nil)
		require.NoError(t, err)
		require.Len(t, dbs, 1)

		defaultEmbed := constants.DefaultEmbeddingModel()
		assert.Equal(t, defaultEmbed.ModelID, dbs[0].EmbeddingModel)
		assert.Equal(t, int(defaultEmbed.EmbeddingDimension), dbs[0].EmbeddingDimension)
	})

	t.Run("should skip entries with empty vector_store_id", func(t *testing.T) {
		mockClient := &configMapMockClient{
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.ExternalVectorDBsConfigMapName, Namespace: "test-ns"},
				Data: map[string]string{
					"valid":   `{"provider_type": "remote::milvus", "vector_store_id": "vs_valid", "config": {}}`,
					"invalid": `{"provider_type": "remote::milvus", "vector_store_id": "", "config": {}}`,
					"broken":  `not valid json`,
				},
			},
		}

		dbs, err := parseExternalVectorDBsConfigMap(mockClient, context.Background(), identity, "test-ns", nil)
		require.NoError(t, err)
		require.Len(t, dbs, 1)
		assert.Equal(t, "vs_valid", dbs[0].VectorStoreID)
	})

	t.Run("should skip entries with empty provider_type", func(t *testing.T) {
		mockClient := &configMapMockClient{
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: constants.ExternalVectorDBsConfigMapName, Namespace: "test-ns"},
				Data: map[string]string{
					"valid":         `{"provider_type": "remote::qdrant", "vector_store_id": "vs_valid", "config": {}}`,
					"missing-type":  `{"provider_type": "", "vector_store_id": "vs_no_type", "config": {}}`,
					"no-type-field": `{"vector_store_id": "vs_no_field", "config": {}}`,
				},
			},
		}

		dbs, err := parseExternalVectorDBsConfigMap(mockClient, context.Background(), identity, "test-ns", nil)
		require.NoError(t, err)
		require.Len(t, dbs, 1)
		assert.Equal(t, "vs_valid", dbs[0].VectorStoreID)
		assert.Equal(t, "remote::qdrant", dbs[0].ProviderType)
	})
}
