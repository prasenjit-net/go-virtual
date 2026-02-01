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

// Statistics API
export const statsApi = {
    getGlobal: async () => {
        const response = await fetch(`${API_BASE}/stats`);
        return handleResponse<any>(response);
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

// Routes API
export const routesApi = {
    get: async () => {
        const response = await fetch(`${API_BASE}/routes`);
        return handleResponse<Record<string, string[]>>(response);
    },
};
