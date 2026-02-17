import * as React from 'react';
import { ExternalVectorDB } from '~/app/types';
import { useGenAiAPI } from './useGenAiAPI';

type UseFetchExternalVectorDBsReturn = {
  data: ExternalVectorDB[];
  currentDefaultProvider: string;
  currentVectorStoreId: string;
  loaded: boolean;
  error: Error | undefined;
  refetch: () => void;
};

const useFetchExternalVectorDBs = (): UseFetchExternalVectorDBsReturn => {
  const { api, apiAvailable } = useGenAiAPI();
  const [data, setData] = React.useState<ExternalVectorDB[]>([]);
  const [currentDefaultProvider, setCurrentDefaultProvider] = React.useState('');
  const [currentVectorStoreId, setCurrentVectorStoreId] = React.useState('');
  const [loaded, setLoaded] = React.useState(false);
  const [error, setError] = React.useState<Error | undefined>(undefined);
  const fetchAttempted = React.useRef(false);

  const doFetch = React.useCallback(() => {
    if (!apiAvailable) {
      return;
    }

    api
      .getExternalVectorDBs({})
      .then((response) => {
        setData(response.databases);
        setCurrentDefaultProvider(response.current_default_provider);
        setCurrentVectorStoreId(response.current_vector_store_id);
        setLoaded(true);
        setError(undefined);
      })
      .catch((err) => {
        // eslint-disable-next-line no-console
        console.error('[useFetchExternalVectorDBs] Error fetching external vector DBs:', err);
        setError(err instanceof Error ? err : new Error(String(err)));
        setData([]);
        setCurrentDefaultProvider('');
        setCurrentVectorStoreId('');
        setLoaded(true);
      });
  }, [apiAvailable, api]);

  React.useEffect(() => {
    if (apiAvailable && !fetchAttempted.current) {
      fetchAttempted.current = true;
      doFetch();
    }
  }, [apiAvailable, doFetch]);

  const refetch = React.useCallback(() => {
    doFetch();
  }, [doFetch]);

  return { data, currentDefaultProvider, currentVectorStoreId, loaded, error, refetch };
};

export default useFetchExternalVectorDBs;
