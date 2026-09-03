import { useEffect, useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    AlertCircle,
    ArrowLeft,
    Check,
    Plus,
    Edit2,
    Sparkles,
    Info,
    Fingerprint,
    X,
    Radio,
    Database
} from 'lucide-react'
import clsx from 'clsx'
import { operationsApi, responsesApi, specsApi, aiApi } from '../services/api'
import type { AIStatus, Operation, ResponseConfig, ResponseConfigInput, SignatureAvailableInputs, SignatureConfig, SignatureConfigResponse, Spec } from '../types'
import PipelinePanel from './Pipeline/PipelinePanel'
import AIGenerateModal from './ResponseDesigner/AIGenerateModal'
import ResponseConfigList from './ResponseDesigner/ResponseConfigList'
import ResponseImportModal from './ResponseDesigner/ResponseImportModal'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
}

const emptySignatureInputs: SignatureAvailableInputs = {
    pathParams: [],
    queryParams: [],
    headerParams: [],
    bodyFields: [],
    hasBody: false,
}

function createSignatureDraft(config: SignatureConfig | null | undefined): SignatureConfig {
    return {
        pathParams: [...(config?.pathParams ?? [])],
        queryParams: [...(config?.queryParams ?? [])],
        headersConfigured: config?.headersConfigured ?? false,
        headers: [...(config?.headers ?? [])],
        includeBody: config?.includeBody ?? null,
        bodyJsonPaths: [...(config?.bodyJsonPaths ?? [])],
    }
}

function normalizeSignatureDraftForSave(draft: SignatureConfig | null): SignatureConfig | null {
    if (!draft) return null
    const normalized: SignatureConfig = {
        pathParams: [...draft.pathParams],
        queryParams: [...draft.queryParams],
        headersConfigured: draft.headersConfigured ?? false,
        headers: draft.headersConfigured ? [...draft.headers] : [],
        includeBody: draft.includeBody ?? null,
        bodyJsonPaths: draft.includeBody === false ? [] : [...draft.bodyJsonPaths],
    }
    const hasOverride =
        normalized.pathParams.length > 0 ||
        normalized.queryParams.length > 0 ||
        normalized.headersConfigured ||
        normalized.includeBody !== null && normalized.includeBody !== undefined ||
        normalized.bodyJsonPaths.length > 0
    return hasOverride ? normalized : null
}

export default function OperationDetail() {
    const { operationId } = useParams<{ operationId: string }>()
    const [showAIModal, setShowAIModal] = useState(false)
    const [showImportModal, setShowImportModal] = useState(false)
    const [importFeedback, setImportFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
    const queryClient = useQueryClient()

    useEffect(() => {
        if (!importFeedback) return
        const timeoutId = window.setTimeout(() => setImportFeedback(null), 3000)
        return () => window.clearTimeout(timeoutId)
    }, [importFeedback])

    const { data: operation, isLoading: opLoading } = useQuery<Operation>({
        queryKey: ['operation', operationId],
        queryFn: () => operationsApi.get(operationId!),
        enabled: !!operationId,
    })

    const { data: spec, isLoading: specLoading } = useQuery<Spec>({
        queryKey: ['spec', operation?.specId],
        queryFn: () => specsApi.get(operation!.specId),
        enabled: !!operation?.specId,
    })

    const { data: responses, isLoading: respLoading } = useQuery<ResponseConfig[]>({
        queryKey: ['responses', operationId],
        queryFn: () => responsesApi.listByOperation(operationId!),
        enabled: !!operationId,
    })

    // Signature config state
    const [sigEditMode, setSigEditMode] = useState(false)
    const [sigConfig, setSigConfig] = useState<SignatureConfig | null>(null)
    const [newHeader, setNewHeader] = useState('')
    const [newQueryParam, setNewQueryParam] = useState('')
    const [newBodyPath, setNewBodyPath] = useState('')

    const signatureQuery = useQuery<SignatureConfigResponse>({
        queryKey: ['signature', operationId],
        queryFn: () => operationsApi.getSignatureConfig(operationId!),
        enabled: !!operationId,
    })

    const { data: aiStatus = { configured: true, provider: 'openai' } } = useQuery<AIStatus>({
        queryKey: ['ai-status'],
        queryFn: () => aiApi.getStatus(),
        staleTime: 60_000,
    })
    const aiConfigured = aiStatus.configured
    const aiProviderLabel = aiStatus.provider === 'claude' ? 'Claude' : aiStatus.provider === 'openai' ? 'OpenAI' : 'AI provider'

    const updateSignatureMutation = useMutation({
        mutationFn: (cfg: SignatureConfig | null) =>
            operationsApi.updateSignatureConfig(operationId!, cfg),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['signature', operationId] })
            queryClient.invalidateQueries({ queryKey: ['operation', operationId] })
            setSigEditMode(false)
        },
    })

    const importResponseMutation = useMutation({
        mutationFn: (input: ResponseConfigInput) => responsesApi.create(operationId!, input),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            setImportFeedback({ type: 'success', message: 'Response imported successfully.' })
            setShowImportModal(false)
        },
        onError: (err: Error) => {
            setImportFeedback({ type: 'error', message: err.message || 'Failed to import response.' })
        },
    })

    const signatureData = signatureQuery.data
    const availableInputs = signatureData?.availableInputs ?? emptySignatureInputs
    const defaultSignatureConfig = signatureData?.defaultSignatureConfig
    const effectiveSignatureConfig = signatureData?.effectiveSignatureConfig
    const bodyIncluded = useMemo(
        () => sigConfig?.includeBody ?? availableInputs.hasBody,
        [sigConfig?.includeBody, availableInputs.hasBody]
    )

    const openSigEdit = () => {
        setSigConfig(createSignatureDraft(signatureData?.signatureConfig))
        setSigEditMode(true)
    }

    const manualResponses = (responses || []).filter((config) => !config.recorded).sort((a, b) => a.priority - b.priority)
    const recordedResponses = (responses || []).filter((config) => config.recorded)

    if (opLoading || respLoading || specLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (!operation) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Operation not found
                </div>
            </div>
        )
    }

    return (
        <div className="p-8">
            {/* Header */}
            <div className="mb-8">
                <Link
                    to={`/specs/${operation.specId}`}
                    className="inline-flex items-center text-sm text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200 mb-4"
                >
                    <ArrowLeft className="w-4 h-4 mr-1" />
                    Back to Specification
                </Link>

                <div className="flex flex-wrap items-center gap-2">
                    <span className={clsx(
                        'px-3 py-1.5 rounded text-sm font-bold uppercase',
                        methodColors[operation.method] || 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                    )}>
                        {operation.method}
                    </span>
                    <h1 className="text-2xl font-mono font-bold text-gray-900 dark:text-slate-100 min-w-0">
                        {operation.path}
                    </h1>
                </div>
                {operation.summary && (
                    <p className="text-gray-500 dark:text-slate-400 mt-2">{operation.summary}</p>
                )}
                <p className="text-sm text-gray-400 dark:text-slate-500 mt-1">
                    Full path: <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1 rounded">{operation.fullPath}</code>
                </p>
            </div>

            {/* Processing Pipeline */}
            <PipelinePanel scope="operation" scopeId={operationId!} />

            {/* Response Configurations */}
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Manual Response Configurations</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-1">
                            Configure curated mock responses with conditions and priorities
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        <div className="relative group">
                            <button
                                onClick={() => aiConfigured && setShowAIModal(true)}
                                disabled={!aiConfigured}
                                className={clsx(
                                    "flex items-center px-4 py-2 rounded-lg transition-colors",
                                    aiConfigured
                                        ? "bg-purple-600 text-white hover:bg-purple-700"
                                        : "bg-purple-200 dark:bg-purple-900/30 text-purple-400 dark:text-purple-600 cursor-not-allowed"
                                )}
                            >
                                <Sparkles className="w-4 h-4 mr-2" />
                                Generate with AI
                            </button>
                            {!aiConfigured && (
                                <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-1.5 text-xs text-white bg-gray-900 dark:bg-slate-700 rounded-lg whitespace-nowrap opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity z-10">
                                    {aiProviderLabel} is not configured
                                    <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900 dark:border-t-slate-700" />
                                </div>
                            )}
                        </div>
                        <Link
                            to={`/operations/${operationId}/responses/new`}
                            className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                        >
                            <Plus className="w-5 h-5 mr-2" />
                            Add Response
                        </Link>
                        <Link
                            to={`/operations/${operationId}/responses/new?kind=collection`}
                            className="flex items-center px-4 py-2 border border-teal-200 text-teal-700 dark:border-teal-800 dark:text-teal-300 rounded-lg hover:bg-teal-50 dark:hover:bg-teal-900/20 transition-colors"
                            title="Fill a response body from a collection query, matched by naming convention"
                        >
                            <Database className="w-4 h-4 mr-2" />
                            Add Collection Response
                        </Link>
                        <button
                            onClick={() => {
                                setImportFeedback(null)
                                setShowImportModal(true)
                            }}
                            className="px-4 py-2 border border-primary-200 text-primary-700 dark:border-primary-800 dark:text-primary-300 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors"
                        >
                            Import Response
                        </button>
                    </div>
                </div>

                {importFeedback && (
                    <div
                        className={clsx(
                            'mx-6 mt-4 p-3 rounded-lg text-sm flex items-start gap-2',
                            importFeedback.type === 'success'
                                ? 'bg-green-50 text-green-700 border border-green-200 dark:bg-green-950/30 dark:text-green-300 dark:border-green-900/40'
                                : 'bg-red-50 text-red-700 border border-red-200 dark:bg-red-950/30 dark:text-red-300 dark:border-red-900/40'
                        )}
                    >
                        {importFeedback.type === 'success' ? (
                            <Check className="w-4 h-4 mt-0.5 shrink-0" />
                        ) : (
                            <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                        )}
                        <span>{importFeedback.message}</span>
                    </div>
                )}

                <ResponseConfigList
                    operationId={operationId!}
                    configs={manualResponses}
                    emptyTitle="No manual response configurations yet"
                    emptyDescription="Add a curated mock response here, while AI-generated and proxy-recorded responses stay on the separate generated responses page."
                    emptyAction={{
                        to: `/operations/${operationId}/responses/new`,
                        label: 'Add First Response',
                    }}
                    enableManualActions
                />
            </div>

            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 flex flex-wrap items-center justify-between gap-4">
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Generated Responses</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-1">
                            Replayable responses generated by AI or proxy fallback are shown on a separate page to keep this operation focused on manual mock setup.
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        {spec?.mode === 'proxy' && (
                            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">
                                <Radio className="w-3 h-3 animate-pulse" />
                                Recording
                            </span>
                        )}
                        <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300">
                            {recordedResponses.length} recorded
                        </span>
                        <Link
                            to={`/operations/${operationId}/recorded-responses`}
                            className="inline-flex items-center px-4 py-2 border border-violet-200 text-violet-700 dark:border-violet-800 dark:text-violet-300 rounded-lg hover:bg-violet-50 dark:hover:bg-violet-900/20 transition-colors"
                        >
                            View Generated Responses
                        </Link>
                    </div>
                </div>
            </div>

            {/* Example Response Fallback Info */}
            {operation.exampleResponse && (
                <div className={clsx(
                    "mt-6 rounded-xl p-6",
                    spec?.useExampleFallback
                        ? "bg-amber-50 border border-amber-200 dark:bg-amber-950/30 dark:border-amber-900/40"
                        : "bg-gray-50 border border-gray-200 dark:bg-slate-900 dark:border-slate-800"
                )}>
                    <div className="flex items-start">
                        <Sparkles className={clsx(
                            "w-5 h-5 mt-0.5 mr-3 flex-shrink-0",
                            spec?.useExampleFallback ? "text-amber-600" : "text-gray-400 dark:text-slate-500"
                        )} />
                        <div className="flex-1">
                            <div className="flex items-center justify-between mb-1">
                                <h3 className={clsx(
                                    "text-sm font-semibold",
                                    spec?.useExampleFallback ? "text-amber-800 dark:text-amber-200" : "text-gray-600 dark:text-slate-300"
                                )}>
                                    Fallback: Example Response from Spec
                                </h3>
                                {!spec?.useExampleFallback && (
                                    <span className="text-xs bg-gray-200 text-gray-600 dark:bg-slate-800 dark:text-slate-300 px-2 py-0.5 rounded">
                                        Disabled at spec level
                                    </span>
                                )}
                            </div>
                            <p className={clsx(
                                "text-sm mb-4",
                                spec?.useExampleFallback ? "text-amber-700 dark:text-amber-300" : "text-gray-500 dark:text-slate-400"
                            )}>
                                {spec?.useExampleFallback
                                    ? "If no configured response matches, this example response from the OpenAPI spec will be returned."
                                    : "Example fallback is disabled for this spec. Enable it in the spec settings to use this response."}
                            </p>
                            <div className={clsx(
                                "bg-white dark:bg-slate-900 rounded-lg p-4 space-y-3",
                                spec?.useExampleFallback ? "border border-amber-200 dark:border-amber-900/40" : "border border-gray-200 dark:border-slate-800 opacity-60"
                            )}>
                                <div className="flex items-center gap-4 text-sm">
                                    <span className={spec?.useExampleFallback ? "text-amber-700 dark:text-amber-300" : "text-gray-500 dark:text-slate-400"}>Status:</span>
                                    <span className={clsx(
                                        'px-2 py-0.5 rounded text-xs font-medium',
                                        operation.exampleResponse.statusCode >= 200 && operation.exampleResponse.statusCode < 300
                                            ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                            : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                    )}>
                                        {operation.exampleResponse.statusCode}
                                    </span>
                                </div>
                                {operation.exampleResponse.headers && Object.keys(operation.exampleResponse.headers).length > 0 && (
                                    <div className="text-sm">
                                        <span className={spec?.useExampleFallback ? "text-amber-700 dark:text-amber-300" : "text-gray-500 dark:text-slate-400"}>Headers:</span>
                                        <div className={clsx(
                                            "mt-1 font-mono text-xs rounded p-2",
                                            spec?.useExampleFallback ? "bg-amber-50 dark:bg-amber-950/30" : "bg-gray-50 dark:bg-slate-800"
                                        )}>
                                            {Object.entries(operation.exampleResponse.headers).map(([key, value]) => (
                                                <div key={key}><span className={spec?.useExampleFallback ? "text-amber-600" : "text-gray-400 dark:text-slate-500"}>{key}:</span> {value}</div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                                {operation.exampleResponse.body && (
                                    <div className="text-sm">
                                        <span className={spec?.useExampleFallback ? "text-amber-700 dark:text-amber-300" : "text-gray-500 dark:text-slate-400"}>Body:</span>
                                        <pre className="mt-1 bg-gray-900 dark:bg-slate-950 text-gray-100 rounded p-3 text-xs overflow-x-auto max-h-48 whitespace-pre">
                                            {(() => { try { return JSON.stringify(JSON.parse(operation.exampleResponse.body), null, 2); } catch { return operation.exampleResponse.body; } })()}
                                        </pre>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Signature Configuration */}
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-violet-100 dark:bg-violet-900/30 rounded-lg">
                            <Fingerprint className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                        </div>
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Signature Configuration</h2>
                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                                Controls what parts of the request are hashed to identify unique responses
                            </p>
                        </div>
                    </div>
                    {!sigEditMode && (
                        <button
                            onClick={openSigEdit}
                            className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-violet-600 border border-violet-200 dark:border-violet-800 rounded-lg hover:bg-violet-50 dark:hover:bg-violet-900/30 transition-colors"
                        >
                            <Edit2 className="w-4 h-4" />
                            Configure
                        </button>
                    )}
                </div>

                <div className="p-6">
                    {!sigEditMode ? (
                        <div className="space-y-3">
                            <div className="flex flex-wrap gap-2 text-sm">
                                <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                    Path params: {(effectiveSignatureConfig?.pathParams?.length ?? 0) === 0 ? 'None' : effectiveSignatureConfig?.pathParams?.join(', ')}
                                </span>
                                <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                    Query params: {(effectiveSignatureConfig?.queryParams?.length ?? 0) === 0 ? 'None' : effectiveSignatureConfig?.queryParams?.join(', ')}
                                </span>
                                <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                    Headers: {(effectiveSignatureConfig?.headers?.length ?? 0) === 0 ? 'None' : effectiveSignatureConfig?.headers?.join(', ')}
                                </span>
                                <span className={clsx(
                                    'inline-flex items-center px-2.5 py-1 rounded-full text-xs',
                                    (effectiveSignatureConfig?.includeBody ?? false)
                                        ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                        : 'bg-gray-100 text-gray-500 dark:bg-slate-800 dark:text-slate-400'
                                )}>
                                    Body: {(effectiveSignatureConfig?.includeBody ?? false) ? ((effectiveSignatureConfig?.bodyJsonPaths?.length ?? 0) > 0 ? 'Selected paths' : 'Full body') : 'Excluded'}
                                </span>
                                {(effectiveSignatureConfig?.bodyJsonPaths?.length ?? 0) > 0 && (
                                    <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                        Body paths: {effectiveSignatureConfig?.bodyJsonPaths?.join(', ')}
                                    </span>
                                )}
                            </div>
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-slate-700 px-3 py-2 text-xs text-gray-500 dark:text-slate-400">
                                {(signatureData?.signatureConfig?.headersConfigured ?? false)
                                    ? 'Headers are explicitly overridden for this operation.'
                                    : `Headers are using defaults from declared operation headers and spec-level signature headers (${(defaultSignatureConfig?.headers ?? []).join(', ') || 'none'}).`}
                            </div>
                        </div>
                    ) : sigConfig ? (
                        <div className="space-y-5">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                                    Path Parameters
                                </label>
                                {availableInputs.pathParams.length > 0 ? (
                                    <>
                                        <p className="mb-2 text-xs text-gray-500 dark:text-slate-400">
                                            Leave this empty to include all declared path parameters.
                                        </p>
                                        <div className="flex flex-wrap gap-2">
                                            {availableInputs.pathParams.map((param) => (
                                                <button
                                                    key={param}
                                                    type="button"
                                                    onClick={() => setSigConfig((current) => current ? ({
                                                        ...current,
                                                        pathParams: current.pathParams.includes(param)
                                                            ? current.pathParams.filter((item) => item !== param)
                                                            : [...current.pathParams, param],
                                                    }) : current)}
                                                    className={clsx(
                                                        'px-3 py-1 rounded-full text-xs border font-mono transition-colors',
                                                        sigConfig.pathParams.includes(param)
                                                            ? 'bg-violet-100 border-violet-300 text-violet-700 dark:bg-violet-900/40 dark:border-violet-700 dark:text-violet-300'
                                                            : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                                    )}
                                                >
                                                    {param}
                                                </button>
                                            ))}
                                        </div>
                                    </>
                                ) : (
                                    <p className="text-sm text-gray-400 dark:text-slate-500 italic">No declared path parameters for this operation</p>
                                )}
                            </div>

                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                                    Query Parameters
                                </label>
                                <p className="mb-2 text-xs text-gray-500 dark:text-slate-400">
                                    Leave this empty to include all declared query parameters.
                                </p>
                                <div className="flex flex-wrap gap-2 mb-3">
                                    {availableInputs.queryParams.map((param) => (
                                        <button
                                            key={param}
                                            type="button"
                                            onClick={() => setSigConfig((current) => current ? ({
                                                ...current,
                                                queryParams: current.queryParams.includes(param)
                                                    ? current.queryParams.filter((item) => item !== param)
                                                    : [...current.queryParams, param],
                                            }) : current)}
                                            className={clsx(
                                                'px-3 py-1 rounded-full text-xs border font-mono transition-colors',
                                                sigConfig.queryParams.includes(param)
                                                    ? 'bg-violet-100 border-violet-300 text-violet-700 dark:bg-violet-900/40 dark:border-violet-700 dark:text-violet-300'
                                                    : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                            )}
                                        >
                                            {param}
                                        </button>
                                    ))}
                                </div>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={newQueryParam}
                                        onChange={e => setNewQueryParam(e.target.value)}
                                        placeholder="Add custom query param…"
                                        onKeyDown={e => {
                                            if (e.key === 'Enter' && newQueryParam.trim()) {
                                                setSigConfig(current => current ? ({
                                                    ...current,
                                                    queryParams: [...current.queryParams, newQueryParam.trim()]
                                                }) : current)
                                                setNewQueryParam('')
                                            }
                                        }}
                                        className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => {
                                            if (newQueryParam.trim()) {
                                                setSigConfig(current => current ? ({
                                                    ...current,
                                                    queryParams: [...current.queryParams, newQueryParam.trim()]
                                                }) : current)
                                                setNewQueryParam('')
                                            }
                                        }}
                                        className="px-3 py-1.5 text-sm bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
                                    >
                                        <Plus className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>

                            <div>
                                <div className="flex items-center justify-between gap-3 mb-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-slate-300">
                                        Headers
                                    </label>
                                    <button
                                        type="button"
                                        onClick={() => setSigConfig((current) => current ? ({
                                            ...current,
                                            headersConfigured: !(current.headersConfigured ?? false),
                                            headers: current.headersConfigured ? [] : current.headers,
                                        }) : current)}
                                        className="px-2.5 py-1 text-xs rounded border border-gray-200 dark:border-slate-700 text-gray-600 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-slate-800"
                                    >
                                        {sigConfig.headersConfigured ? 'Use defaults' : 'Customize headers'}
                                    </button>
                                </div>
                                {!sigConfig.headersConfigured ? (
                                    <div className="rounded-lg border border-dashed border-gray-200 dark:border-slate-700 px-3 py-2 text-xs text-gray-500 dark:text-slate-400">
                                        Default headers: {(defaultSignatureConfig?.headers ?? []).join(', ') || 'none'}
                                    </div>
                                ) : (
                                    <>
                                        <p className="mb-2 text-xs text-gray-500 dark:text-slate-400">
                                            Explicit override. Leave the list empty to include no headers for this operation.
                                        </p>
                                        <div className="flex flex-wrap gap-2 mb-2">
                                            {(sigConfig.headers ?? []).map(h => (
                                                <span key={h} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 font-mono">
                                                    {h}
                                                    <button
                                                        onClick={() => setSigConfig(current => current ? ({ ...current, headers: current.headers.filter(x => x !== h) }) : current)}
                                                        className="hover:text-red-500"
                                                    >
                                                        <X className="w-3 h-3" />
                                                    </button>
                                                </span>
                                            ))}
                                        </div>
                                        <div className="flex flex-wrap gap-2 mb-3">
                                            {availableInputs.headerParams.map((header) => (
                                                <button
                                                    key={header}
                                                    type="button"
                                                    onClick={() => setSigConfig((current) => current ? ({
                                                        ...current,
                                                        headers: current.headers.includes(header)
                                                            ? current.headers.filter((item) => item !== header)
                                                            : [...current.headers, header],
                                                    }) : current)}
                                                    className={clsx(
                                                        'px-3 py-1 rounded-full text-xs border font-mono transition-colors',
                                                        sigConfig.headers.includes(header)
                                                            ? 'bg-blue-100 border-blue-300 text-blue-700 dark:bg-blue-900/40 dark:border-blue-700 dark:text-blue-300'
                                                            : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                                    )}
                                                >
                                                    {header}
                                                </button>
                                            ))}
                                        </div>
                                        <div className="flex gap-2">
                                            <input
                                                type="text"
                                                value={newHeader}
                                                onChange={e => setNewHeader(e.target.value)}
                                                placeholder="Add header name…"
                                                onKeyDown={e => {
                                                    if (e.key === 'Enter' && newHeader.trim()) {
                                                        setSigConfig(current => current ? ({
                                                            ...current,
                                                            headers: [...current.headers, newHeader.trim()]
                                                        }) : current)
                                                        setNewHeader('')
                                                    }
                                                }}
                                                className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500"
                                            />
                                            <button
                                                type="button"
                                                onClick={() => {
                                                    if (newHeader.trim()) {
                                                        setSigConfig(current => current ? ({
                                                            ...current,
                                                            headers: [...current.headers, newHeader.trim()]
                                                        }) : current)
                                                        setNewHeader('')
                                                    }
                                                }}
                                                className="px-3 py-1.5 text-sm bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
                                            >
                                                <Plus className="w-4 h-4" />
                                            </button>
                                        </div>
                                    </>
                                )}
                            </div>

                            <div>
                                <label className="flex items-center gap-2 cursor-pointer select-none">
                                    <input
                                        type="checkbox"
                                        checked={bodyIncluded}
                                        disabled={!availableInputs.hasBody}
                                        onChange={e => setSigConfig(current => current ? ({
                                            ...current,
                                            includeBody: e.target.checked ? true : false,
                                            bodyJsonPaths: e.target.checked ? current.bodyJsonPaths : [],
                                        }) : current)}
                                        className="w-4 h-4 text-violet-600 border-gray-300 rounded focus:ring-violet-500"
                                    />
                                    <span className="text-sm font-medium text-gray-700 dark:text-slate-300">Include request body in signature</span>
                                </label>
                                {!availableInputs.hasBody && (
                                    <p className="mt-2 text-sm text-gray-400 dark:text-slate-500 italic">This operation does not declare a request body</p>
                                )}
                            </div>

                            {availableInputs.hasBody && bodyIncluded && (
                                <div>
                                    <div className="flex items-center justify-between gap-3 mb-2">
                                        <label className="text-sm font-medium text-gray-700 dark:text-slate-300">
                                            Body JSON Paths
                                        </label>
                                        <button
                                            type="button"
                                            onClick={() => setSigConfig(current => current ? ({ ...current, includeBody: true, bodyJsonPaths: [] }) : current)}
                                            className="px-2.5 py-1 text-xs rounded border border-gray-200 dark:border-slate-700 text-gray-600 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-slate-800"
                                        >
                                            Use full body
                                        </button>
                                    </div>
                                    <p className="mb-2 text-xs text-gray-500 dark:text-slate-400">
                                        Leave this empty to hash the full body. Add paths to hash only selected values.
                                    </p>
                                    <div className="flex flex-wrap gap-2 mb-3">
                                        {availableInputs.bodyFields.map((path) => (
                                            <button
                                                key={path}
                                                type="button"
                                                onClick={() => setSigConfig((current) => current ? ({
                                                    ...current,
                                                    includeBody: true,
                                                    bodyJsonPaths: current.bodyJsonPaths.includes(path)
                                                        ? current.bodyJsonPaths.filter((item) => item !== path)
                                                        : [...current.bodyJsonPaths, path],
                                                }) : current)}
                                                className={clsx(
                                                    'px-3 py-1 rounded-full text-xs border font-mono transition-colors',
                                                    sigConfig.bodyJsonPaths.includes(path)
                                                        ? 'bg-green-100 border-green-300 text-green-700 dark:bg-green-900/40 dark:border-green-700 dark:text-green-300'
                                                        : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                                )}
                                            >
                                                {path}
                                            </button>
                                        ))}
                                    </div>
                                    <div className="flex flex-wrap gap-2 mb-2">
                                        {(sigConfig.bodyJsonPaths ?? []).map(p => (
                                            <span key={p} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 font-mono">
                                                {p}
                                                <button
                                                    onClick={() => setSigConfig(current => current ? ({ ...current, bodyJsonPaths: current.bodyJsonPaths.filter(x => x !== p) }) : current)}
                                                    className="hover:text-red-500"
                                                >
                                                    <X className="w-3 h-3" />
                                                </button>
                                            </span>
                                        ))}
                                    </div>
                                    <div className="flex gap-2">
                                        <input
                                            type="text"
                                            value={newBodyPath}
                                            onChange={e => setNewBodyPath(e.target.value)}
                                            placeholder="e.g. user.id or items.0.name"
                                            onKeyDown={e => {
                                                if (e.key === 'Enter' && newBodyPath.trim()) {
                                                    setSigConfig(current => current ? ({
                                                        ...current,
                                                        includeBody: true,
                                                        bodyJsonPaths: [...current.bodyJsonPaths, newBodyPath.trim()]
                                                    }) : current)
                                                    setNewBodyPath('')
                                                }
                                            }}
                                            className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500 font-mono"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => {
                                                if (newBodyPath.trim()) {
                                                    setSigConfig(current => current ? ({
                                                        ...current,
                                                        includeBody: true,
                                                        bodyJsonPaths: [...current.bodyJsonPaths, newBodyPath.trim()]
                                                    }) : current)
                                                    setNewBodyPath('')
                                                }
                                            }}
                                            className="px-3 py-1.5 text-sm bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
                                        >
                                            <Plus className="w-4 h-4" />
                                        </button>
                                    </div>
                                </div>
                            )}

                            <div className="flex gap-3 pt-2 border-t border-gray-100 dark:border-slate-800">
                                <button
                                    onClick={() => updateSignatureMutation.mutate(normalizeSignatureDraftForSave(sigConfig))}
                                    disabled={updateSignatureMutation.isPending}
                                    className="px-4 py-2 bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50 transition-colors text-sm"
                                >
                                    {updateSignatureMutation.isPending ? 'Saving…' : 'Save'}
                                </button>
                                <button
                                    onClick={() => { setSigEditMode(false); setSigConfig(null) }}
                                    className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors text-sm"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={() => updateSignatureMutation.mutate(null)}
                                    disabled={updateSignatureMutation.isPending}
                                    className="ml-auto text-xs text-gray-400 hover:text-red-500 transition-colors"
                                    title="Reset to defaults"
                                >
                                    Reset to defaults
                                </button>
                            </div>
                        </div>
                    ) : (
                        <div className="text-sm text-gray-500 dark:text-slate-400">
                            Loading signature configuration…
                        </div>
                    )}
                </div>
            </div>

            {/* Response Matching Info */}
            <div className="mt-6 bg-blue-50 border border-blue-200 dark:bg-blue-950/30 dark:border-blue-900/40 rounded-xl p-4">
                <div className="flex items-start">
                    <Info className="w-5 h-5 text-blue-600 mt-0.5 mr-3 flex-shrink-0" />
                    <div className="text-sm text-blue-700 dark:text-blue-300">
                        <strong>Response Matching Order:</strong>
                        <ol className="list-decimal list-inside mt-1 space-y-0.5">
                            <li>Enabled configured responses are checked first, lowest priority number first; the first condition match wins.</li>
                            <li>If none match, recorded proxy responses are checked next using the same order.</li>
                            <li>In AI mode, if nothing matches, a response can be generated from the operation and active AI scenario.</li>
                            <li>In proxy mode, if nothing matches, the request is sent to the backend and the response may be recorded for reuse.</li>
                            {operation.exampleResponse && spec?.useExampleFallback && (
                                <li>In standard mode, the OpenAPI example response is used next (status {operation.exampleResponse.statusCode}).</li>
                            )}
                            <li>If nothing matches, the operation returns 404.</li>
                        </ol>
                    </div>
                </div>
            </div>

            {/* Editor Modal */}

            {/* AI Generate Modal */}
            {showAIModal && operation && (
                <AIGenerateModal
                    operationId={operationId!}
                    operationMethod={operation.method}
                    operationPath={operation.path}
                    onClose={() => setShowAIModal(false)}
                />
            )}
            {showImportModal && (
                <ResponseImportModal
                    onClose={() => setShowImportModal(false)}
                    onImport={(input) => importResponseMutation.mutate(input)}
                    isSubmitting={importResponseMutation.isPending}
                />
            )}
        </div>
    )
}
