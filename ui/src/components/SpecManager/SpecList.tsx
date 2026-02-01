import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
    Plus,
    Upload,
    FileCode2,
    ToggleLeft,
    ToggleRight,
    Trash2,
    Activity,
    ExternalLink,
    Sparkles
} from 'lucide-react'
import clsx from 'clsx'
import { specsApi } from '../../services/api'
import type { Spec } from '../../types'
import UploadSpecModal from './UploadSpecModal'

export default function SpecList() {
    const [showUploadModal, setShowUploadModal] = useState(false)
    const queryClient = useQueryClient()

    const { data: specs, isLoading, error } = useQuery<Spec[]>({
        queryKey: ['specs'],
        queryFn: specsApi.list,
    })

    const toggleEnabledMutation = useMutation({
        mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
            enabled ? specsApi.disable(id) : specsApi.enable(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['specs'] })
        },
    })

    const toggleTracingMutation = useMutation({
        mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
            specsApi.toggleTracing(id, !enabled),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['specs'] })
        },
    })

    const toggleExampleFallbackMutation = useMutation({
        mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
            specsApi.toggleExampleFallback(id, !enabled),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['specs'] })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: specsApi.delete,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['specs'] })
        },
    })

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 rounded-xl"></div>
                    <div className="h-32 bg-gray-200 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
                    Failed to load specs: {(error as Error).message}
                </div>
            </div>
        )
    }

    return (
        <div className="p-8">
            <div className="flex items-center justify-between mb-8">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">API Specifications</h1>
                    <p className="text-gray-500 mt-1">
                        Manage your OpenAPI 3 specifications and configure virtual responses
                    </p>
                </div>
                <button
                    onClick={() => setShowUploadModal(true)}
                    className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                >
                    <Upload className="w-5 h-5 mr-2" />
                    Upload Spec
                </button>
            </div>

            {specs && specs.length > 0 ? (
                <div className="space-y-4">
                    {specs.map((spec) => (
                        <div
                            key={spec.id}
                            className="bg-white rounded-xl shadow-sm border border-gray-200 p-6"
                        >
                            <div className="flex items-start justify-between">
                                <div className="flex items-start">
                                    <div className="p-3 bg-primary-100 rounded-lg">
                                        <FileCode2 className="w-6 h-6 text-primary-600" />
                                    </div>
                                    <div className="ml-4">
                                        <Link
                                            to={`/specs/${spec.id}`}
                                            className="text-lg font-semibold text-gray-900 hover:text-primary-600 flex items-center"
                                        >
                                            {spec.name}
                                            <ExternalLink className="w-4 h-4 ml-2" />
                                        </Link>
                                        <p className="text-sm text-gray-500 mt-1">
                                            {spec.description || 'No description'}
                                        </p>
                                        <div className="flex items-center gap-4 mt-3 text-sm">
                                            <span className="text-gray-500">
                                                Version: <span className="font-medium text-gray-700">{spec.version}</span>
                                            </span>
                                            <span className="text-gray-500">
                                                Base Path: <code className="font-mono bg-gray-100 px-1 rounded">{spec.basePath || '/'}</code>
                                            </span>
                                            <span className="text-gray-500">
                                                Operations: <span className="font-medium text-gray-700">{spec.operationCount}</span>
                                            </span>
                                        </div>
                                    </div>
                                </div>

                                <div className="flex items-center gap-4">
                                    {/* Example Fallback Toggle */}
                                    <button
                                        onClick={() => toggleExampleFallbackMutation.mutate({ id: spec.id, enabled: spec.useExampleFallback })}
                                        className={clsx(
                                            'flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
                                            spec.useExampleFallback
                                                ? 'bg-amber-100 text-amber-700'
                                                : 'bg-gray-100 text-gray-500'
                                        )}
                                        title={spec.useExampleFallback ? 'Disable example fallback' : 'Enable example fallback'}
                                    >
                                        <Sparkles className="w-4 h-4" />
                                        Fallback
                                    </button>

                                    {/* Tracing Toggle */}
                                    <button
                                        onClick={() => toggleTracingMutation.mutate({ id: spec.id, enabled: spec.tracing })}
                                        className={clsx(
                                            'flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
                                            spec.tracing
                                                ? 'bg-yellow-100 text-yellow-700'
                                                : 'bg-gray-100 text-gray-500'
                                        )}
                                        title={spec.tracing ? 'Disable tracing' : 'Enable tracing'}
                                    >
                                        <Activity className="w-4 h-4" />
                                        Trace
                                    </button>

                                    {/* Enable/Disable Toggle */}
                                    <button
                                        onClick={() => toggleEnabledMutation.mutate({ id: spec.id, enabled: spec.enabled })}
                                        className={clsx(
                                            'flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
                                            spec.enabled
                                                ? 'bg-green-100 text-green-700'
                                                : 'bg-gray-100 text-gray-500'
                                        )}
                                    >
                                        {spec.enabled ? (
                                            <>
                                                <ToggleRight className="w-5 h-5" />
                                                Enabled
                                            </>
                                        ) : (
                                            <>
                                                <ToggleLeft className="w-5 h-5" />
                                                Disabled
                                            </>
                                        )}
                                    </button>

                                    {/* Delete */}
                                    <button
                                        onClick={() => {
                                            if (confirm('Are you sure you want to delete this spec?')) {
                                                deleteMutation.mutate(spec.id)
                                            }
                                        }}
                                        className="p-2 text-gray-400 hover:text-red-600 rounded-lg hover:bg-red-50 transition-colors"
                                        title="Delete spec"
                                    >
                                        <Trash2 className="w-5 h-5" />
                                    </button>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-12 text-center">
                    <FileCode2 className="w-16 h-16 text-gray-300 mx-auto mb-4" />
                    <h3 className="text-lg font-medium text-gray-900 mb-2">
                        No API Specifications
                    </h3>
                    <p className="text-gray-500 mb-6">
                        Upload your first OpenAPI 3 specification to get started
                    </p>
                    <button
                        onClick={() => setShowUploadModal(true)}
                        className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        Upload Spec
                    </button>
                </div>
            )}

            {showUploadModal && (
                <UploadSpecModal onClose={() => setShowUploadModal(false)} />
            )}
        </div>
    )
}
