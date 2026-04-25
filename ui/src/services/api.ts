const API_BASE = '/_api';

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
        return handleResponse<any[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}`);
        return handleResponse<any>(response);
    },

    create: async (data: { name?: string; content: string; basePath: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/specs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    update: async (id: string, data: Partial<any>) => {
        const response = await fetch(`${API_BASE}/specs/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}`, {
            method: 'DELETE',
        });
        return handleResponse<any>(response);
    },

    enable: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/enable`, {
            method: 'PUT',
        });
        return handleResponse<any>(response);
    },

    disable: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/disable`, {
            method: 'PUT',
        });
        return handleResponse<any>(response);
    },

    toggleTracing: async (id: string, enabled: boolean) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tracing`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled }),
        });
        return handleResponse<any>(response);
    },

    toggleExampleFallback: async (id: string, enabled: boolean) => {
        const response = await fetch(`${API_BASE}/specs/${id}/example-fallback`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled }),
        });
        return handleResponse<any>(response);
    },

    setBackendURI: async (id: string, backendUri: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/backend`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ backendUri }),
        });
        return handleResponse<any>(response);
    },

    setMode: async (id: string, mode: import('../types').SpecMode) => {
        const response = await fetch(`${API_BASE}/specs/${id}/mode`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ mode }),
        });
        return handleResponse<any>(response);
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
        return handleResponse<any>(response);
    },

    getTags: async (id: string) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tags`);
        return handleResponse<any>(response);
    },

    updateTags: async (id: string, tags: string[]) => {
        const response = await fetch(`${API_BASE}/specs/${id}/tags`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tags }),
        });
        return handleResponse<any>(response);
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
        return handleResponse<any[]>(response);
    },

    create: async (data: { name: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/tags`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    update: async (name: string, data: { name: string; description?: string }) => {
        const response = await fetch(`${API_BASE}/tags/${encodeURIComponent(name)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    delete: async (name: string) => {
        const response = await fetch(`${API_BASE}/tags/${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });
        return handleResponse<any>(response);
    },
};

// Operations API
export const operationsApi = {
    listBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/specs/${specId}/operations`);
        return handleResponse<any[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/operations/${id}`);
        return handleResponse<any>(response);
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

};

// Response configs API
export const responsesApi = {
    listByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses`);
        return handleResponse<any[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/responses/${id}`);
        return handleResponse<any>(response);
    },

    create: async (operationId: string, data: any) => {
        const response = await fetch(`${API_BASE}/operations/${operationId}/responses`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    update: async (id: string, data: Partial<any>) => {
        const response = await fetch(`${API_BASE}/responses/${id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        return handleResponse<any>(response);
    },

    delete: async (id: string) => {
        const response = await fetch(`${API_BASE}/responses/${id}`, {
            method: 'DELETE',
        });
        return handleResponse<any>(response);
    },

    updatePriority: async (id: string, priority: number) => {
        const response = await fetch(`${API_BASE}/responses/${id}/priority`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ priority }),
        });
        return handleResponse<any>(response);
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

// Statistics API
export const statsApi = {
    getGlobal: async () => {
        const response = await fetch(`${API_BASE}/stats`);
        return handleResponse<any>(response);
    },

    createStream: () => {
        return new EventSource(`${API_BASE}/stats/stream`);
    },

    getBySpec: async (specId: string) => {
        const response = await fetch(`${API_BASE}/stats/specs/${specId}`);
        return handleResponse<any>(response);
    },

    getByOperation: async (operationId: string) => {
        const response = await fetch(`${API_BASE}/stats/operations/${operationId}`);
        return handleResponse<any>(response);
    },

    reset: async () => {
        const response = await fetch(`${API_BASE}/stats/reset`, {
            method: 'POST',
        });
        return handleResponse<any>(response);
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
        return handleResponse<any[]>(response);
    },

    get: async (id: string) => {
        const response = await fetch(`${API_BASE}/traces/${id}`);
        return handleResponse<any>(response);
    },

    clear: async (specId?: string) => {
        const url = specId ? `${API_BASE}/traces?specId=${specId}` : `${API_BASE}/traces`;
        const response = await fetch(url, {
            method: 'DELETE',
        });
        return handleResponse<any>(response);
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
        return handleResponse<any>(response);
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

    test: async (id: string, input: {
        path?: Record<string, string>;
        query?: Record<string, string>;
        header?: Record<string, string>;
        body?: any;
    }) => {
        const response = await fetch(`${API_BASE}/scripts/${id}/test`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ input }),
        });
        return handleResponse<{ output: any; durationMs: number; error: string | null }>(response);
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

// Global Store API
export const storeApi = {
    list: async () => {
        const response = await fetch(`${API_BASE}/store`);
        return handleResponse<import('../types').StoreEntry[]>(response);
    },

    get: async (key: string) => {
        const response = await fetch(`${API_BASE}/store/${encodeURIComponent(key)}`);
        return handleResponse<{ key: string; value: any }>(response);
    },

    upsert: async (key: string, value: any) => {
        const response = await fetch(`${API_BASE}/store/${encodeURIComponent(key)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ value }),
        });
        return handleResponse<{ key: string; value: any }>(response);
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

    isConfigured: async (): Promise<boolean> => {
        const response = await fetch(`${API_BASE}/ai/status`);
        if (!response.ok) return false;
        const data = await response.json();
        return data.configured === true;
    },
};
