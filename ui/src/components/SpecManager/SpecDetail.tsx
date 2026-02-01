import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
    ArrowLeft,
    FileCode2,
    ChevronRight,
    Sparkles
} from 'lucide-react'
import clsx from 'clsx'
import { specsApi, operationsApi } from '../../services/api'
import type { Spec, OperationSummary } from '../../types'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700',
    POST: 'bg-blue-100 text-blue-700',
    PUT: 'bg-yellow-100 text-yellow-700',
    DELETE: 'bg-red-100 text-red-700',
    PATCH: 'bg-purple-100 text-purple-700',
    HEAD: 'bg-gray-100 text-gray-700',
    OPTIONS: 'bg-gray-100 text-gray-700',
}

export default function SpecDetail() {
    const { specId } = useParams<{ specId: string }>()

    const { data: spec, isLoading: specLoading } = useQuery<Spec>({
        queryKey: ['spec', specId],
        queryFn: () => specsApi.get(specId!),
        enabled: !!specId,
    })

    const { data: operations, isLoading: opsLoading } = useQuery<OperationSummary[]>({
        queryKey: ['operations', specId],
        queryFn: () => operationsApi.listBySpec(specId!),
        enabled: !!specId,
    })

    if (specLoading || opsLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (!spec) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
                    Specification not found
                </div>
            </div>
        )
    }

    // Group operations by tag
    const groupedOps = (operations || []).reduce((acc, op) => {
        const tag = op.operationId.split('_')[0] || 'default'
        if (!acc[tag]) acc[tag] = []
        acc[tag].push(op)
        return acc
    }, {} as Record<string, OperationSummary[]>)

    return (
        <div className="p-8">
            {/* Header */}
            <div className="mb-8">
                <Link
                    to="/specs"
                    className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700 mb-4"
                >
                    <ArrowLeft className="w-4 h-4 mr-1" />
                    Back to Specifications
                </Link>

                <div className="flex items-start justify-between">
                    <div className="flex items-start">
                        <div className="p-3 bg-primary-100 rounded-lg">
                            <FileCode2 className="w-8 h-8 text-primary-600" />
                        </div>
                        <div className="ml-4">
                            <h1 className="text-2xl font-bold text-gray-900">{spec.name}</h1>
                            <p className="text-gray-500 mt-1">{spec.description || 'No description'}</p>
                            <div className="flex items-center gap-4 mt-3 text-sm">
                                <span className={clsx(
                                    'px-2 py-1 rounded-full text-xs font-medium',
                                    spec.enabled ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
                                )}>
                                    {spec.enabled ? 'Enabled' : 'Disabled'}
                                </span>
                                <span className="text-gray-500">
                                    Version: <span className="font-medium text-gray-700">{spec.version}</span>
                                </span>
                                <span className="text-gray-500">
                                    Base Path: <code className="font-mono bg-gray-100 px-1 rounded">{spec.basePath || '/'}</code>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Operations */}
            <div className="bg-white rounded-xl shadow-sm border border-gray-200">
                <div className="p-6 border-b border-gray-200">
                    <h2 className="text-lg font-semibold text-gray-900">
                        Operations ({operations?.length || 0})
                    </h2>
                </div>

                {operations && operations.length > 0 ? (
                    <div className="divide-y divide-gray-100">
                        {Object.entries(groupedOps).map(([tag, ops]) => (
                            <div key={tag}>
                                <div className="px-6 py-3 bg-gray-50 text-sm font-medium text-gray-500 uppercase">
                                    {tag}
                                </div>
                                {ops.map((op) => (
                                    <Link
                                        key={op.id}
                                        to={`/operations/${op.id}`}
                                        className="flex items-center justify-between px-6 py-4 hover:bg-gray-50 transition-colors"
                                    >
                                        <div className="flex items-center">
                                            <span className={clsx(
                                                'px-2 py-1 rounded text-xs font-bold uppercase w-20 text-center',
                                                methodColors[op.method] || 'bg-gray-100 text-gray-700'
                                            )}>
                                                {op.method}
                                            </span>
                                            <div className="ml-4">
                                                <p className="font-mono text-sm text-gray-900">{op.path}</p>
                                                {op.summary && (
                                                    <p className="text-sm text-gray-500 mt-0.5">{op.summary}</p>
                                                )}
                                            </div>
                                        </div>
                                        <div className="flex items-center text-gray-400">
                                            {op.hasExampleResponse && (
                                                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-700 mr-2" title="Has example response from spec">
                                                    <Sparkles className="w-3 h-3 mr-1" />
                                                    Example
                                                </span>
                                            )}
                                            <span className="text-sm mr-2">
                                                {op.responseCount} response{op.responseCount !== 1 ? 's' : ''}
                                            </span>
                                            <ChevronRight className="w-5 h-5" />
                                        </div>
                                    </Link>
                                ))}
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="p-12 text-center text-gray-500">
                        No operations found in this specification
                    </div>
                )}
            </div>
        </div>
    )
}
