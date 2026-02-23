import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    ArrowLeft,
    Plus,
    Trash2,
    GripVertical,
    ToggleLeft,
    ToggleRight,
    Edit2,
    ChevronDown,
    ChevronRight,
    Sparkles,
    Info,
    Fingerprint,
    X,
    Radio
} from 'lucide-react'
import clsx from 'clsx'
import { operationsApi, responsesApi, specsApi } from '../services/api'
import type { Operation, ResponseConfig, Spec, SignatureConfig } from '../types'
import ScriptBindingsPanel from './ScriptManager/ScriptBindingsPanel'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
}

export default function OperationDetail() {
    const { operationId } = useParams<{ operationId: string }>()
    const [expandedConfig, setExpandedConfig] = useState<string | null>(null)
    const queryClient = useQueryClient()

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

    const deleteMutation = useMutation({
        mutationFn: responsesApi.delete,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
        },
    })

    const toggleMutation = useMutation({
        mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
            responsesApi.update(id, { enabled: !enabled }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
        },
    })

    // Signature config state
    const [sigEditMode, setSigEditMode] = useState(false)
    const [sigConfig, setSigConfig] = useState<SignatureConfig | null>(null)
    const [newHeader, setNewHeader] = useState('')
    const [newQueryParam, setNewQueryParam] = useState('')
    const [newBodyPath, setNewBodyPath] = useState('')

    const signatureQuery = useQuery({
        queryKey: ['signature', operationId],
        queryFn: () => operationsApi.getSignatureConfig(operationId!),
        enabled: !!operationId,
    })

    const updateSignatureMutation = useMutation({
        mutationFn: (cfg: SignatureConfig | null) =>
            operationsApi.updateSignatureConfig(operationId!, cfg),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['signature', operationId] })
            queryClient.invalidateQueries({ queryKey: ['operation', operationId] })
            setSigEditMode(false)
        },
    })

    // Extract path params from operation path (e.g. /pets/{id} → ['id'])
    const pathParamNames = operation
        ? [...(operation.path.matchAll(/\{([^}]+)\}/g))].map(m => m[1])
        : []

    const openSigEdit = () => {
        setSigConfig(signatureQuery.data?.signatureConfig ?? null)
        setSigEditMode(true)
    }

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

                <div className="flex items-center">
                    <span className={clsx(
                        'px-3 py-1.5 rounded text-sm font-bold uppercase',
                        methodColors[operation.method] || 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                    )}>
                        {operation.method}
                    </span>
                    <h1 className="text-2xl font-mono font-bold text-gray-900 dark:text-slate-100 ml-4">
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

            {/* Response Configurations */}
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Response Configurations</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-1">
                            Configure mock responses with conditions and priorities
                        </p>
                    </div>
                    <Link
                        to={`/operations/${operationId}/responses/new`}
                        className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        Add Response
                    </Link>
                </div>

                {responses && responses.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {responses.map((config, index) => (
                            <div key={config.id} className="p-4">
                                <div
                                    className="flex items-center justify-between cursor-pointer"
                                    onClick={() => setExpandedConfig(
                                        expandedConfig === config.id ? null : config.id
                                    )}
                                >
                                    <div className="flex items-center">
                                        <GripVertical className="w-5 h-5 text-gray-300 dark:text-slate-600 mr-3" />
                                        <span className="w-8 h-8 flex items-center justify-center bg-gray-100 dark:bg-slate-800 rounded-full text-sm font-medium text-gray-600 dark:text-slate-300 mr-3">
                                            {index + 1}
                                        </span>
                                        <div>
                                            <div className="flex items-center">
                                                <span className="font-medium text-gray-900 dark:text-slate-100">{config.name}</span>
                                                {config.recorded && (
                                                    <span className="ml-2 inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">
                                                        <Radio className="w-2.5 h-2.5" />
                                                        Recorded
                                                    </span>
                                                )}
                                                <span className="ml-2 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-slate-300">
                                                    {config.tag || 'default'}
                                                </span>
                                                <span className={clsx(
                                                    'ml-3 px-2 py-0.5 rounded text-xs font-medium',
                                                    config.statusCode >= 200 && config.statusCode < 300
                                                        ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                                        : config.statusCode >= 400
                                                            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                                            : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                                )}>
                                                    {config.statusCode}
                                                </span>
                                                {config.conditions.length > 0 && (
                                                    <span className="ml-2 text-xs text-gray-400 dark:text-slate-500">
                                                        {config.conditions.length} condition{config.conditions.length !== 1 ? 's' : ''}
                                                    </span>
                                                )}
                                            </div>
                                            {config.description && (
                                                <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">{config.description}</p>
                                            )}
                                        </div>
                                    </div>

                                    <div className="flex items-center gap-2">
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                toggleMutation.mutate({ id: config.id, enabled: config.enabled })
                                            }}
                                            className={clsx(
                                                'p-2 rounded-lg transition-colors',
                                                config.enabled
                                                    ? 'text-green-600 hover:bg-green-50 dark:hover:bg-green-950/40'
                                                    : 'text-gray-400 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-slate-800'
                                            )}
                                            title={config.enabled ? 'Disable' : 'Enable'}
                                        >
                                            {config.enabled ? (
                                                <ToggleRight className="w-5 h-5" />
                                            ) : (
                                                <ToggleLeft className="w-5 h-5" />
                                            )}
                                        </button>
                                        <Link
                                            to={`/responses/${config.id}/edit`}
                                            onClick={(e) => e.stopPropagation()}
                                            className="p-2 text-gray-400 dark:text-slate-500 hover:text-primary-600 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/30 transition-colors"
                                            title="Edit"
                                        >
                                            <Edit2 className="w-5 h-5" />
                                        </Link>
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                if (confirm('Delete this response configuration?')) {
                                                    deleteMutation.mutate(config.id)
                                                }
                                            }}
                                            className="p-2 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                            title="Delete"
                                        >
                                            <Trash2 className="w-5 h-5" />
                                        </button>
                                        {expandedConfig === config.id ? (
                                            <ChevronDown className="w-5 h-5 text-gray-400 dark:text-slate-500" />
                                        ) : (
                                            <ChevronRight className="w-5 h-5 text-gray-400 dark:text-slate-500" />
                                        )}
                                    </div>
                                </div>

                                {/* Expanded Details */}
                                {expandedConfig === config.id && (
                                    <div className="mt-4 ml-16 space-y-4">
                                        {/* Conditions */}
                                        {config.conditions.length > 0 && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Conditions</h4>
                                                <div className="space-y-2">
                                                    {config.conditions.map((cond, i) => (
                                                        <div key={i} className="flex items-center text-sm bg-gray-50 dark:bg-slate-800 rounded px-3 py-2">
                                                            <span className={clsx(
                                                                'font-mono',
                                                                cond.source === 'signature'
                                                                    ? 'text-violet-600 dark:text-violet-400'
                                                                    : 'text-purple-600'
                                                            )}>{cond.source}</span>
                                                            {cond.key && (
                                                                <>
                                                                    <span className="mx-2 text-gray-400 dark:text-slate-500">.</span>
                                                                    <span className="text-blue-600 font-mono">{cond.key}</span>
                                                                </>
                                                            )}
                                                            <span className="mx-2 text-gray-500 dark:text-slate-400">{cond.operator}</span>
                                                            <span className="text-green-600 font-mono">"{cond.source === 'signature' ? cond.value.substring(0, 8) + '…' : cond.value}"</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        )}

                                        {/* Headers */}
                                        {Object.keys(config.headers).length > 0 && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Headers</h4>
                                                <div className="bg-gray-50 dark:bg-slate-800 rounded p-3 font-mono text-sm">
                                                    {Object.entries(config.headers).map(([key, value]) => (
                                                        <div key={key}>
                                                            <span className="text-purple-600">{key}</span>
                                                            <span className="text-gray-400 dark:text-slate-500">: </span>
                                                            <span className="text-green-600">{value}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        )}

                                        {/* Body */}
                                        {config.body && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Body</h4>
                                                <pre className="bg-gray-900 text-gray-100 rounded p-4 text-sm overflow-x-auto">
                                                    {config.body}
                                                </pre>
                                            </div>
                                        )}

                                        {/* Delay */}
                                        {config.delay > 0 && (
                                            <div className="text-sm text-gray-500 dark:text-slate-400">
                                                Response delay: <span className="font-medium">{config.delay}ms</span>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="p-12 text-center">
                        <p className="text-gray-500 dark:text-slate-400 mb-4">No response configurations yet</p>
                        <Link
                            to={`/operations/${operationId}/responses/new`}
                            className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                        >
                            <Plus className="w-5 h-5 mr-2" />
                            Add First Response
                        </Link>
                    </div>
                )}
            </div>

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
                        /* Read-only summary */
                        <div className="flex flex-wrap gap-2 text-sm">
                            <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                Path params: {(signatureQuery.data?.signatureConfig?.pathParams?.length ?? 0) === 0 ? 'All' : signatureQuery.data?.signatureConfig?.pathParams?.join(', ')}
                            </span>
                            <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                Query params: {(signatureQuery.data?.signatureConfig?.queryParams?.length ?? 0) === 0 ? 'All' : signatureQuery.data?.signatureConfig?.queryParams?.join(', ')}
                            </span>
                            <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                Headers: {(signatureQuery.data?.signatureConfig?.headers?.length ?? 0) === 0 ? 'None' : signatureQuery.data?.signatureConfig?.headers?.join(', ')}
                            </span>
                            <span className={clsx(
                                'inline-flex items-center px-2.5 py-1 rounded-full text-xs',
                                (signatureQuery.data?.signatureConfig?.includeBody ?? true)
                                    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    : 'bg-gray-100 text-gray-500 dark:bg-slate-800 dark:text-slate-400'
                            )}>
                                Body: {(signatureQuery.data?.signatureConfig?.includeBody ?? true) ? 'Included' : 'Excluded'}
                            </span>
                            {(signatureQuery.data?.signatureConfig?.bodyJsonPaths?.length ?? 0) > 0 && (
                                <span className="inline-flex items-center px-2.5 py-1 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-700 dark:text-slate-300 text-xs">
                                    Body paths: {signatureQuery.data?.signatureConfig?.bodyJsonPaths?.join(', ')}
                                </span>
                            )}
                        </div>
                    ) : (
                        /* Edit form */
                        <div className="space-y-5">
                            {/* Path Params */}
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                                    Path Parameters
                                </label>
                                {pathParamNames.length > 0 ? (
                                    <div className="flex flex-wrap gap-2">
                                        <button
                                            type="button"
                                            onClick={() => setSigConfig(c => ({ ...(c ?? { queryParams: [], headers: [], includeBody: true, bodyJsonPaths: [] }), pathParams: [] }))}
                                            className={clsx(
                                                'px-3 py-1 rounded-full text-xs border transition-colors',
                                                (sigConfig?.pathParams?.length ?? 0) === 0
                                                    ? 'bg-violet-100 border-violet-300 text-violet-700 dark:bg-violet-900/40 dark:border-violet-700 dark:text-violet-300'
                                                    : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                            )}
                                        >
                                            All
                                        </button>
                                        {pathParamNames.map(p => (
                                            <button
                                                key={p}
                                                type="button"
                                                onClick={() => setSigConfig(c => {
                                                    const base: SignatureConfig = c ?? { pathParams: [], queryParams: [], headers: [], includeBody: true, bodyJsonPaths: [] }
                                                    const current = base.pathParams ?? []
                                                    return {
                                                        ...base,
                                                        pathParams: current.includes(p)
                                                            ? current.filter((x: string) => x !== p)
                                                            : [...current, p]
                                                    }
                                                })}
                                                className={clsx(
                                                    'px-3 py-1 rounded-full text-xs border font-mono transition-colors',
                                                    sigConfig?.pathParams?.includes(p)
                                                        ? 'bg-violet-100 border-violet-300 text-violet-700 dark:bg-violet-900/40 dark:border-violet-700 dark:text-violet-300'
                                                        : 'bg-white border-gray-200 text-gray-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300'
                                                )}
                                            >
                                                {p}
                                            </button>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="text-sm text-gray-400 dark:text-slate-500 italic">No path parameters in this operation</p>
                                )}
                            </div>

                            {/* Query Params */}
                            <div>
                                <div className="flex items-center justify-between mb-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-slate-300">
                                        Query Parameters
                                    </label>
                                    <button
                                        type="button"
                                        onClick={() => setSigConfig(c => ({ ...(c ?? { pathParams: [], headers: [], includeBody: true, bodyJsonPaths: [] }), queryParams: [] }))}
                                        className={clsx(
                                            'px-2 py-0.5 rounded text-xs border transition-colors',
                                            (sigConfig?.queryParams?.length ?? 0) === 0
                                                ? 'bg-violet-100 border-violet-300 text-violet-700 dark:bg-violet-900/40 dark:border-violet-700 dark:text-violet-300'
                                                : 'bg-white border-gray-200 text-gray-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-400'
                                        )}
                                    >
                                        Include All
                                    </button>
                                </div>
                                <div className="flex flex-wrap gap-2 mb-2">
                                    {(sigConfig?.queryParams ?? []).map(q => (
                                        <span key={q} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300 font-mono">
                                            {q}
                                            <button
                                                onClick={() => setSigConfig(c => ({ ...c!, queryParams: (c?.queryParams ?? []).filter(x => x !== q) }))}
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
                                        value={newQueryParam}
                                        onChange={e => setNewQueryParam(e.target.value)}
                                        placeholder="Add query param…"
                                        onKeyDown={e => {
                                            if (e.key === 'Enter' && newQueryParam.trim()) {
                                                setSigConfig(c => ({ ...(c ?? { pathParams: [], headers: [], includeBody: true, bodyJsonPaths: [] }), queryParams: [...(c?.queryParams ?? []), newQueryParam.trim()] }))
                                                setNewQueryParam('')
                                            }
                                        }}
                                        className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => {
                                            if (newQueryParam.trim()) {
                                                setSigConfig(c => ({ ...(c ?? { pathParams: [], headers: [], includeBody: true, bodyJsonPaths: [] }), queryParams: [...(c?.queryParams ?? []), newQueryParam.trim()] }))
                                                setNewQueryParam('')
                                            }
                                        }}
                                        className="px-3 py-1.5 text-sm bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
                                    >
                                        <Plus className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>

                            {/* Headers */}
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                                    Headers <span className="text-xs text-gray-400 dark:text-slate-500 font-normal">(empty = none included)</span>
                                </label>
                                <div className="flex flex-wrap gap-2 mb-2">
                                    {(sigConfig?.headers ?? []).map(h => (
                                        <span key={h} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 font-mono">
                                            {h}
                                            <button
                                                onClick={() => setSigConfig(c => ({ ...c!, headers: (c?.headers ?? []).filter(x => x !== h) }))}
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
                                        value={newHeader}
                                        onChange={e => setNewHeader(e.target.value)}
                                        placeholder="Add header name…"
                                        onKeyDown={e => {
                                            if (e.key === 'Enter' && newHeader.trim()) {
                                                setSigConfig(c => ({ ...(c ?? { pathParams: [], queryParams: [], includeBody: true, bodyJsonPaths: [] }), headers: [...(c?.headers ?? []), newHeader.trim()] }))
                                                setNewHeader('')
                                            }
                                        }}
                                        className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500"
                                    />
                                    <button
                                        type="button"
                                        onClick={() => {
                                            if (newHeader.trim()) {
                                                setSigConfig(c => ({ ...(c ?? { pathParams: [], queryParams: [], includeBody: true, bodyJsonPaths: [] }), headers: [...(c?.headers ?? []), newHeader.trim()] }))
                                                setNewHeader('')
                                            }
                                        }}
                                        className="px-3 py-1.5 text-sm bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-slate-300 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
                                    >
                                        <Plus className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>

                            {/* Body */}
                            <div>
                                <label className="flex items-center gap-2 cursor-pointer select-none">
                                    <input
                                        type="checkbox"
                                        checked={sigConfig?.includeBody ?? true}
                                        onChange={e => setSigConfig(c => ({ ...(c ?? { pathParams: [], queryParams: [], headers: [], bodyJsonPaths: [] }), includeBody: e.target.checked }))}
                                        className="w-4 h-4 text-violet-600 border-gray-300 rounded focus:ring-violet-500"
                                    />
                                    <span className="text-sm font-medium text-gray-700 dark:text-slate-300">Include request body in signature</span>
                                </label>
                            </div>

                            {/* Body JSON Paths (only shown when includeBody is true) */}
                            {(sigConfig?.includeBody ?? true) && (
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                                        Body JSON Paths <span className="text-xs text-gray-400 dark:text-slate-500 font-normal">(empty = use full body)</span>
                                    </label>
                                    <div className="flex flex-wrap gap-2 mb-2">
                                        {(sigConfig?.bodyJsonPaths ?? []).map(p => (
                                            <span key={p} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 font-mono">
                                                {p}
                                                <button
                                                    onClick={() => setSigConfig(c => ({ ...c!, bodyJsonPaths: (c?.bodyJsonPaths ?? []).filter(x => x !== p) }))}
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
                                                    setSigConfig(c => ({ ...(c ?? { pathParams: [], queryParams: [], headers: [], includeBody: true }), bodyJsonPaths: [...(c?.bodyJsonPaths ?? []), newBodyPath.trim()] }))
                                                    setNewBodyPath('')
                                                }
                                            }}
                                            className="flex-1 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-violet-500 font-mono"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => {
                                                if (newBodyPath.trim()) {
                                                    setSigConfig(c => ({ ...(c ?? { pathParams: [], queryParams: [], headers: [], includeBody: true }), bodyJsonPaths: [...(c?.bodyJsonPaths ?? []), newBodyPath.trim()] }))
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

                            {/* Actions */}
                            <div className="flex gap-3 pt-2 border-t border-gray-100 dark:border-slate-800">
                                <button
                                    onClick={() => updateSignatureMutation.mutate(sigConfig)}
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
                    )}
                </div>
            </div>

            {/* Script Bindings */}
            <ScriptBindingsPanel operationId={operationId!} />

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
                                        <pre className="mt-1 bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-x-auto max-h-48">
                                            {operation.exampleResponse.body}
                                        </pre>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Response Priority Info */}
            <div className="mt-6 bg-blue-50 border border-blue-200 dark:bg-blue-950/30 dark:border-blue-900/40 rounded-xl p-4">
                <div className="flex items-start">
                    <Info className="w-5 h-5 text-blue-600 mt-0.5 mr-3 flex-shrink-0" />
                    <div className="text-sm text-blue-700 dark:text-blue-300">
                        <strong>Response Matching Order:</strong>
                        <ol className="list-decimal list-inside mt-1 space-y-0.5">
                            <li>First enabled response with matching conditions (by priority)</li>
                            {operation.exampleResponse && spec?.useExampleFallback && (
                                <li>Example response from OpenAPI spec (status {operation.exampleResponse.statusCode})</li>
                            )}
                            <li>404 Not Found</li>
                        </ol>
                    </div>
                </div>
            </div>

            {/* Editor Modal */}
        </div>
    )
}
