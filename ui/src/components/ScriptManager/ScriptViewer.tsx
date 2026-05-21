import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import Editor from '@monaco-editor/react'
import {
    ArrowLeft, Edit2, Trash2, Clock, ToggleLeft, ToggleRight, Code2
} from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi } from '../../services/api'
import type { Script } from '../../types'
import { useState } from 'react'
import { useIsDark } from '../../hooks/useIsDark'

export default function ScriptViewer() {
    const { scriptId } = useParams<{ scriptId: string }>()
    const navigate = useNavigate()
    const queryClient = useQueryClient()
    const [pendingDelete, setPendingDelete] = useState(false)
    const isDarkTheme = useIsDark()

    const { data: script, isLoading, error } = useQuery<Script>({
        queryKey: ['script', scriptId],
        queryFn: () => scriptsApi.get(scriptId!),
        enabled: !!scriptId,
        staleTime: 0,
    })

    const toggleMutation = useMutation({
        mutationFn: (s: Script) =>
            scriptsApi.update(s.id, {
                name: s.name,
                description: s.description,
                source: s.source ?? '',
                timeout: s.timeout,
                enabled: !s.enabled,
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['scripts'] })
            queryClient.invalidateQueries({ queryKey: ['script', scriptId] })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (id: string) => scriptsApi.delete(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['scripts'] })
            navigate('/scripts')
        },
    })

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-56"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                    <div className="h-64 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (error || !script) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Failed to load script: {(error as Error)?.message ?? 'Not found'}
                </div>
            </div>
        )
    }

    return (
        <div className="p-8 max-w-5xl">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
                <div className="flex items-center gap-4 min-w-0">
                    <button
                        onClick={() => navigate('/scripts')}
                        className="p-2 text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors flex-shrink-0"
                    >
                        <ArrowLeft className="w-5 h-5" />
                    </button>
                    <div className="min-w-0">
                        <div className="flex items-center gap-2.5">
                            <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100 truncate">
                                {script.name}
                            </h1>
                            {!script.enabled && (
                                <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 px-2 py-0.5 rounded-full flex-shrink-0">
                                    Disabled
                                </span>
                            )}
                        </div>
                        {script.description && (
                            <p className="text-gray-500 dark:text-slate-400 mt-0.5 text-sm truncate">
                                {script.description}
                            </p>
                        )}
                    </div>
                </div>

                {/* Action buttons */}
                <div className="flex items-center gap-2 flex-shrink-0 ml-4">
                    {/* Toggle enable/disable */}
                    <button
                        onClick={() => toggleMutation.mutate(script)}
                        disabled={toggleMutation.isPending}
                        className={clsx(
                            'p-2 rounded-lg transition-colors',
                            script.enabled
                                ? 'text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/40'
                                : 'text-gray-400 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-slate-800'
                        )}
                        title={script.enabled ? 'Disable script' : 'Enable script'}
                    >
                        {script.enabled
                            ? <ToggleRight className="w-5 h-5" />
                            : <ToggleLeft className="w-5 h-5" />
                        }
                    </button>

                    {/* Delete */}
                    <button
                        onClick={() => setPendingDelete(true)}
                        className="p-2 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                        title="Delete script"
                    >
                        <Trash2 className="w-5 h-5" />
                    </button>

                    {/* Edit — primary CTA */}
                    <button
                        onClick={() => navigate(`/scripts/${scriptId}/edit`)}
                        className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors text-sm font-medium"
                    >
                        <Edit2 className="w-4 h-4" />
                        Edit Script
                    </button>
                </div>
            </div>

            <div className="space-y-6">
                {/* Metadata card */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <div className="flex items-center gap-3 mb-4">
                        <div className={clsx(
                            'p-2 rounded-lg',
                            script.enabled
                                ? 'bg-emerald-100 dark:bg-emerald-900/30'
                                : 'bg-gray-100 dark:bg-slate-800'
                        )}>
                            <Code2 className={clsx(
                                'w-5 h-5',
                                script.enabled
                                    ? 'text-emerald-600 dark:text-emerald-400'
                                    : 'text-gray-400 dark:text-slate-500'
                            )} />
                        </div>
                        <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">Details</h2>
                    </div>
                    <dl className="grid grid-cols-2 sm:grid-cols-3 gap-x-6 gap-y-4 text-sm">
                        <div>
                            <dt className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase tracking-wide mb-0.5">Status</dt>
                            <dd className={clsx(
                                'font-medium',
                                script.enabled
                                    ? 'text-emerald-600 dark:text-emerald-400'
                                    : 'text-gray-500 dark:text-slate-400'
                            )}>
                                {script.enabled ? 'Enabled' : 'Disabled'}
                            </dd>
                        </div>
                        <div>
                            <dt className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase tracking-wide mb-0.5">
                                <span className="flex items-center gap-1"><Clock className="w-3 h-3" /> Timeout</span>
                            </dt>
                            <dd className="text-gray-900 dark:text-slate-100">
                                {script.timeout > 0 ? `${script.timeout} ms` : 'Global default'}
                            </dd>
                        </div>
                        <div>
                            <dt className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase tracking-wide mb-0.5">Script ID</dt>
                            <dd className="font-mono text-xs text-gray-400 dark:text-slate-500 break-all">{script.id}</dd>
                        </div>
                    </dl>
                </div>

                {/* Source — read-only editor */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 overflow-hidden">
                    <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100 dark:border-slate-800">
                        <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">Source</h2>
                        <span className="text-xs text-gray-400 dark:text-slate-500 bg-gray-100 dark:bg-slate-800 px-2 py-0.5 rounded font-mono">
                            Starlark
                        </span>
                    </div>
                    <div className="h-[420px]">
                        <Editor
                            language="python"
                            value={script.source ?? ''}
                            options={{
                                readOnly: true,
                                minimap: { enabled: false },
                                fontSize: 13,
                                lineNumbers: 'on',
                                scrollBeyondLastLine: false,
                                wordWrap: 'on',
                                renderLineHighlight: 'none',
                                cursorStyle: 'line',
                                domReadOnly: true,
                            }}
                            theme={isDarkTheme ? 'vs-dark' : 'light'}
                        />
                    </div>
                    <div className="px-6 py-3 border-t border-gray-100 dark:border-slate-800 flex justify-end">
                        <button
                            onClick={() => navigate(`/scripts/${scriptId}/edit`)}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors text-sm font-medium"
                        >
                            <Edit2 className="w-4 h-4" />
                            Edit Script
                        </button>
                    </div>
                </div>
            </div>

            {/* Delete confirmation modal */}
            {pendingDelete && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-md p-6">
                        <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100 mb-2">Delete Script</h3>
                        <p className="text-gray-600 dark:text-slate-400 mb-1">
                            Are you sure you want to delete <strong className="text-gray-900 dark:text-slate-100">{script.name}</strong>?
                        </p>
                        <p className="text-sm text-amber-600 dark:text-amber-400 mb-6">
                            This will also remove all operation bindings that reference this script.
                        </p>
                        <div className="flex gap-3 justify-end">
                            <button
                                onClick={() => setPendingDelete(false)}
                                className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => deleteMutation.mutate(script.id)}
                                disabled={deleteMutation.isPending}
                                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition-colors"
                            >
                                {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
