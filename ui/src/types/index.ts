// Spec types
export interface Spec {
    id: string;
    name: string;
    version: string;
    description: string;
    content?: string;
    basePath: string;
    enabled: boolean;
    tracing: boolean;
    useExampleFallback: boolean;
    enabledTags?: string[];
    backendUri: string;
    proxyMode: boolean;
    createdAt: string;
    updatedAt: string;
    operationCount?: number;
}

export interface SpecInput {
    name?: string;
    content: string;
    basePath: string;
    description?: string;
}

// Operation types
export interface Operation {
    id: string;
    specId: string;
    method: string;
    path: string;
    fullPath: string;
    operationId: string;
    summary: string;
    description: string;
    tags: string[];
    responses?: ResponseConfig[];
    exampleResponse?: ExampleResponse;
    signatureConfig?: SignatureConfig | null;
}

// SignatureConfig controls which request parts contribute to the signature hash
export interface SignatureConfig {
    // Specific path parameter names to include. Empty = include ALL.
    pathParams: string[];
    // Specific query parameter names to include. Empty = include ALL.
    queryParams: string[];
    // Specific header names to include. Empty = include NONE.
    headers: string[];
    // Whether the request body contributes to the signature.
    includeBody: boolean;
    // Specific gjson paths within the body to include. Empty + includeBody = full body.
    bodyJsonPaths: string[];
}

export interface ExampleResponse {
    statusCode: number;
    headers?: Record<string, string>;
    body: string;
}

export interface OperationSummary {
    id: string;
    specId: string;
    method: string;
    path: string;
    fullPath: string;
    operationId: string;
    summary: string;
    responseCount: number;
    hasExampleResponse: boolean;
}

// Response config types
export interface ResponseConfig {
    id: string;
    operationId: string;
    name: string;
    description: string;
    tag?: string;
    priority: number;
    conditions: Condition[];
    statusCode: number;
    headers: Record<string, string>;
    body: string;
    delay: number;
    enabled: boolean;
    recorded: boolean;
}

export interface ResponseConfigInput {
    name: string;
    description?: string;
    tag?: string;
    priority: number;
    conditions: Condition[];
    statusCode: number;
    headers: Record<string, string>;
    body: string;
    delay?: number;
    enabled: boolean;
}

export interface Tag {
    name: string;
    description?: string;
    createdAt?: string;
    updatedAt?: string;
}

// Condition types
export interface Condition {
    source: 'path' | 'query' | 'header' | 'body' | 'signature';
    key: string;
    operator: ConditionOperator;
    value: string;
}

export type ConditionOperator =
    | 'eq' | 'ne' | 'contains' | 'notContains'
    | 'regex' | 'exists' | 'notExists'
    | 'gt' | 'lt' | 'gte' | 'lte'
    | 'startsWith' | 'endsWith';

// Trace types
export interface Trace {
    id: string;
    specId: string;
    specName: string;
    operationId: string;
    operationPath: string;
    timestamp: string;
    duration: number;
    request: TraceRequest;
    response: TraceResponse;
    matchedConfigId?: string;
    matchedConfig?: string;
    // Proxy recording fields
    proxyMode?: boolean;
    signature?: string;
    backendUri?: string;
}

export interface TraceRequest {
    method: string;
    url: string;
    path: string;
    query: Record<string, string[]>;
    headers: Record<string, string[]>;
    body: string;
}

export interface TraceResponse {
    statusCode: number;
    headers: Record<string, string[]>;
    body: string;
}

// Statistics types
export interface GlobalStats {
    totalRequests: number;
    totalErrors: number;
    activeSpecs: number;
    totalOperations: number;
    avgResponseTimeMs: number;
    requestsPerSecond: number;
    startTime: string;
    uptime: string;
    topOperations: OperationStat[];
    recentErrors: ErrorStat[];
    requestsByHour: HourlyStat[];
}

export interface SpecStats {
    specId: string;
    specName: string;
    totalRequests: number;
    totalErrors: number;
    avgResponseTimeMs: number;
    operations: OperationStat[];
}

export interface OperationStat {
    operationId: string;
    specId: string;
    method: string;
    path: string;
    totalRequests: number;
    totalErrors: number;
    avgResponseTimeMs: number;
    minResponseTimeMs: number;
    maxResponseTimeMs: number;
    lastRequestTime?: string;
}

export interface ErrorStat {
    timestamp: string;
    specId: string;
    operationId: string;
    path: string;
    method: string;
    statusCode: number;
    error: string;
}

export interface HourlyStat {
    hour: string;
    requests: number;
    errors: number;
}

// Branding configuration returned by /_api/branding
export interface Branding {
    appTitle: string;
    appSubtitle: string;
}

// ---- Scripting Phase 1 ----

export interface Script {
    id: string;
    name: string;
    description: string;
    timeout: number;
    enabled: boolean;
    createdAt: string;
    updatedAt: string;
    source?: string; // Included only in GET /:id, absent in list
}

export interface ScriptInput {
    name: string;
    description: string;
    source: string;
    timeout: number;
    enabled: boolean;
}

export interface ScriptBinding {
    id: string;
    operationId: string;
    scriptId: string;
    scriptName?: string;
    outputKey: string;
    order: number;
    enabled: boolean;
}

export interface ScriptBindingInput {
    scriptId: string;
    outputKey: string;
    order: number;
    enabled: boolean;
}

export interface ScriptTrace {
    bindingId: string;
    scriptId: string;
    scriptName: string;
    outputKey: string;
    durationMs: number;
    output?: any;
    error?: string;
}
