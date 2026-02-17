package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/opendatahub-io/gen-ai/internal/constants"
	"github.com/opendatahub-io/gen-ai/internal/integrations"
	k8s "github.com/opendatahub-io/gen-ai/internal/integrations/kubernetes"
	"github.com/opendatahub-io/gen-ai/internal/models"
)

// ExternalVectorDBsListEnvelope is the response envelope for listing external vector databases.
type ExternalVectorDBsListEnvelope = Envelope[models.ExternalVectorDBsListData, None]

// ExternalVectorDBsListHandler handles GET /gen-ai/api/v1/external-vector-dbs?namespace=X
// Reads external vector database configurations from the gen-ai-external-vector-dbs ConfigMap
// and resolves the current default_provider_id from the LlamaStack config in the same namespace.
func (app *App) ExternalVectorDBsListHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	namespace, ok := ctx.Value(constants.NamespaceQueryParameterKey).(string)
	if !ok || namespace == "" {
		app.badRequestResponse(w, r, fmt.Errorf("missing namespace in the context"))
		return
	}

	identity, ok := ctx.Value(constants.RequestIdentityKey).(*integrations.RequestIdentity)
	if !ok || identity == nil {
		app.unauthorizedResponse(w, r, fmt.Errorf("missing RequestIdentity in context"))
		return
	}

	k8sClient, err := app.kubernetesClientFactory.GetClient(ctx)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Read external vector DB ConfigMap from project namespace
	databases, err := parseExternalVectorDBsConfigMap(k8sClient, ctx, identity, namespace, app.logger)
	if err != nil {
		app.handleConfigMapError(w, r, err, constants.ExternalVectorDBsConfigMapName, namespace)
		return
	}

	// Resolve current default_provider_id from LlamaStack config (non-fatal if it fails)
	currentDefault := resolveCurrentDefaultProvider(k8sClient, ctx, identity, namespace)

	// Look up the vector_store_id of the current default provider
	currentVectorStoreID := ""
	if currentDefault != "" {
		for _, db := range databases {
			if db.Name == currentDefault {
				currentVectorStoreID = db.VectorStoreID
				break
			}
		}
	}

	responseData := models.ExternalVectorDBsListData{
		Databases:              databases,
		CurrentDefaultProvider: currentDefault,
		CurrentVectorStoreID:   currentVectorStoreID,
	}

	response := ExternalVectorDBsListEnvelope{
		Data: responseData,
	}

	if err := app.WriteJSON(w, http.StatusOK, response, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// parseExternalVectorDBsConfigMap reads the external vector DBs ConfigMap and parses each entry.
// Each entry must include provider_type, vector_store_id, and config.
// embedding_model and embedding_dimension are optional with backward-compatible defaults.
func parseExternalVectorDBsConfigMap(
	k8sClient k8s.KubernetesClientInterface,
	ctx context.Context,
	identity *integrations.RequestIdentity,
	namespace string,
	logger *slog.Logger,
) ([]models.ExternalVectorDBConfig, error) {
	configMap, err := k8sClient.GetConfigMap(ctx, identity, namespace, constants.ExternalVectorDBsConfigMapName)
	if err != nil {
		return nil, err
	}

	defaultEmbedding := constants.DefaultEmbeddingModel()
	databases := models.ParseExternalVectorDBsFromConfigMapData(
		configMap.Data,
		defaultEmbedding.ModelID,
		int(defaultEmbedding.EmbeddingDimension),
		logger,
	)

	return databases, nil
}

// resolveCurrentDefaultProvider reads the LlamaStack config to get the current default vector provider.
// Returns empty string if resolution fails (non-fatal).
func resolveCurrentDefaultProvider(
	k8sClient k8s.KubernetesClientInterface,
	ctx context.Context,
	identity *integrations.RequestIdentity,
	namespace string,
) string {
	lsdList, err := k8sClient.GetLlamaStackDistributions(ctx, identity, namespace)
	if err != nil || len(lsdList.Items) == 0 {
		return ""
	}

	lsd := lsdList.Items[0]
	configMapName := constants.LlamaStackConfigMapName
	if lsd.Spec.Server.UserConfig != nil && lsd.Spec.Server.UserConfig.ConfigMapName != "" {
		configMapName = lsd.Spec.Server.UserConfig.ConfigMapName
	}

	configMap, err := k8sClient.GetConfigMap(ctx, identity, namespace, configMapName)
	if err != nil {
		return ""
	}

	configYAML, exists := configMap.Data[constants.LlamaStackConfigYAMLKey]
	if !exists {
		return ""
	}

	var config k8s.LlamaStackConfig
	if err := config.FromYAML(configYAML); err != nil {
		return ""
	}

	return config.VectorStores.DefaultProviderID
}
