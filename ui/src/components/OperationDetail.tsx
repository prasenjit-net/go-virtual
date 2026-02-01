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
    ChevronRight
} from 'lucide-react'
import clsx from 'clsx'
import { operationsApi, responsesApi } from '../services/api'
import type { Operation, ResponseConfig } from '../types'
import ResponseConfigEditor from './ResponseDesigner/ResponseConfigEditor'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700',
    POST: 'bg-blue-100 text-blue-700',
    PUT: 'bg-yellow-100 text-yellow-700',
    DELETE: 'bg-red-100 text-red-700',
    PATCH: 'bg-purple-100 text-purple-700',
}

export default function OperationDetail() {
    const { operationId } = useParams<{ operationId: string }>()
    const [showEditor, setShowEditor] = useState(false)
    const [editingConfig, setEditingConfig] = useState<ResponseConfig | null>(null)
    const [expandedConfig, setExpandedConfig] = useState<string | null>(null)
    const queryClient = useQueryClient()

    const { data: operation, isLoading: opLoading } = useQuery<Operation>({
        queryKey: ['operation', operationId],
        queryFn: () => operationsApi.get(operationId!),
        enabled: !!operationId,
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

    if (opLoading || respLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (!operation) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
                    Operation not found
                </div>
            </div>
        )
    }

    const handleEdit = (config: ResponseConfig) => {
        setEditingConfig(config)
        setShowEditor(true)
    }

    const handleCreate = () => {
        setEditingConfig(null)
        setShowEditor(true)
    }

    const handleEditorClose = () => {
        setShowEditor(false)
        setEditingConfig(null)
    }

    return (
        <div className="p-8">
            {/* Header */}
            <div className="mb-8">
                <Link
                    to={`/specs/${operation.specId}`}
                    className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700 mb-4"
                >
                    <ArrowLeft className="w-4 h-4 mr-1" />
                    Back to Specification
                </Link>

                <div className="flex items-center">
                    <span className={clsx(
                        'px-3 py-1.5 rounded text-sm font-bold uppercase',
                        methodColors[operation.method] || 'bg-gray-100 text-gray-700'
                    )}>
                        {operation.method}
                    </span>
                    <h1 className="text-2xl font-mono font-bold text-gray-900 ml-4">
                        {operation.path}
                    </h1>
                </div>
                {operation.summary && (
                    <p className="text-gray-500 mt-2">{operation.summary}</p>
                )}
                <p className="text-sm text-gray-400 mt-1">
                    Full path: <code className="font-mono bg-gray-100 px-1 rounded">{operation.fullPath}</code>
                </p>
            </div>

            {/* Response Configurations */}
            <div className="bg-white rounded-xl shadow-sm border border-gray-200">
                <div className="p-6 border-b border-gray-200 flex items-center justify-between">
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900">Response Configurations</h2>
                        <p className="text-sm text-gray-500 mt-1">
                            Configure mock responses with conditions and priorities
                        </p>
                    </div>
                    <button
                        onClick={handleCreate}
                        className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        Add Response
                    </button>
                </div>

                {responses && responses.length > 0 ? (
                    <div className="divide-y divide-gray-100">
                        {responses.map((config, index) => (
                            <div key={config.id} className="p-4">
                                <div
                                    className="flex items-center justify-between cursor-pointer"
                                    onClick={() => setExpandedConfig(
                                        expandedConfig === config.id ? null : config.id
                                    )}
                                >
                                    <div className="flex items-center">
                                        <GripVertical className="w-5 h-5 text-gray-300 mr-3" />
                                        <span className="w-8 h-8 flex items-center justify-center bg-gray-100 rounded-full text-sm font-medium text-gray-600 mr-3">
                                            {index + 1}
                                        </span>
                                        <div>
                                            <div className="flex items-center">
                                                <span className="font-medium text-gray-900">{config.name}</span>
                                                <span className={clsx(
                                                    'ml-3 px-2 py-0.5 rounded text-xs font-medium',
                                                    config.statusCode >= 200 && config.statusCode < 300
                                                        ? 'bg-green-100 text-green-700'
                                                        : config.statusCode >= 400
                                                            ? 'bg-red-100 text-red-700'
                                                            : 'bg-gray-100 text-gray-700'
                                                )}>
                                                    {config.statusCode}
                                                </span>
                                                {config.conditions.length > 0 && (
                                                    <span className="ml-2 text-xs text-gray-400">
                                                        {config.conditions.length} condition{config.conditions.length !== 1 ? 's' : ''}
                                                    </span>
                                                )}
                                            </div>
                                            {config.description && (
                                                <p className="text-sm text-gray-500 mt-0.5">{config.description}</p>
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
                                                    ? 'text-green-600 hover:bg-green-50'
                                                    : 'text-gray-400 hover:bg-gray-100'
                                            )}
                                            title={config.enabled ? 'Disable' : 'Enable'}
                                        >
                                            {config.enabled ? (
                                                <ToggleRight className="w-5 h-5" />
                                            ) : (
                                                <ToggleLeft className="w-5 h-5" />
                                            )}
                                        </button>
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                handleEdit(config)
                                            }}
                                            className="p-2 text-gray-400 hover:text-primary-600 rounded-lg hover:bg-primary-50 transition-colors"
                                            title="Edit"
                                        >
                                            <Edit2 className="w-5 h-5" />
                                        </button>
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                if (confirm('Delete this response configuration?')) {
                                                    deleteMutation.mutate(config.id)
                                                }
                                            }}
                                            className="p-2 text-gray-400 hover:text-red-600 rounded-lg hover:bg-red-50 transition-colors"
                                            title="Delete"
                                        >
                                            <Trash2 className="w-5 h-5" />
                                        </button>
                                        {expandedConfig === config.id ? (
                                            <ChevronDown className="w-5 h-5 text-gray-400" />
                                        ) : (
                                            <ChevronRight className="w-5 h-5 text-gray-400" />
                                        )}
                                    </div>
                                </div>

                                {/* Expanded Details */}
                                {expandedConfig === config.id && (
                                    <div className="mt-4 ml-16 space-y-4">
                                        {/* Conditions */}
                                        {config.conditions.length > 0 && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 mb-2">Conditions</h4>
                                                <div className="space-y-2">
                                                    {config.conditions.map((cond, i) => (
                                                        <div key={i} className="flex items-center text-sm bg-gray-50 rounded px-3 py-2">
                                                            <span className="text-purple-600 font-mono">{cond.source}</span>
                                                            <span className="mx-2 text-gray-400">.</span>
                                                            <span className="text-blue-600 font-mono">{cond.key}</span>
                                                            <span className="mx-2 text-gray-500">{cond.operator}</span>
                                                            <span className="text-green-600 font-mono">"{cond.value}"</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        )}

                                        {/* Headers */}
                                        {Object.keys(config.headers).length > 0 && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 mb-2">Headers</h4>
                                                <div className="bg-gray-50 rounded p-3 font-mono text-sm">
                                                    {Object.entries(config.headers).map(([key, value]) => (
                                                        <div key={key}>
                                                            <span className="text-purple-600">{key}</span>
                                                            <span className="text-gray-400">: </span>
                                                            <span className="text-green-600">{value}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        )}

                                        {/* Body */}
                                        {config.body && (
                                            <div>
                                                <h4 className="text-sm font-medium text-gray-700 mb-2">Body</h4>
                                                <pre className="bg-gray-900 text-gray-100 rounded p-4 text-sm overflow-x-auto">
                                                    {config.body}
                                                </pre>
                                            </div>
                                        )}

                                        {/* Delay */}
                                        {config.delay > 0 && (
                                            <div className="text-sm text-gray-500">
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
                        <p className="text-gray-500 mb-4">No response configurations yet</p>
                        <button
                            onClick={handleCreate}
                            className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                        >
                            <Plus className="w-5 h-5 mr-2" />
                            Add First Response
                        </button>
                    </div>
                )}
            </div>

            {/* Editor Modal */}
            {showEditor && (
                <ResponseConfigEditor
                    operationId={operationId!}
                    config={editingConfig}
                    onClose={handleEditorClose}
                />
            )}
        </div>
    )
}
