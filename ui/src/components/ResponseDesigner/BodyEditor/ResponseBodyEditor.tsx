import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type * as Monaco from 'monaco-editor'
import clsx from 'clsx'
import {
    collectionMappingsApi,
    operationsApi,
    responseScriptBindingsApi,
    scriptBindingsApi,
    validationRulesApi,
} from '../../../services/api'
import { buildTemplateSourceOptions } from './templateSources'
import TemplateTextEditor from './TemplateTextEditor'
import VisualTemplateEditor from './VisualTemplateEditor'

type BodyMode = 'text' | 'visual'

interface ResponseBodyEditorProps {
    body: string
    onBodyChange: (body: string) => void
    operationId: string
    responseConfigId?: string
    readOnly?: boolean
    isDark: boolean
    height?: string
    heightClass?: string
    lineNumbers?: 'on' | 'off'
    minimap?: boolean
    folding?: boolean
    bodyError?: string
    onTextEditorMount?: (editor: Monaco.editor.IStandaloneCodeEditor, monaco: typeof Monaco) => void
}

export default function ResponseBodyEditor({
    body,
    onBodyChange,
    operationId,
    responseConfigId,
    readOnly = false,
    isDark,
    height = '240px',
    heightClass = 'h-[360px]',
    lineNumbers = 'off',
    minimap = false,
    folding = false,
    bodyError,
    onTextEditorMount,
}: ResponseBodyEditorProps) {
    const [mode, setMode] = useState<BodyMode>('text')

    const { data: operation } = useQuery({
        queryKey: ['operation', operationId],
        queryFn: () => operationsApi.get(operationId),
        enabled: !!operationId,
    })

    const { data: operationScriptBindings } = useQuery({
        queryKey: ['scriptBindings', operationId],
        queryFn: () => scriptBindingsApi.listByOperation(operationId),
        enabled: !!operationId,
    })

    const { data: responseScriptBindings } = useQuery({
        queryKey: ['responseScriptBindings', operationId, responseConfigId],
        queryFn: () => responseScriptBindingsApi.listByResponse(operationId, responseConfigId || ''),
        enabled: !!operationId && !!responseConfigId,
    })

    const { data: operationCollectionMappings } = useQuery({
        queryKey: ['collectionMappings', 'operation', operationId],
        queryFn: () => collectionMappingsApi.listByOperation(operationId),
        enabled: !!operationId,
    })

    const { data: responseCollectionMappings } = useQuery({
        queryKey: ['collectionMappings', 'response', operationId, responseConfigId],
        queryFn: () => collectionMappingsApi.listByResponse(operationId, responseConfigId || ''),
        enabled: !!operationId && !!responseConfigId,
    })

    const { data: validationRules } = useQuery({
        queryKey: ['validationRules', operationId],
        queryFn: () => validationRulesApi.listByOperation(operationId),
        enabled: !!operationId,
    })

    const sources = useMemo(() => buildTemplateSourceOptions({
        operation,
        operationScriptBindings,
        responseScriptBindings,
        operationCollectionMappings,
        responseCollectionMappings,
        validationRules,
        responseConfigId,
    }), [
        operation,
        operationScriptBindings,
        responseScriptBindings,
        operationCollectionMappings,
        responseCollectionMappings,
        validationRules,
        responseConfigId,
    ])

    return (
        <div className="min-h-0">
            <div className="mb-2 flex items-center justify-between gap-3">
                <div className="inline-flex h-8 overflow-hidden rounded-md border border-gray-300 bg-gray-50 p-0.5 dark:border-slate-700 dark:bg-slate-900">
                    {(['text', 'visual'] as const).map((item) => (
                        <button
                            key={item}
                            type="button"
                            onClick={() => setMode(item)}
                            className={clsx(
                                'h-7 px-3 text-xs font-medium capitalize transition-colors',
                                mode === item
                                    ? 'rounded bg-white text-gray-900 shadow-sm dark:bg-slate-800 dark:text-slate-100'
                                    : 'text-gray-500 hover:text-gray-800 dark:text-slate-400 dark:hover:text-slate-200',
                            )}
                        >
                            {item}
                        </button>
                    ))}
                </div>
                {mode === 'visual' && !responseConfigId && (
                    <span className="truncate text-xs text-gray-500 dark:text-slate-400">
                        Response-scoped outputs unlock after save.
                    </span>
                )}
            </div>

            <div
                className={clsx(
                    mode === 'text' && 'overflow-hidden rounded-lg border',
                    mode === 'text' && (bodyError ? 'border-red-300 dark:border-red-700' : 'border-gray-300 dark:border-slate-700'),
                )}
            >
                {mode === 'text' ? (
                    <TemplateTextEditor
                        body={body}
                        onBodyChange={onBodyChange}
                        height={height}
                        readOnly={readOnly}
                        isDark={isDark}
                        lineNumbers={lineNumbers}
                        minimap={minimap}
                        folding={folding}
                        onMount={onTextEditorMount}
                    />
                ) : (
                    <VisualTemplateEditor
                        body={body}
                        onBodyChange={onBodyChange}
                        sources={sources}
                        readOnly={readOnly}
                        heightClass={heightClass}
                    />
                )}
            </div>
        </div>
    )
}

