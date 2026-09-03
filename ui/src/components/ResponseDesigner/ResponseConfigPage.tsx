import { useMemo } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { operationsApi, responsesApi } from '../../services/api'
import type { Operation, ResponseConfig } from '../../types'
import CollectionResponseEditor from './CollectionResponseEditor'
import ResponseConfigEditor from './ResponseConfigEditor'
import ResponseConfigIDE from './ResponseConfigIDE'

export default function ResponseConfigPage() {
    const navigate = useNavigate()
    const [searchParams] = useSearchParams()
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

    const { data: _operation, isLoading: opLoading } = useQuery<Operation>({
        queryKey: ['operation', effectiveOperationId],
        queryFn: () => operationsApi.get(effectiveOperationId),
        enabled: !!effectiveOperationId,
    })

    const isLoading = opLoading || respLoading
    const source = searchParams.get('source')
    const backToRecorded = source === 'recorded'
    const backPath = backToRecorded
        ? `/operations/${effectiveOperationId}/recorded-responses`
        : `/operations/${effectiveOperationId}`

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

    const sharedProps = {
        operationId: effectiveOperationId,
        config: responseConfig || null,
        readOnly: backToRecorded,
    }

    // Collection Responses use one unified editor (no desktop IDE / mobile
    // form split): the kind is either loaded from an existing response, or
    // chosen at creation time via ?kind=collection.
    const isCollectionResponse = responseConfig
        ? responseConfig.kind === 'collection'
        : searchParams.get('kind') === 'collection'

    if (isCollectionResponse) {
        return (
            <div className="h-full overflow-hidden">
                <CollectionResponseEditor
                    operationId={effectiveOperationId}
                    config={responseConfig || null}
                    onClose={() => navigate(backPath)}
                />
            </div>
        )
    }

    return (
        <>
            {/* Desktop: full IDE layout (md and above) */}
            <div className="hidden md:block h-full overflow-hidden">
                <ResponseConfigIDE
                    {...sharedProps}
                    onSaved={() => navigate(backPath)}
                    onBack={() => navigate(backPath)}
                />
            </div>

            {/* Mobile: classic scrollable form */}
            <div className="md:hidden p-4 space-y-6">
                <ResponseConfigEditor
                    {...sharedProps}
                    onClose={() => navigate(backPath)}
                    variant="page"
                />
            </div>
        </>
    )
}

