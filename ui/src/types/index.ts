// Spec types
export type SpecMode = 'standard' | 'ai' | 'proxy';
export type ResponseOrigin = 'manual' | 'proxy' | 'ai';
export type ResponseTier = 'configured' | 'recorded' | 'fallback';

export interface AIStatus {
    configured: boolean;
    provider: string;
    model?: string;
}

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
    signatureHeaders?: string[];
    mode: SpecMode;
    backendUri: string;
    proxyMode: boolean;
    modePolicy: ModePolicy;
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
    declaredPathParams?: string[];
    declaredQueryParams?: string[];
    declaredHeaderParams?: string[];
    declaredBodyFields?: string[];
    hasRequestBody?: boolean;
}

export interface ConditionalModeConfig {
    enabled: boolean;
    conditions: Condition[];
}

export interface ModePolicy {
    configured?: boolean;
    ai: ConditionalModeConfig;
    proxy: ConditionalModeConfig;
}

export type AIScenarioKind = 'success' | 'error';

export interface AIScenario {
    id: string;
    name: string;
    description: string;
    responseKind: AIScenarioKind;
    statusCode: number;
    count: number;
    instructions: string;
    enabled: boolean;
    createdAt: string;
    updatedAt: string;
}

// SignatureConfig controls which request parts contribute to the signature hash
export interface SignatureConfig {
    // Specific path parameter names to include. Empty = include ALL.
    pathParams: string[];
    // Specific query parameter names to include. Empty = include ALL.
    queryParams: string[];
    // Whether the header list explicitly overrides default declared/spec headers.
    headersConfigured?: boolean;
    // Specific header names to include. Empty + headersConfigured=true = include NONE.
    headers: string[];
    // Whether the request body contributes to the signature. Null/undefined = use default.
    includeBody?: boolean | null;
    // Specific gjson paths within the body to include. Empty + includeBody = full body.
    bodyJsonPaths: string[];
}

export interface SignatureAvailableInputs {
    pathParams: string[];
    queryParams: string[];
    headerParams: string[];
    bodyFields: string[];
    hasBody: boolean;
}

export interface SignatureConfigResponse {
    signatureConfig: SignatureConfig | null;
    defaultSignatureConfig: SignatureConfig;
    effectiveSignatureConfig: SignatureConfig;
    availableInputs: SignatureAvailableInputs;
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
    origin: ResponseOrigin;
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
    source: 'path' | 'query' | 'header' | 'body' | 'signature' | 'script';
    key: string;
    operator: ConditionOperator;
    value: string;
    /** Optional Go time layout hint for date operators (e.g. "2006-01-02"). Auto-detected when absent. */
    format?: string;
    /** When true, the condition result is inverted. */
    negate?: boolean;
}

export type ConditionOperator =
    | 'eq' | 'contains'
    | 'regex' | 'exists'
    | 'gt' | 'lt' | 'gte' | 'lte'
    | 'startsWith' | 'endsWith'
    // Date operators — value accepts date literals or dynamic tokens:
    // now, today, yesterday, tomorrow, now+Nd, now-Nd, now+Nh, now-Nh, now+Nm, now-Nm
    | 'dateEq' | 'dateBefore' | 'dateAfter' | 'dateLte' | 'dateGte'
    | 'dateInPast' | 'dateInFuture' | 'dateToday'
    | 'dateBetween'; // value = "<from>,<to>"

/** Deprecated operators kept for backward compat — normalised client-side */
export type DeprecatedConditionOperator = 'ne' | 'notContains' | 'notExists';

export interface RegexPatternToken {
    token: string;
    description: string;
    pattern: string;
}

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
    matchedConfigOrigin?: ResponseOrigin;
    mode?: SpecMode;
    responseSource?: 'config' | 'example' | 'ai' | 'proxy';
    responseTier?: ResponseTier;
    aiSkippedReason?: string;
    proxySkippedReason?: string;
    aiScenarioRequested?: string;
    aiScenarioApplied?: string;
    // Proxy recording fields
    proxyMode?: boolean;
    signature?: string;
    backendUri?: string;
    // Phase 1: script traces
    scripts?: ScriptTrace[];
    // Phase 2: session trace
    session?: SessionTrace;
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
    specId?: string;
    operationId?: string;
    responseConfigId?: string;
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
    logs?: string[];
}

// ---- Scripting Phase 2 — Session Store ----

export interface StoreEntry {
    key: string;
    value: any;
    createdAt: string;
    updatedAt: string;
}

export interface StoreEntryInput {
    value: any;
}

export interface SessionInfo {
    id: string;
    createdAt: string;
    lastActive: string;
    entryCount: number;
    storeSnapshot?: Record<string, any>;
}

export interface SessionListResponse {
    sessions: SessionInfo[];
    count: number;
}

export interface StoreAccessEvent {
    op: string;
    key?: string;
    value?: any;
}

export interface SessionTrace {
    id: string;
    isNew: boolean;
    storeAccess?: StoreAccessEvent[];
}

// ---- Archives ----

export interface ArchiveCounts {
    specs: number;
    responses: number;
    scripts: number;
    tags: number;
    storeEntries: number;
}

export interface ArchiveMeta {
    id: string;
    filename: string;
    label: string;
    createdAt: string;
    sizeBytes: number;
    appVersion: string;
    counts: ArchiveCounts;
}

export interface RestoreError {
    path: string;
    message: string;
}

export interface RestoreResult {
    created: Record<string, number>;
    updated: Record<string, number>;
    wipedFirst: boolean;
    errors?: RestoreError[];
}

export interface RestoreResponse {
    backupCreated?: ArchiveMeta;
    result: RestoreResult;
}
