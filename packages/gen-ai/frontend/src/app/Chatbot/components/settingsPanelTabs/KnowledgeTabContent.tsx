import * as React from 'react';
import {
  Form,
  FormGroup,
  Switch,
  AlertGroup,
  Alert,
  AlertVariant,
  Spinner,
  Dropdown,
  DropdownItem,
  DropdownList,
  MenuToggle,
  MenuToggleElement,
  Divider,
  Content,
  ContentVariants,
} from '@patternfly/react-core';
import { fireMiscTrackingEvent } from '@odh-dashboard/internal/concepts/analyticsTracking/segmentIOUtils';
import { ChatbotSourceUploadPanel } from '~/app/Chatbot/sourceUpload/ChatbotSourceUploadPanel';
import { UseSourceManagementReturn } from '~/app/Chatbot/hooks/useSourceManagement';
import { UseFileManagementReturn } from '~/app/Chatbot/hooks/useFileManagement';
import TabContentWrapper from '~/app/Chatbot/components/settingsPanelTabs/TabContentWrapper';
import UploadedFilesList from '~/app/Chatbot/components/UploadedFilesList';
import { useChatbotConfigStore, selectRagEnabled, DEFAULT_CONFIG_ID } from '~/app/Chatbot/store';
import { ExternalVectorDB } from '~/app/types';
import useFetchExternalVectorDBs from '~/app/hooks/useFetchExternalVectorDBs';

const INLINE_PROVIDER_VALUE = '__inline__';

interface KnowledgeTabContentProps {
  configId?: string;
  sourceManagement: UseSourceManagementReturn;
  fileManagement: UseFileManagementReturn;
  alerts: {
    uploadSuccessAlert: React.ReactElement | undefined;
    deleteSuccessAlert: React.ReactElement | undefined;
    errorAlert: React.ReactElement | undefined;
  };
}

const KnowledgeTabContent: React.FunctionComponent<KnowledgeTabContentProps> = ({
  configId = DEFAULT_CONFIG_ID,
  sourceManagement,
  fileManagement,
  alerts,
}) => {
  const isRagEnabled = useChatbotConfigStore(selectRagEnabled(configId));
  const updateRagEnabled = useChatbotConfigStore((state) => state.updateRagEnabled);
  const updateExternalVectorStoreId = useChatbotConfigStore(
    (state) => state.updateExternalVectorStoreId,
  );

  const { data: externalDBs, loaded: dbsLoaded, error: dbsError } = useFetchExternalVectorDBs();

  // Dropdown starts unselected — user must explicitly choose
  const [selectedProvider, setSelectedProvider] = React.useState<string>('');
  const [isDropdownOpen, setIsDropdownOpen] = React.useState(false);

  // When user selects a provider in the dropdown, update the store immediately (no API call)
  const handleProviderSelect = React.useCallback(
    (_event: unknown, value: string | number | undefined) => {
      if (typeof value !== 'string') {
        return;
      }
      setSelectedProvider(value);
      setIsDropdownOpen(false);

      if (value === INLINE_PROVIDER_VALUE) {
        updateExternalVectorStoreId(null);
      } else {
        const selectedDB = externalDBs.find((db: ExternalVectorDB) => db.name === value);
        if (selectedDB?.vector_store_id) {
          updateExternalVectorStoreId(selectedDB.vector_store_id);
        }
      }
    },
    [externalDBs, updateExternalVectorStoreId],
  );

  const selectedLabel =
    selectedProvider === INLINE_PROVIDER_VALUE
      ? 'Inline'
      : externalDBs.find((db) => db.name === selectedProvider)?.name || '';

  const headerActions = (
    <Switch
      id="rag-toggle-switch"
      isChecked={isRagEnabled}
      data-testid="rag-toggle-switch"
      onChange={(_, checked) => {
        updateRagEnabled(configId, checked);
        fireMiscTrackingEvent('Playground RAG Toggle Selected', {
          isRag: checked,
        });
      }}
      aria-label="Toggle RAG mode"
    />
  );

  return (
    <TabContentWrapper
      title="Knowledge"
      headerActions={headerActions}
      titleTestId="rag-section-title"
    >
      <Form>
        {/* External Vector Database Section */}
        <FormGroup fieldId="external-vector-db" className="pf-v6-u-mb-md">
          {!dbsLoaded ? (
            <Spinner size="md" aria-label="Loading external vector databases" />
          ) : dbsError ? (
            <Alert
              variant={AlertVariant.warning}
              isInline
              isPlain
              title="Could not load external vector databases"
              data-testid="external-db-error-alert"
            >
              {dbsError.message}
            </Alert>
          ) : externalDBs.length === 0 ? (
            <Alert
              variant={AlertVariant.info}
              isInline
              isPlain
              title="No external vector databases configured"
              data-testid="external-db-empty-alert"
            >
              Create a ConfigMap named &quot;gen-ai-external-vector-dbs&quot; in your project
              namespace to configure external vector databases.
            </Alert>
          ) : (
            <Dropdown
              isOpen={isDropdownOpen}
              onSelect={handleProviderSelect}
              onOpenChange={(open: boolean) => setIsDropdownOpen(open)}
              popperProps={{
                appendTo: () => document.body,
                position: 'left',
              }}
              toggle={(toggleRef: React.Ref<MenuToggleElement>) => (
                <MenuToggle
                  ref={toggleRef}
                  onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                  isExpanded={isDropdownOpen}
                  style={{ width: '100%' }}
                  data-testid="external-vector-db-toggle"
                >
                  {selectedProvider ? selectedLabel : 'Available vector stores'}
                </MenuToggle>
              )}
              shouldFocusToggleOnSelect
            >
              <DropdownList
                style={{ maxHeight: '300px', overflowY: 'auto' }}
                data-testid="external-vector-db-list"
              >
                <DropdownItem
                  key={INLINE_PROVIDER_VALUE}
                  value={INLINE_PROVIDER_VALUE}
                  data-testid="external-db-option-inline"
                  description="Default inline vector store"
                >
                  Inline
                </DropdownItem>
                <Divider component="li" />
                {externalDBs.map((db) => (
                  <DropdownItem
                    key={db.name}
                    value={db.name}
                    data-testid={`external-db-option-${db.name}`}
                    description={
                      db.vector_store_id
                        ? `${db.provider_type} | VS: ${db.vector_store_id}`
                        : db.provider_type
                    }
                  >
                    {db.name}
                  </DropdownItem>
                ))}
              </DropdownList>
            </Dropdown>
          )}
        </FormGroup>

        <Divider className="pf-v6-u-mt-md" />

        {/* Inline RAG Upload Section */}
        <Content component={ContentVariants.h4}>Upload documents for inline RAG</Content>
        <FormGroup fieldId="sources">
          <ChatbotSourceUploadPanel
            successAlert={alerts.uploadSuccessAlert}
            errorAlert={alerts.errorAlert}
            handleSourceDrop={sourceManagement.handleSourceDrop}
            removeUploadedSource={sourceManagement.removeUploadedSource}
            filesWithSettings={sourceManagement.filesWithSettings}
            uploadedFilesCount={fileManagement.files.length}
            maxFilesAllowed={10}
            isFilesLoading={fileManagement.isLoading}
          />
        </FormGroup>
        <FormGroup fieldId="uploaded-files" className="pf-v6-u-mt-md">
          <AlertGroup hasAnimations isToast isLiveRegion>
            {alerts.deleteSuccessAlert}
          </AlertGroup>
          <UploadedFilesList
            files={fileManagement.files}
            isLoading={fileManagement.isLoading}
            isDeleting={fileManagement.isDeleting}
            error={fileManagement.error}
            onDeleteFile={fileManagement.deleteFileById}
            providerLabel={
              selectedProvider === INLINE_PROVIDER_VALUE || !selectedProvider
                ? 'inline'
                : selectedProvider
            }
            isInlineProvider={selectedProvider === INLINE_PROVIDER_VALUE || !selectedProvider}
          />
        </FormGroup>
      </Form>
    </TabContentWrapper>
  );
};

export default KnowledgeTabContent;
