const API_BASE = '/_api';

import type { Operation, ResponseConfig, Spec, Tag } from '../types';

async function handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
        const error = await response.json().catch(() => ({ error: 'Unknown error' }));
        throw new Error(error.error || `HTTP ${response.status}`);
    }
    return response.json();
}

// Specs API
export const specsApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/specs`);
        return handleResponse<Spec[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}`);
        return handleResponse<Spec>(response);
    },

    create: async (data: { name?: string; content: string; basePath: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/specs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<Spec>(response);
    },

    update: async (id: string, data: Partial<Spec>) => {
        const response = await fetch(`${API_BASE}/specs/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<Spec>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}`, {
            method: 'DELETE',
        });
        return handleResponse<{ message: string }>(response);
    },

    enable: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/enable`, {
            method: 'PUT',
        });
        return handleResponse<Spec>(response);
    },

    disable: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/disable`, {
            method: 'PUT',
        });
        return handleResponse<Spec>(response);
    },

    toggleTracing: async (id: string, enabled: boolean) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tracing`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled }),
        });
        return handleResponse<Spec>(response);
    },

    toggleExampleFallback: async (id: string, enabled: boolean) => {
        const response = await fetch(`${API_BASE}/specs/${id}/example-fallback`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled }),
        });
        return handleResponse<Spec>(response);
    },

    setBackendURI: async (id: string, backendUri: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/backend`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ backendUri }),
        });
        return handleResponse<Spec>(response);
    },

    setMode: async (id: string, mode: import('../types').SpecMode) => {
        const response = await fetch(`${API_BASE}/specs/${id}/mode`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ mode }),
        });
        return handleResponse<Spec>(response);
    },

    getModePolicy: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/mode-policy`);
        return handleResponse<{ modePolicy: import('../types').ModePolicy }>(response);
    },

    updateModePolicy: async (id: string, modePolicy: import('../types').ModePolicy) => {
        const response = await fetch(`${API_BASE}/specs/${id}/mode-policy`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ modePolicy }),
        });
        return handleResponse<{ modePolicy: import('../types').ModePolicy }>(response);
    },

    toggleProxyMode: async (id: string, enabled: boolean) => {
        const response = await fetch(`${API_BASE}/specs/${id}/proxy-mode`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled }),
        });
        return handleResponse<Spec>(response);
    },

    getTags: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tags`);
        return handleResponse<{ tags: string[] }>(response);
    },

    updateTags: async (id: string, tags: string[]) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tags`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tags }),
        });
        return handleResponse<{ tags: string[] }>(response);
    },
};

export const aiScenariosApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/ai-scenarios`);
        return handleResponse<{ scenarios: import('../types').AIScenario[] }>(response);
    },

    create: async (scenario: Partial<import('../types').AIScenario>) => {
        const response = await fetch(`${API_BASE}/ai-scenarios`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ scenario }),
        });
        return handleResponse<{ scenario: import('../types').AIScenario }>(response);
    },

    update: async (scenarioId: string, scenario: Partial<import('../types').AIScenario>) => {
        const response = await fetch(`${API_BASE}/ai-scenarios/${scenarioId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ scenario }),
        });
        return handleResponse<{ scenario: import('../types').AIScenario }>(response);
    },

    delete: async (scenarioId: string) => {
        const response = await fetch(`${API_BASE}/ai-scenarios/${scenarioId}`, {
            method: 'DELETE',
        });
        return handleResponse<{ message: string }>(response);
    },
};

// Tags API
export const tagsApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/tags`);
        return handleResponse<Tag[]>(response);
    },

    create: async (data: { name: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/tags`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<Tag>(response);
    },

    update: async (name: string, data: { name: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/tags/${encodeURIComponent(name)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<Tag>(response);
    },

    delete: async (name: string) => {
        const response = await fetch(`${API_BASE}/tags/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        return handleResponse<{ message: string }>(response);
    },
};

// Operations API
export const operationsApi = {
    listBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/operations`);
        return handleResponse<import('../types').OperationSummary[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/operations/${id}`);
        return handleResponse<Operation>(response);
    },

    getSignatureConfig: async (id: string) => {
        const response = await fetch(`${API_BASE}/operations/${id}/signature`);
        return handleResponse<import('../types').SignatureConfigResponse>(response);
    },

    updateSignatureConfig: async (id: string, signatureConfig: import('../types').SignatureConfig | null) => {
        const response = await fetch(`${API_BASE}/operations/${id}/signature`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ signatureConfig }),
        });
        return handleResponse<{ signatureConfig: import('../types').SignatureConfig | null; effectiveSignatureConfig: import('../types').SignatureConfig }>(response);
    },

    getSpecExamples: async (id: string) => {
        const response = await fetch(`${API_BASE}/operations/${id}/spec-examples`);
        return handleResponse<import('../types').SpecExample[]>(response);
    },

};

// Response configs API
export const responsesApi = {
    listByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses`);
        return handleResponse<ResponseConfig[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/responses/${id}`);
        return handleResponse<ResponseConfig>(response);
    },

    create: async (operationId: string, data: import('../types').ResponseConfigInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<ResponseConfig>(response);
    },

    update: async (id: string, data: Partial<import('../types').ResponseConfigInput>) => {
        const response = await fetch(`${API_BASE}/responses/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<ResponseConfig>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/responses/${id}`, {
            method: 'DELETE',
        });
        return handleResponse<{ message: string }>(response);
    },

    updatePriority: async (id: string, priority: number) => {
        const response = await fetch(`${API_BASE}/responses/${id}/priority`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ priority }),
        });
        return handleResponse<ResponseConfig>(response);
    },

    clone: async (id: string, name: string) => {
        const response = await fetch(`${API_BASE}/responses/${id}/clone`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name }),
        });
        return handleResponse<import('../types').ResponseConfig>(response);
    },
};

// Templates API
export const templatesApi = {
    validate: async (body: string) => {
        const response = await fetch(`${API_BASE}/templates/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ body }),
        });

        if (response.ok) {
            return { valid: true as const };
        }

        const error = await response.json().catch(() => ({ error: 'Invalid template' }));
        return { valid: false as const, error: error.error || 'Invalid template' };
    },
};

// Conditions API
export const conditionsApi = {
    listRegexPatterns: async () => {
        const response = await fetch(`${API_BASE}/conditions/regex-patterns`);
        return handleResponse<{ token: string; description: string; pattern: string }[]>(response);
    },
};

// Statistics API
export const statsApi = {
    getGlobal: async () => {
        const response = await fetch(`${API_BASE}/stats`);
        return handleResponse<import('../types').GlobalStats>(response);
    },

    createStream: () => {
        return new EventSource(`${API_BASE}/stats/stream`);
    },

    getBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/stats/specs/${specId}`);
        return handleResponse<import('../types').SpecStats>(response);
    },

    getByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/stats/operations/${operationId}`);
        return handleResponse<import('../types').SpecStats>(response);
    },

    reset: async () => {
        const response = await fetch(`${API_BASE}/stats/reset`, {
            method: 'POST',
        });
        return handleResponse<{ message: string }>(response);
    },
};

// Traces API
export const tracesApi = {
    list: async (params?: { specId?: string; operationId?: string; method?: string }) => {
        const searchParams = new URLSearchParams();
        if (params?.specId) searchParams.set('specId', params.specId);
        if (params?.operationId) searchParams.set('operationId', params.operationId);
        if (params?.method) searchParams.set('method', params.method);

        const response = await fetch(`${API_BASE}/traces?${searchParams}`);
        return handleResponse<import('../types').Trace[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/traces/${id}`);
        return handleResponse<import('../types').Trace>(response);
    },

    clear: async (specId?: string) => {
        const url = specId ? `${API_BASE}/traces?specId=${specId}` : `${API_BASE}/traces`;
        const response = await fetch(url, {
            method: 'DELETE',
        });
        return handleResponse<{ message: string }>(response);
    },

    // WebSocket for live traces
    createStream: () => {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        return new WebSocket(`${protocol}//${window.location.host}/_api/traces/stream`);
    },
};

// Health API
export const healthApi = {
    check: async () => {
        const response = await fetch(`${API_BASE}/health`);
        return handleResponse<{ status: string }>(response);
    },
};

// Branding API
export const brandingApi = {
    get: async () => {
        const response = await fetch(`${API_BASE}/branding`);
        return handleResponse<{ appTitle: string; appSubtitle: string }>(response);
    },
};

// Routes API
export const routesApi = {
    get: async () => {
        const response = await fetch(`${API_BASE}/routes`);
        return handleResponse<Record<string, string[]>>(response);
    },
};

// Scripts API
export const scriptsApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/scripts`);
        return handleResponse<import('../types').Script[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/scripts/${id}`);
        return handleResponse<import('../types').Script>(response);
    },

    create: async (data: import('../types').ScriptInput) => {
        const response = await fetch(`${API_BASE}/scripts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').Script>(response);
    },

    update: async (id: string, data: import('../types').ScriptInput) => {
        const response = await fetch(`${API_BASE}/scripts/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').Script>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/scripts/${id}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    validate: async (source: string) => {
        const response = await fetch(`${API_BASE}/scripts/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ source }),
        });
        return handleResponse<{ valid: boolean; error: string | null }>(response);
    },

    testSource: async (source: string, timeout: number, input: {
        path?: Record<string, string>;
        query?: Record<string, string>;
        header?: Record<string, string>;
        body?: unknown;
    }) => {
        const response = await fetch(`${API_BASE}/scripts/test-source`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ source, timeout, input }),
        });
        return handleResponse<{ output: unknown; durationMs: number; logs?: string[]; error: string | null }>(response);
    },

    test: async (id: string, input: {
        path?: Record<string, string>;
        query?: Record<string, string>;
        header?: Record<string, string>;
        body?: unknown;
    }) => {
        const response = await fetch(`${API_BASE}/scripts/${id}/test`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ input }),
        });
        return handleResponse<{ output: unknown; durationMs: number; error: string | null }>(response);
    },
};

// Script Bindings API
export const scriptBindingsApi = {
    listByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/scripts`);
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },

    create: async (operationId: string, data: import('../types').ScriptBindingInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/scripts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    update: async (operationId: string, bindingId: string, data: Partial<import('../types').ScriptBindingInput>) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/scripts/${bindingId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    delete: async (operationId: string, bindingId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/scripts/${bindingId}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    reorder: async (operationId: string, items: { id: string; order: number }[]) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/scripts/reorder`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(items),
        });
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },
};

export const specScriptBindingsApi = {
    list: async (specId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/scripts`);
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },

    create: async (specId: string, data: import('../types').ScriptBindingInput) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/scripts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    update: async (specId: string, bindingId: string, data: Partial<import('../types').ScriptBindingInput>) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/scripts/${bindingId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    delete: async (specId: string, bindingId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/scripts/${bindingId}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    reorder: async (specId: string, items: { id: string; order: number }[]) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/scripts/reorder`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(items),
        });
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },
};

export const responseScriptBindingsApi = {
    listByResponse: async (operationId: string, responseConfigId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/scripts`);
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },

    create: async (operationId: string, responseConfigId: string, data: import('../types').ScriptBindingInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/scripts`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    update: async (operationId: string, responseConfigId: string, bindingId: string, data: Partial<import('../types').ScriptBindingInput>) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/scripts/${bindingId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ScriptBinding>(response);
    },

    delete: async (operationId: string, responseConfigId: string, bindingId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/scripts/${bindingId}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    reorder: async (operationId: string, responseConfigId: string, items: { id: string; order: number }[]) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/scripts/reorder`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(items),
        });
        return handleResponse<import('../types').ScriptBinding[]>(response);
    },
};

// Collection Mappings API
export const collectionMappingsApi = {
    listBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/mappings`);
        return handleResponse<import('../types').CollectionMapping[]>(response);
    },

    createForSpec: async (specId: string, data: import('../types').CollectionMappingInput) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/mappings`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').CollectionMapping>(response);
    },

    listByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/mappings`);
        return handleResponse<import('../types').CollectionMapping[]>(response);
    },

    createForOperation: async (operationId: string, data: import('../types').CollectionMappingInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/mappings`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').CollectionMapping>(response);
    },

    listByResponse: async (operationId: string, responseConfigId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/mappings`);
        return handleResponse<import('../types').CollectionMapping[]>(response);
    },

    create: async (operationId: string, responseConfigId: string, data: import('../types').CollectionMappingInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses/${responseConfigId}/mappings`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').CollectionMapping>(response);
    },

    update: async (mappingId: string, data: import('../types').CollectionMappingInput) => {
        const response = await fetch(`${API_BASE}/mappings/${mappingId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').CollectionMapping>(response);
    },

    delete: async (mappingId: string) => {
        const response = await fetch(`${API_BASE}/mappings/${mappingId}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },
};

// Validation Rules API
export const validationRulesApi = {
    listBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/validations`);
        return handleResponse<import('../types').ValidationRule[]>(response);
    },

    listByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/validations`);
        return handleResponse<import('../types').ValidationRule[]>(response);
    },

    createForSpec: async (specId: string, data: import('../types').ValidationInput) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/validations`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ValidationRule>(response);
    },

    createForOperation: async (operationId: string, data: import('../types').ValidationInput) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/validations`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ValidationRule>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/validations/${id}`);
        return handleResponse<import('../types').ValidationRule>(response);
    },

    update: async (id: string, data: import('../types').ValidationInput) => {
        const response = await fetch(`${API_BASE}/validations/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<import('../types').ValidationRule>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/validations/${id}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },
};

// Global Store API
export const storeApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/store`);
        return handleResponse<import('../types').StoreEntry[]>(response);
    },

    get: async (key: string) => {
        const response = await fetch(`${API_BASE}/store/${encodeURIComponent(key)}`);
        return handleResponse<{ key: string; value: unknown }>(response);
    },

    upsert: async (key: string, value: unknown) => {
        const response = await fetch(`${API_BASE}/store/${encodeURIComponent(key)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ value }),
        });
        return handleResponse<{ key: string; value: unknown }>(response);
    },

    delete: async (key: string) => {
        const response = await fetch(`${API_BASE}/store/${encodeURIComponent(key)}`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    clear: async () => {
        const response = await fetch(`${API_BASE}/store?confirm=true`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },
};

// Collections API
export const collectionsApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/store/collections`);
        return handleResponse<import('../types').CollectionInfo[]>(response);
    },

    get: async (name: string) => {
        const response = await fetch(`${API_BASE}/store/collections/${encodeURIComponent(name)}`);
        return handleResponse<import('../types').CollectionDocument[]>(response);
    },

    insert: async (name: string, doc: import('../types').CollectionDocument) => {
        const response = await fetch(`${API_BASE}/store/collections/${encodeURIComponent(name)}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(doc),
        });
        return handleResponse<import('../types').CollectionDocument>(response);
    },

    update: async (name: string, index: number, changes: import('../types').CollectionDocument) => {
        const response = await fetch(`${API_BASE}/store/collections/${encodeURIComponent(name)}/${index}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(changes),
        });
        return handleResponse<import('../types').CollectionDocument>(response);
    },

    deleteDoc: async (name: string, index: number) => {
        const response = await fetch(`${API_BASE}/store/collections/${encodeURIComponent(name)}/${index}`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    clear: async (name: string) => {
        const response = await fetch(`${API_BASE}/store/collections/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },
};

// Sessions API
export const sessionsApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/sessions`);
        return handleResponse<import('../types').SessionListResponse>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/sessions/${encodeURIComponent(id)}`);
        return handleResponse<import('../types').SessionInfo>(response);
    },

    invalidate: async (id: string) => {
        const response = await fetch(`${API_BASE}/sessions/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    invalidateAll: async () => {
        const response = await fetch(`${API_BASE}/sessions?confirm=true`, {
            method: 'DELETE',
        });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },
};
// Archives API
export const archivesApi = {
    info: async () => {
        const response = await fetch(`${API_BASE}/archives/info`);
        return handleResponse<{ mode: 'full' | 'snapshot' }>(response);
    },

    list: async () => {
        const response = await fetch(`${API_BASE}/archives`);
        return handleResponse<import('../types').ArchiveMeta[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/archives/${id}`);
        return handleResponse<import('../types').ArchiveMeta>(response);
    },

    create: async (label?: string) => {
        const response = await fetch(`${API_BASE}/archives`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ label: label ?? '' }),
        });
        return handleResponse<import('../types').ArchiveMeta>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/archives/${id}`, { method: 'DELETE' });
        if (!response.ok && response.status !== 204) {
            const text = await response.text();
            throw new Error(text || `HTTP ${response.status}`);
        }
    },

    downloadUrl: (id: string) => `${API_BASE}/archives/${id}/download`,

    snapshotDownloadUrl: () => `${API_BASE}/archives/snapshot`,

    upload: async (file: File, label?: string) => {
        const form = new FormData();
        form.append('archive', file);
        if (label) form.append('label', label);
        const response = await fetch(`${API_BASE}/archives/upload`, {
            method: 'POST',
            body: form,
        });
        return handleResponse<import('../types').ArchiveMeta>(response);
    },

    restoreSnapshot: async (file: File) => {
        const form = new FormData();
        form.append('archive', file);
        const response = await fetch(`${API_BASE}/archives/snapshot/restore`, {
            method: 'POST',
            body: form,
        });
        return handleResponse<import('../types').RestoreResponse>(response);
    },

    restore: async (id: string, opts: {
        createBackupFirst: boolean;
        backupLabel?: string;
        wipeFirst: boolean;
    }) => {
        const response = await fetch(`${API_BASE}/archives/${id}/restore`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                createBackupFirst: opts.createBackupFirst,
                backupLabel: opts.backupLabel ?? '',
                wipeFirst: opts.wipeFirst,
            }),
        });
        return handleResponse<import('../types').RestoreResponse>(response);
    },
};

// AI generation API
export const aiApi = {
    generateResponse: async (operationId: string, userPrompt?: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/ai-response`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ userPrompt: userPrompt ?? '' }),
        });
        return handleResponse<import('../types').ResponseConfig>(response);
    },

    generateScript: async (
        userPrompt: string,
        options?: { operationId?: string; currentSource?: string; history?: Array<{ role: 'user' | 'assistant'; content: string }> }
    ): Promise<{ source: string }> => {
        const response = await fetch(`${API_BASE}/scripts/ai-generate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                userPrompt,
                operationId: options?.operationId ?? '',
                currentSource: options?.currentSource ?? '',
                history: options?.history ?? [],
            }),
        });
        return handleResponse<{ source: string }>(response);
    },

    getStatus: async (): Promise<import('../types').AIStatus> => {
        const response = await fetch(`${API_BASE}/ai/status`);
        if (!response.ok) {
            return { configured: false, provider: 'openai' };
        }
        return handleResponse<import('../types').AIStatus>(response);
    },

    isConfigured: async (): Promise<boolean> => {
        const data = await aiApi.getStatus();
        return data.configured === true;
    },
};

// Pipeline API
export const pipelineApi = {
    list: async (scope: import('../types').PipelineScope, scopeId: string): Promise<{ steps: import('../types').PipelineStep[] }> => {
        const prefix = scope === 'spec' ? 'specs' : scope === 'operation' ? 'operations' : 'responses';
        const response = await fetch(`${API_BASE}/${prefix}/${scopeId}/pipeline`);
        return handleResponse<{ steps: import('../types').PipelineStep[] }>(response);
    },

    reorder: async (scope: import('../types').PipelineScope, scopeId: string, items: import('../types').PipelineReorderItem[]): Promise<{ reordered: number }> => {
        const prefix = scope === 'spec' ? 'specs' : scope === 'operation' ? 'operations' : 'responses';
        const response = await fetch(`${API_BASE}/${prefix}/${scopeId}/pipeline/reorder`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(items),
        });
        return handleResponse<{ reordered: number }>(response);
    },
};
