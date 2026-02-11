import { useMemo } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { operationsApi, responsesApi } from '../../services/api'
import type { Operation, ResponseConfig } from '../../types'
import ResponseConfigEditor from './ResponseConfigEditor'

export default function ResponseConfigPage() {
    const navigate = useNavigate()
    const { operationId, responseId } = useParams<{ operationId?: string; responseId?: string }>()

    const { data: responseConfig, isLoading: respLoading } = useQuery<ResponseConfig>({
        queryKey: ['response', responseId],
        queryFn: () => responsesApi.get(responseId!),
        enabled: !!responseId,
    })

    const effectiveOperationId = useMemo(() => {
        if (responseConfig?.operationId) {
            return responseConfig.operationId
        }
        return operationId || ''
    }, [operationId, responseConfig?.operationId])

    const { data: operation, isLoading: opLoading } = useQuery<Operation>({
        queryKey: ['operation', effectiveOperationId],
        queryFn: () => operationsApi.get(effectiveOperationId),
        enabled: !!effectiveOperationId,
    })

    const isLoading = opLoading || respLoading

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (!effectiveOperationId) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Operation not found
                </div>
            </div>
        )
    }

    return (
        <div className="p-8 space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-4">
                <div className="space-y-2">
                    <Link
                        to={`/operations/${effectiveOperationId}`}
                        className="inline-flex items-center text-sm text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200"
                    >
                        <ArrowLeft className="w-4 h-4 mr-1" />
                        Back to Operation
                    </Link>
                    <div>
                        <h1 className="text-2xl font-semibold text-gray-900 dark:text-slate-100">
                            {responseId ? 'Edit Response Configuration' : 'Create Response Configuration'}
                        </h1>
                        <p className="text-sm text-gray-500 dark:text-slate-400">
                            Configure conditions, headers, and response body templates for this operation.
                        </p>
                    </div>
                </div>
                {operation && (
                    <div className="rounded-xl border border-gray-200 dark:border-slate-800 px-4 py-2 bg-white dark:bg-slate-900 text-sm text-gray-600 dark:text-slate-300">
                        <span className="font-semibold text-gray-900 dark:text-slate-100 mr-2">{operation.method}</span>
                        <span className="font-mono">{operation.path}</span>
                    </div>
                )}
            </div>

            <ResponseConfigEditor
                operationId={effectiveOperationId}
                config={responseConfig || null}
                onClose={() => navigate(`/operations/${effectiveOperationId}`)}
                variant="page"
            />
        </div>
    )
}
