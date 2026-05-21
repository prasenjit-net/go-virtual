import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
    AlertCircle,
    Bot,
    ChevronDown,
    ChevronRight,
    Code2,
    Check,
    Copy,
    Edit2,
    Eye,
    GripVertical,
    Plus,
    Radio,
    ToggleLeft,
    ToggleRight,
    Trash2,
} from 'lucide-react'
import clsx from 'clsx'
import { responsesApi, responseScriptBindingsApi } from '../../services/api'
import type { ResponseConfig, ScriptBinding } from '../../types'
import { serializeResponseForClipboard } from './responseTransfer'

interface ResponseConfigListProps {
    operationId: string
    configs: ResponseConfig[]
    emptyTitle: string
    emptyDescription?: string
    emptyAction?: {
        to: string
        label: string
    }
    editSource?: 'operation' | 'recorded'
    enableManualActions?: boolean
}

function ResponseScriptsSection({ operationId, configId }: { operationId: string; configId: string }) {
    const { data: bindings, isLoading } = useQuery<ScriptBinding[]>({
        queryKey: ['responseScriptBindings', configId],
        queryFn: () => responseScriptBindingsApi.listByResponse(operationId, configId),
    })

    if (isLoading || !bindings || bindings.length === 0) return null

    return (
        <div>
            <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Scripts</h4>
            <div className="space-y-1">
                {bindings.sort((a, b) => a.order - b.order).map((b) => (
                    <div key={b.id} className="flex items-center gap-2 text-sm bg-emerald-50 dark:bg-emerald-950/30 rounded px-3 py-2">
                        <Code2 className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400 flex-shrink-0" />
                        <span className="font-mono text-emerald-700 dark:text-emerald-300">{b.outputKey}</span>
                        <span className="text-gray-400 dark:text-slate-500">←</span>
                        <span className="text-gray-600 dark:text-slate-300">{b.scriptName || b.scriptId}</span>
                        {!b.enabled && (
                            <span className="ml-auto text-xs text-gray-400 dark:text-slate-500 italic">disabled</span>
                        )}
                    </div>
                ))}
            </div>
        </div>
    )
}

export default function ResponseConfigList({
    operationId,
    configs,
    emptyTitle,
    emptyDescription,
    emptyAction,
    editSource = 'operation',
    enableManualActions = false,
}: ResponseConfigListProps) {
    const [expandedConfig, setExpandedConfig] = useState<string | null>(null)
    const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
    const queryClient = useQueryClient()

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

    const cloneMutation = useMutation({
        mutationFn: ({ id, name }: { id: string; name: string }) => responsesApi.clone(id, name),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            setFeedback({ type: 'success', message: 'Response cloned successfully.' })
        },
        onError: (err: Error) => {
            setFeedback({ type: 'error', message: err.message || 'Failed to clone response.' })
        },
    })

    const handleClone = (e: React.MouseEvent, config: ResponseConfig) => {
        e.stopPropagation()
        cloneMutation.mutate({ id: config.id, name: `${config.name} clone` })
    }

    const handleCopy = async (e: React.MouseEvent, config: ResponseConfig) => {
        e.stopPropagation()
        try {
            if (!navigator?.clipboard?.writeText) {
                throw new Error('Clipboard API is not available in this browser context')
            }
            await navigator.clipboard.writeText(serializeResponseForClipboard(config))
            setFeedback({ type: 'success', message: 'Response payload copied to clipboard.' })
        } catch (err) {
            setFeedback({ type: 'error', message: (err as Error).message || 'Failed to copy payload.' })
        }
    }

    const isRecorded = (config: ResponseConfig) => config.recorded === true
    const editSuffix = editSource === 'recorded' ? '?source=recorded' : ''

    const renderOriginBadge = (config: ResponseConfig) => {
        if (!config.recorded) {
            return null
        }
        if (config.origin === 'ai') {
            return (
                <span className="ml-2 inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/40 dark:text-fuchsia-300">
                    <Bot className="w-2.5 h-2.5" />
                    AI
                </span>
            )
        }
        return (
            <span className="ml-2 inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300">
                <Radio className="w-2.5 h-2.5" />
                Proxy
            </span>
        )
    }

    if (configs.length === 0) {
        return (
            <div className="p-12 text-center">
                <p className="text-gray-700 dark:text-slate-200 font-medium">{emptyTitle}</p>
                {emptyDescription && (
                    <p className="text-gray-500 dark:text-slate-400 mt-2 mb-4">{emptyDescription}</p>
                )}
                {emptyAction && (
                    <Link
                        to={emptyAction.to}
                        className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        {emptyAction.label}
                    </Link>
                )}
            </div>
        )
    }

    return (
        <div>
            {feedback && (
                <div
                    className={clsx(
                        'mx-4 mt-4 mb-2 p-3 rounded-lg text-sm flex items-start gap-2',
                        feedback.type === 'success'
                            ? 'bg-green-50 text-green-700 border border-green-200 dark:bg-green-950/30 dark:text-green-300 dark:border-green-900/40'
                            : 'bg-red-50 text-red-700 border border-red-200 dark:bg-red-950/30 dark:text-red-300 dark:border-red-900/40'
                    )}
                >
                    {feedback.type === 'success' ? (
                        <Check className="w-4 h-4 mt-0.5 shrink-0" />
                    ) : (
                        <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                    )}
                    <span>{feedback.message}</span>
                </div>
            )}
            <div className="divide-y divide-gray-100 dark:divide-slate-800">
                {configs.map((config, index) => (
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
                                    {renderOriginBadge(config)}
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
                            {isRecorded(config) ? (
                                <>
                                    <Link
                                        to={`/responses/${config.id}/edit?source=recorded`}
                                        onClick={(e) => e.stopPropagation()}
                                        className="p-2 text-gray-400 dark:text-slate-500 hover:text-blue-600 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors"
                                        title="View (read-only)"
                                    >
                                        <Eye className="w-5 h-5" />
                                    </Link>
                                </>
                            ) : (
                                <>
                                    <Link
                                        to={`/responses/${config.id}/edit${editSuffix}`}
                                        onClick={(e) => e.stopPropagation()}
                                        className="p-2 text-gray-400 dark:text-slate-500 hover:text-primary-600 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/30 transition-colors"
                                        title="Edit"
                                    >
                                        <Edit2 className="w-5 h-5" />
                                    </Link>
                                    {enableManualActions && (
                                        <button
                                            onClick={(e) => void handleCopy(e, config)}
                                            className="p-2 text-gray-400 dark:text-slate-500 hover:text-blue-600 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors"
                                            title="Copy response payload"
                                        >
                                            <Copy className="w-5 h-5" />
                                        </button>
                                    )}
                                    {enableManualActions && (
                                        <button
                                            onClick={(e) => handleClone(e, config)}
                                            disabled={cloneMutation.isPending}
                                            className="p-2 text-gray-400 dark:text-slate-500 hover:text-emerald-600 rounded-lg hover:bg-emerald-50 dark:hover:bg-emerald-900/30 transition-colors"
                                            title="Clone response"
                                        >
                                            <Plus className="w-5 h-5" />
                                        </button>
                                    )}
                                </>
                            )}
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

                    {expandedConfig === config.id && (
                        <div className="mt-4 ml-16 space-y-4">
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

                            <ResponseScriptsSection operationId={operationId} configId={config.id} />

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

                            {config.body && (
                                <div>
                                    <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Body</h4>
                                    <pre className="bg-gray-100 dark:bg-gray-900 text-gray-900 dark:text-gray-100 rounded p-4 text-sm overflow-x-auto">
                                        {config.body}
                                    </pre>
                                </div>
                            )}

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
        </div>
    )
}
