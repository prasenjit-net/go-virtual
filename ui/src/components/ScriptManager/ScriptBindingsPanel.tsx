import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
    Plus, Trash2, ToggleLeft, ToggleRight, ChevronUp, ChevronDown,
    Code2, ExternalLink, X, Loader2, Info
} from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi, scriptBindingsApi } from '../../services/api'
import type { Script, ScriptBinding } from '../../types'

interface Props {
    operationId: string
}

interface AttachForm {
    scriptId: string
    outputKey: string
    order: number
    enabled: boolean
}

export default function ScriptBindingsPanel({ operationId }: Props) {
    const queryClient = useQueryClient()
    const [modalOpen, setModalOpen] = useState(false)
    const [form, setForm] = useState<AttachForm>({ scriptId: '', outputKey: '', order: 0, enabled: true })
    const [formError, setFormError] = useState<string | null>(null)

    // All available scripts (for the attach dropdown)
    const { data: allScripts } = useQuery<Script[]>({
        queryKey: ['scripts'],
        queryFn: scriptsApi.list,
    })

    // Bindings for this operation
    const { data: bindings, isLoading } = useQuery<ScriptBinding[]>({
        queryKey: ['scriptBindings', operationId],
        queryFn: () => scriptBindingsApi.listByOperation(operationId),
    })

    const invalidate = () => {
        queryClient.invalidateQueries({ queryKey: ['scriptBindings', operationId] })
    }

    const toggleMutation = useMutation({
        mutationFn: ({ binding }: { binding: ScriptBinding }) =>
            scriptBindingsApi.update(operationId, binding.id, {
                scriptId: binding.scriptId,
                outputKey: binding.outputKey,
                order: binding.order,
                enabled: !binding.enabled,
            }),
        onSuccess: invalidate,
    })

    const deleteMutation = useMutation({
        mutationFn: (bindingId: string) => scriptBindingsApi.delete(operationId, bindingId),
        onSuccess: invalidate,
    })

    const attachMutation = useMutation({
        mutationFn: () => scriptBindingsApi.create(operationId, form),
        onSuccess: () => {
            invalidate()
            setModalOpen(false)
            setForm({ scriptId: '', outputKey: '', order: (bindings?.length ?? 0), enabled: true })
            setFormError(null)
        },
        onError: (e) => setFormError((e as Error).message),
    })

    const reorderMutation = useMutation({
        mutationFn: (items: { id: string; order: number }[]) =>
            scriptBindingsApi.reorder(operationId, items),
        onSuccess: invalidate,
    })

    const openModal = () => {
        setForm({
            scriptId: allScripts?.[0]?.id ?? '',
            outputKey: '',
            order: bindings?.length ?? 0,
            enabled: true,
        })
        setFormError(null)
        setModalOpen(true)
    }

    const moveBinding = (binding: ScriptBinding, direction: 'up' | 'down') => {
        if (!bindings) return
        const sorted = [...bindings].sort((a, b) => a.order - b.order)
        const idx = sorted.findIndex((b) => b.id === binding.id)
        if (direction === 'up' && idx === 0) return
        if (direction === 'down' && idx === sorted.length - 1) return

        const swapIdx = direction === 'up' ? idx - 1 : idx + 1
        const newOrder = sorted.map((b, i) => ({ id: b.id, order: i }))
        // Swap orders
        const temp = newOrder[idx].order
        newOrder[idx].order = newOrder[swapIdx].order
        newOrder[swapIdx].order = temp

        reorderMutation.mutate(newOrder)
    }

    const sortedBindings = bindings
        ? [...bindings].sort((a, b) => a.order - b.order)
        : []

    return (
        <>
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-emerald-100 dark:bg-emerald-900/30 rounded-lg">
                            <Code2 className="w-5 h-5 text-emerald-600 dark:text-emerald-400" />
                        </div>
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Script Bindings</h2>
                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                                Scripts run in order before the response is rendered. Output is available as{' '}
                                <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">{'{{.script.<key>.*}}'}</code>
                            </p>
                        </div>
                    </div>
                    <button
                        onClick={openModal}
                        className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-emerald-600 border border-emerald-200 dark:border-emerald-800 rounded-lg hover:bg-emerald-50 dark:hover:bg-emerald-900/30 transition-colors"
                    >
                        <Plus className="w-4 h-4" />
                        Attach Script
                    </button>
                </div>

                {isLoading ? (
                    <div className="p-6">
                        <div className="animate-pulse space-y-3">
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg"></div>
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg"></div>
                        </div>
                    </div>
                ) : sortedBindings.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {sortedBindings.map((binding, idx) => (
                            <div key={binding.id} className="flex items-center gap-3 px-6 py-4">
                                {/* Order controls */}
                                <div className="flex flex-col gap-0.5 flex-shrink-0">
                                    <button
                                        onClick={() => moveBinding(binding, 'up')}
                                        disabled={idx === 0 || reorderMutation.isPending}
                                        className="p-0.5 text-gray-300 dark:text-slate-600 hover:text-gray-600 dark:hover:text-slate-300 disabled:opacity-30 transition-colors"
                                    >
                                        <ChevronUp className="w-4 h-4" />
                                    </button>
                                    <button
                                        onClick={() => moveBinding(binding, 'down')}
                                        disabled={idx === sortedBindings.length - 1 || reorderMutation.isPending}
                                        className="p-0.5 text-gray-300 dark:text-slate-600 hover:text-gray-600 dark:hover:text-slate-300 disabled:opacity-30 transition-colors"
                                    >
                                        <ChevronDown className="w-4 h-4" />
                                    </button>
                                </div>

                                {/* Order badge */}
                                <span className="w-6 text-center text-xs text-gray-400 dark:text-slate-500 flex-shrink-0 tabular-nums">
                                    {idx + 1}
                                </span>

                                {/* Main content */}
                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                        <span className="text-sm font-medium text-gray-900 dark:text-slate-100 truncate">
                                            {binding.scriptName || binding.scriptId}
                                        </span>
                                        {!binding.enabled && (
                                            <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 px-1.5 py-0.5 rounded-full flex-shrink-0">
                                                Disabled
                                            </span>
                                        )}
                                    </div>
                                    <div className="flex items-center gap-2 mt-0.5 text-xs text-gray-500 dark:text-slate-400">
                                        <span>Output key:</span>
                                        <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded text-emerald-700 dark:text-emerald-400">
                                            {`{{.script.${binding.outputKey}.*}}`}
                                        </code>
                                        <Link
                                            to={`/scripts/${binding.scriptId}/edit`}
                                            className="text-primary-500 hover:text-primary-600 flex items-center gap-0.5"
                                            title="Edit script"
                                        >
                                            <ExternalLink className="w-3 h-3" />
                                        </Link>
                                    </div>
                                </div>

                                {/* Actions */}
                                <div className="flex items-center gap-1 flex-shrink-0">
                                    <button
                                        onClick={() => toggleMutation.mutate({ binding })}
                                        disabled={toggleMutation.isPending}
                                        className={clsx(
                                            'p-1.5 rounded-lg transition-colors',
                                            binding.enabled
                                                ? 'text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/40'
                                                : 'text-gray-400 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-slate-800'
                                        )}
                                        title={binding.enabled ? 'Disable' : 'Enable'}
                                    >
                                        {binding.enabled
                                            ? <ToggleRight className="w-4 h-4" />
                                            : <ToggleLeft className="w-4 h-4" />
                                        }
                                    </button>
                                    <button
                                        onClick={() => {
                                            if (confirm(`Remove binding for "${binding.scriptName || binding.scriptId}"?`)) {
                                                deleteMutation.mutate(binding.id)
                                            }
                                        }}
                                        className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                        title="Remove binding"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                    </button>
                                </div>
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="p-10 text-center">
                        <Code2 className="w-8 h-8 text-gray-300 dark:text-slate-700 mx-auto mb-3" />
                        <p className="text-sm text-gray-500 dark:text-slate-400 mb-1">No scripts attached to this operation</p>
                        <p className="text-xs text-gray-400 dark:text-slate-500">
                            Attach a script to enrich responses with dynamic computed data.
                        </p>
                    </div>
                )}

                {/* Info bar */}
                {sortedBindings.length > 0 && (
                    <div className="px-6 py-3 border-t border-gray-100 dark:border-slate-800 flex items-center gap-2 text-xs text-gray-400 dark:text-slate-500">
                        <Info className="w-3.5 h-3.5 flex-shrink-0" />
                        Scripts execute in listed order on every matched request. Access output in templates via{' '}
                        <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1 rounded">
                            {`{{.script.<outputKey>.fieldName}}`}
                        </code>
                    </div>
                )}
            </div>

            {/* Attach Script Modal */}
            {modalOpen && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-md p-6">
                        <div className="flex items-center justify-between mb-5">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Attach Script</h3>
                            <button
                                onClick={() => setModalOpen(false)}
                                className="p-1.5 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                            >
                                <X className="w-4 h-4" />
                            </button>
                        </div>

                        <div className="space-y-4">
                            {/* Script selector */}
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                    Script <span className="text-red-500">*</span>
                                </label>
                                {allScripts && allScripts.length > 0 ? (
                                    <select
                                        value={form.scriptId}
                                        onChange={(e) => setForm({ ...form, scriptId: e.target.value })}
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                    >
                                        <option value="">Select a script…</option>
                                        {allScripts.map((s) => (
                                            <option key={s.id} value={s.id} disabled={!s.enabled}>
                                                {s.name}{!s.enabled ? ' (disabled)' : ''}
                                            </option>
                                        ))}
                                    </select>
                                ) : (
                                    <div className="text-sm text-gray-500 dark:text-slate-400 p-3 border border-gray-200 dark:border-slate-700 rounded-lg">
                                        No scripts available.{' '}
                                        <Link to="/scripts/new" className="text-primary-600 hover:underline">Create one first</Link>.
                                    </div>
                                )}
                            </div>

                            {/* Output key */}
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                    Output Key <span className="text-red-500">*</span>
                                </label>
                                <input
                                    type="text"
                                    value={form.outputKey}
                                    onChange={(e) => setForm({ ...form, outputKey: e.target.value.replace(/[^a-zA-Z0-9_]/g, '') })}
                                    placeholder="e.g. userData"
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono"
                                />
                                {form.outputKey && (
                                    <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                                        Access output via <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1 rounded text-emerald-700 dark:text-emerald-400">
                                            {`{{.script.${form.outputKey}.fieldName}}`}
                                        </code>
                                    </p>
                                )}
                            </div>

                            {/* Order */}
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                    Execution Order
                                </label>
                                <input
                                    type="number"
                                    value={form.order}
                                    min={0}
                                    onChange={(e) => setForm({ ...form, order: parseInt(e.target.value) || 0 })}
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                />
                                <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">Lower = executes first</p>
                            </div>

                            {/* Enabled toggle */}
                            <div className="flex items-center gap-3">
                                <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                                <button
                                    type="button"
                                    onClick={() => setForm({ ...form, enabled: !form.enabled })}
                                    className={clsx(
                                        'transition-colors',
                                        form.enabled
                                            ? 'text-emerald-600 hover:text-emerald-700'
                                            : 'text-gray-400 dark:text-slate-500 hover:text-gray-600'
                                    )}
                                >
                                    {form.enabled
                                        ? <ToggleRight className="w-7 h-7" />
                                        : <ToggleLeft className="w-7 h-7" />
                                    }
                                </button>
                            </div>

                            {formError && (
                                <div className="text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3">
                                    {formError}
                                </div>
                            )}
                        </div>

                        <div className="flex gap-3 mt-6">
                            <button
                                onClick={() => attachMutation.mutate()}
                                disabled={attachMutation.isPending || !form.scriptId || !form.outputKey}
                                className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
                            >
                                {attachMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                                {attachMutation.isPending ? 'Attaching…' : 'Attach'}
                            </button>
                            <button
                                onClick={() => setModalOpen(false)}
                                className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </>
    )
}
