import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { X, BookOpen, AlertTriangle, ChevronDown, ChevronRight } from 'lucide-react'
import { operationsApi } from '../../services/api'
import type { SpecExample } from '../../types'

interface ExamplePickerModalProps {
    operationId: string
    onSelect: (example: SpecExample) => void
    onClose: () => void
}

function statusBadgeClass(code: number): string {
    if (code >= 200 && code < 300) {
        return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
    }
    if (code >= 300 && code < 400) {
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
    }
    if (code >= 400 && code < 500) {
        return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300'
    }
    if (code >= 500) {
        return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    }
    return 'bg-gray-100 text-gray-700 dark:bg-slate-700 dark:text-slate-300'
}

function statusLabel(code: number): string {
    if (code === 0) return 'default'
    return String(code)
}

export default function ExamplePickerModal({ operationId, onSelect, onClose }: ExamplePickerModalProps) {
    const [expanded, setExpanded] = useState<number | null>(null)

    const { data: examples = [], isLoading } = useQuery({
        queryKey: ['spec-examples', operationId],
        queryFn: () => operationsApi.getSpecExamples(operationId),
        staleTime: 60_000,
    })

    const toggle = (code: number) => setExpanded(prev => prev === code ? null : code)

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
            <div className="bg-white dark:bg-slate-800 rounded-xl shadow-2xl w-full max-w-2xl max-h-[80vh] flex flex-col border border-gray-200 dark:border-slate-700">
                {/* Header */}
                <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-slate-700">
                    <div className="flex items-center gap-2">
                        <BookOpen className="w-5 h-5 text-indigo-500" />
                        <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                            Spec Examples
                        </h2>
                    </div>
                    <button
                        onClick={onClose}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors"
                        aria-label="Close"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                {/* Body */}
                <div className="flex-1 overflow-y-auto">
                    {isLoading && (
                        <div className="flex items-center justify-center py-12 text-sm text-gray-400 dark:text-slate-500">
                            Loading examples…
                        </div>
                    )}

                    {!isLoading && examples.length === 0 && (
                        <div className="flex flex-col items-center justify-center py-12 gap-2 text-center px-6">
                            <BookOpen className="w-8 h-8 text-gray-300 dark:text-slate-600" />
                            <p className="text-sm font-medium text-gray-500 dark:text-slate-400">No spec examples available</p>
                            <p className="text-xs text-gray-400 dark:text-slate-500">
                                This operation has no response examples or schema definitions in the spec.
                            </p>
                        </div>
                    )}

                    {!isLoading && examples.length > 0 && (
                        <ul className="divide-y divide-gray-100 dark:divide-slate-700">
                            {examples.map((ex) => (
                                <li key={ex.statusCode} className="p-4">
                                    <div className="flex items-start gap-3">
                                        {/* Status + meta */}
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-center gap-2 mb-1 flex-wrap">
                                                <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-semibold ${statusBadgeClass(ex.statusCode)}`}>
                                                    {statusLabel(ex.statusCode)}
                                                </span>
                                                {ex.contentType && (
                                                    <span className="text-xs text-gray-400 dark:text-slate-500 font-mono">
                                                        {ex.contentType}
                                                    </span>
                                                )}
                                                {ex.description && (
                                                    <span className="text-xs text-gray-500 dark:text-slate-400 truncate">
                                                        {ex.description}
                                                    </span>
                                                )}
                                            </div>

                                            {ex.schemaHint && !ex.bodyExample && (
                                                <p className="text-xs text-gray-400 dark:text-slate-500 italic mb-1">
                                                    Schema: {ex.schemaHint}
                                                </p>
                                            )}

                                            {ex.bodyExample && (
                                                <div className="mt-1">
                                                    <button
                                                        type="button"
                                                        onClick={() => toggle(ex.statusCode)}
                                                        className="flex items-center gap-1 text-xs text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200 transition-colors"
                                                    >
                                                        {expanded === ex.statusCode
                                                            ? <ChevronDown className="w-3.5 h-3.5" />
                                                            : <ChevronRight className="w-3.5 h-3.5" />}
                                                        {expanded === ex.statusCode ? 'Hide body' : 'Preview body'}
                                                    </button>
                                                    {expanded === ex.statusCode && (
                                                        <pre className="mt-2 text-xs font-mono bg-gray-50 dark:bg-slate-900 border border-gray-200 dark:border-slate-700 rounded p-3 overflow-x-auto max-h-48 text-gray-700 dark:text-slate-300 whitespace-pre-wrap break-all">
                                                            {ex.bodyExample}
                                                        </pre>
                                                    )}
                                                </div>
                                            )}

                                            {!ex.bodyExample && !ex.schemaHint && (
                                                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No body</p>
                                            )}
                                        </div>

                                        {/* Action */}
                                        <button
                                            type="button"
                                            onClick={() => { onSelect(ex); onClose(); }}
                                            className="flex-shrink-0 px-3 py-1.5 text-xs font-medium bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg transition-colors"
                                        >
                                            Use this
                                        </button>
                                    </div>
                                </li>
                            ))}
                        </ul>
                    )}
                </div>

                {/* Warning footer */}
                <div className="px-5 py-3 border-t border-amber-200 dark:border-amber-800/50 bg-amber-50 dark:bg-amber-900/20 rounded-b-xl">
                    <div className="flex items-start gap-2">
                        <AlertTriangle className="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" />
                        <p className="text-xs text-amber-700 dark:text-amber-300">
                            Selecting an example will <strong>override</strong> the current status code, response body, and Content-Type header.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    )
}
