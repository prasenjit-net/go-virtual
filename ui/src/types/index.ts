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
    priority: number;
    conditions: Condition[];
    statusCode: number;
    headers: Record<string, string>;
    body: string;
    delay: number;
    enabled: boolean;
}

export interface ResponseConfigInput {
    name: string;
    description?: string;
    priority: number;
    conditions: Condition[];
    statusCode: number;
    headers: Record<string, string>;
    body: string;
    delay?: number;
    enabled: boolean;
}

// Condition types
export interface Condition {
    source: 'path' | 'query' | 'header' | 'body';
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
