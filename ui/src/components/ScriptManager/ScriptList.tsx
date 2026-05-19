import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { Plus, Code2, Trash2, ToggleLeft, ToggleRight, Clock, FileCode } from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi } from '../../services/api'
import type { Script } from '../../types'

export default function ScriptList() {
    const queryClient = useQueryClient()
    const navigate = useNavigate()
    const [pendingDelete, setPendingDelete] = useState<Script | null>(null)

    const { data: scripts, isLoading, error } = useQuery<Script[]>({
        queryKey: ['scripts'],
        queryFn: scriptsApi.list,
    })

    const toggleMutation = useMutation({
        mutationFn: ({ script }: { script: Script }) =>
            scriptsApi.update(script.id, {
                name: script.name,
                description: script.description,
                source: script.source ?? '',
                timeout: script.timeout,
                enabled: !script.enabled,
            }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['scripts'] })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (id: string) => scriptsApi.delete(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['scripts'] })
            setPendingDelete(null)
        },
    })

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-24 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                    <div className="h-24 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Failed to load scripts: {(error as Error).message}
                </div>
            </div>
        )
    }

    return (
        <div className="p-8">
            <div className="flex items-center justify-between mb-8">
                <div className="flex items-start gap-3">
                    <div className="rounded-lg bg-primary-100 p-3 dark:bg-primary-900/30">
                        <Code2 className="h-6 w-6 text-primary-600 dark:text-primary-400" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Scripts</h1>
                        <p className="mt-1 text-gray-500 dark:text-slate-400 text-sm">
                            Manage Starlark scripts that can be attached to API operations
                        </p>
                    </div>
                </div>
                <Link
                    to="/scripts/new"
                    className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                >
                    <Plus className="w-5 h-5 mr-2" />
                    New Script
                </Link>
            </div>

            {scripts && scripts.length > 0 ? (
                <div className="space-y-3">
                    {scripts.map((script) => (
                        <div
                            key={script.id}
                            onClick={() => navigate(`/scripts/${script.id}/edit`)}
                            className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-5 cursor-pointer hover:border-primary-300 dark:hover:border-primary-700 hover:shadow-md transition-all"
                        >
                            <div className="flex items-center justify-between">
                                <div className="flex items-center min-w-0">
                                    <div className={clsx(
                                        'p-2.5 rounded-lg flex-shrink-0',
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
                                    <div className="ml-4 min-w-0">
                                        <div className="flex items-center gap-2">
                                            <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 truncate">
                                                {script.name}
                                            </h3>
                                            {!script.enabled && (
                                                <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 px-2 py-0.5 rounded-full flex-shrink-0">
                                                    Disabled
                                                </span>
                                            )}
                                        </div>
                                        {script.description && (
                                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5 truncate">
                                                {script.description}
                                            </p>
                                        )}
                                        <div className="flex items-center gap-3 mt-1.5 text-xs text-gray-400 dark:text-slate-500">
                                            {script.timeout > 0 && (
                                                <span className="flex items-center gap-1">
                                                    <Clock className="w-3 h-3" />
                                                    {script.timeout}ms timeout
                                                </span>
                                            )}
                                            <span className="font-mono text-gray-300 dark:text-slate-600">{script.id}</span>
                                        </div>
                                    </div>
                                </div>

                                <div className="flex items-center gap-1 flex-shrink-0 ml-4">
                                    {/* Click to open hint */}
                                    <span className="text-xs text-gray-300 dark:text-slate-600 mr-1 hidden sm:flex items-center gap-1">
                                        Open
                                    </span>

                                    {/* Enable/disable toggle */}
                                    <button
                                        onClick={(e) => { e.stopPropagation(); toggleMutation.mutate({ script }) }}
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
                                        onClick={(e) => { e.stopPropagation(); setPendingDelete(script) }}
                                        className="p-2 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                        title="Delete script"
                                    >
                                        <Trash2 className="w-5 h-5" />
                                    </button>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            ) : (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-16 text-center">
                    <FileCode className="w-12 h-12 text-gray-300 dark:text-slate-600 mx-auto mb-4" />
                    <h3 className="text-lg font-medium text-gray-900 dark:text-slate-100 mb-2">No scripts yet</h3>
                    <p className="text-gray-500 dark:text-slate-400 mb-6 max-w-md mx-auto">
                        Scripts let you run Starlark code per request and inject dynamic values into response templates via{' '}
                        <code className="font-mono text-sm bg-gray-100 dark:bg-slate-800 px-1 rounded">{'{{.script.<key>.*}}'}</code>.
                    </p>
                    <Link
                        to="/scripts/new"
                        className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        Create First Script
                    </Link>
                </div>
            )}

            {/* Delete confirmation modal */}
            {pendingDelete && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-md p-6">
                        <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100 mb-2">Delete Script</h3>
                        <p className="text-gray-600 dark:text-slate-400 mb-1">
                            Are you sure you want to delete <strong className="text-gray-900 dark:text-slate-100">{pendingDelete.name}</strong>?
                        </p>
                        <p className="text-sm text-amber-600 dark:text-amber-400 mb-6">
                            This will also remove all operation bindings that reference this script.
                        </p>
                        <div className="flex gap-3 justify-end">
                            <button
                                onClick={() => setPendingDelete(null)}
                                className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => deleteMutation.mutate(pendingDelete.id)}
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
