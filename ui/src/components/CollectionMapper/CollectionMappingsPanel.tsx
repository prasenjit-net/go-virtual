import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Plus, Trash2, ToggleLeft, ToggleRight,
    Database, X, Loader2, Info, ChevronDown, ChevronRight,
} from 'lucide-react'
import clsx from 'clsx'
import { collectionMappingsApi, operationsApi } from '../../services/api'
import type { CollectionMapping, CollectionMappingInput, CollectionOpType, FieldMappingRule, Operation } from '../../types'

interface Props {
    operationId: string
    responseConfigId: string
    pendingMapping: CollectionMappingInput | null
    onPendingMappingChange: (m: CollectionMappingInput | null) => void
}

// ─── constants ───────────────────────────────────────────────────────────────

const OP_OPTIONS: { value: CollectionOpType; label: string }[] = [
    { value: 'insert',    label: 'Insert' },
    { value: 'find-one',  label: 'Find One' },
    { value: 'find-many', label: 'Find Many' },
    { value: 'update',    label: 'Update' },
    { value: 'upsert',    label: 'Upsert' },
    { value: 'delete',    label: 'Delete' },
]

const SOURCE_OPTIONS: { value: FieldMappingRule['sourceType']; label: string }[] = [
    { value: 'literal', label: 'Literal' },
    { value: 'path',    label: 'Path param' },
    { value: 'query',   label: 'Query param' },
    { value: 'header',  label: 'Header' },
    { value: 'body',    label: 'Body (gjson)' },
    { value: 'session', label: 'Session key' },
    { value: 'store',   label: 'Global store' },
]

const OP_LABELS: Record<CollectionOpType, string> = {
    insert: 'Insert', 'find-one': 'Find One', 'find-many': 'Find Many',
    update: 'Update', upsert: 'Upsert', delete: 'Delete',
}

const OP_COLORS: Record<CollectionOpType, string> = {
    insert:      'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    'find-one':  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    'find-many': 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
    update:      'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    upsert:      'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    delete:      'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

const EMPTY_RULE: FieldMappingRule = { targetField: '', sourceType: 'literal', sourceKey: '' }

const EMPTY_FORM: CollectionMappingInput = {
    collectionName: '', name: '', operation: 'insert',
    filterRules: [], dataRules: [], outputKey: '', order: 0, enabled: true,
}

function showFilterRules(op: CollectionOpType) { return op !== 'insert' }
function showDataRules(op: CollectionOpType)   { return op === 'insert' || op === 'update' || op === 'upsert' }

// ─── spec hints ──────────────────────────────────────────────────────────────

interface OperationHints { pathParams: string[]; queryParams: string[]; headerParams: string[]; bodyFields: string[] }

function hintsFromOperation(op: Operation | undefined): OperationHints {
    return {
        pathParams:   op?.declaredPathParams   ?? [],
        queryParams:  op?.declaredQueryParams  ?? [],
        headerParams: op?.declaredHeaderParams ?? [],
        bodyFields:   op?.declaredBodyFields   ?? [],
    }
}

function sourceKeyOptionsFor(t: FieldMappingRule['sourceType'], h: OperationHints): string[] {
    if (t === 'path')   return h.pathParams
    if (t === 'query')  return h.queryParams
    if (t === 'header') return h.headerParams
    if (t === 'body')   return h.bodyFields
    return []
}

// ─── rule editor ─────────────────────────────────────────────────────────────

function RuleEditor({ rules, onChange, label, hint, hints, idPrefix }: {
    rules: FieldMappingRule[]
    onChange: (r: FieldMappingRule[]) => void
    label: string; hint: string; hints: OperationHints; idPrefix: string
}) {
    const add    = () => onChange([...rules, { ...EMPTY_RULE }])
    const remove = (i: number) => onChange(rules.filter((_, idx) => idx !== i))
    const update = (i: number, patch: Partial<FieldMappingRule>) =>
        onChange(rules.map((r, idx) => idx === i ? { ...r, ...patch } : r))

    return (
        <div>
            <div className="flex items-center justify-between mb-2">
                <div>
                    <span className="text-xs font-semibold text-gray-700 dark:text-slate-300 uppercase tracking-wide">{label}</span>
                    <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">{hint}</p>
                </div>
                <button type="button" onClick={add} className="text-xs text-primary-600 dark:text-primary-400 hover:underline flex items-center gap-1">
                    <Plus className="w-3 h-3" /> Add rule
                </button>
            </div>
            {rules.length === 0 ? (
                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No rules — click "Add rule" to add one.</p>
            ) : (
                <div className="space-y-2">
                    {rules.map((rule, i) => {
                        const opts = sourceKeyOptionsFor(rule.sourceType, hints)
                        const dlId = `${idPrefix}-src-${i}`
                        return (
                            <div key={i} className="flex items-center gap-2">
                                <input
                                    type="text" value={rule.targetField} placeholder="Field name"
                                    onChange={(e) => update(i, { targetField: e.target.value })}
                                    className="flex-1 min-w-0 text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                                />
                                <select
                                    value={rule.sourceType}
                                    onChange={(e) => update(i, { sourceType: e.target.value as FieldMappingRule['sourceType'], sourceKey: '' })}
                                    className="text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                                >
                                    {SOURCE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                                </select>
                                <div className="flex-1 min-w-0">
                                    <input
                                        type="text" list={opts.length > 0 ? dlId : undefined}
                                        value={rule.sourceKey}
                                        placeholder={rule.sourceType === 'literal' ? 'value' : 'key / path'}
                                        onChange={(e) => update(i, { sourceKey: e.target.value })}
                                        className="w-full text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                                    />
                                    {opts.length > 0 && (
                                        <datalist id={dlId}>
                                            {opts.map((o) => <option key={o} value={o} />)}
                                        </datalist>
                                    )}
                                </div>
                                <button type="button" onClick={() => remove(i)} className="p-1 text-gray-400 hover:text-red-500 dark:hover:text-red-400 flex-shrink-0">
                                    <X className="w-3.5 h-3.5" />
                                </button>
                            </div>
                        )
                    })}
                </div>
            )}
        </div>
    )
}

// ─── mapping form ─────────────────────────────────────────────────────────────

function MappingForm({ form, onChange, hints, idPrefix }: {
    form: CollectionMappingInput; onChange: (f: CollectionMappingInput) => void
    hints: OperationHints; idPrefix: string
}) {
    const set = <K extends keyof CollectionMappingInput>(k: K, v: CollectionMappingInput[K]) =>
        onChange({ ...form, [k]: v })

    return (
        <div className="space-y-4">
            <div>
                <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Name <span className="text-gray-400">(optional)</span></label>
                <input type="text" value={form.name ?? ''} placeholder="e.g. Create User"
                    onChange={(e) => set('name', e.target.value)}
                    className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500"
                />
            </div>

            <div className="grid grid-cols-2 gap-3">
                <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Collection <span className="text-red-500">*</span></label>
                    <input type="text" value={form.collectionName} placeholder="e.g. users" required
                        onChange={(e) => set('collectionName', e.target.value)}
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                    />
                </div>
                <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Operation <span className="text-red-500">*</span></label>
                    <select value={form.operation} onChange={(e) => onChange({ ...form, operation: e.target.value as CollectionOpType })}
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                    >
                        {OP_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                    </select>
                </div>
            </div>

            <div>
                <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Output key <span className="text-red-500">*</span></label>
                <input type="text" value={form.outputKey} placeholder="e.g. user" required
                    onChange={(e) => set('outputKey', e.target.value)}
                    className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                />
                {form.outputKey && (
                    <p className="mt-1 text-xs text-gray-500 dark:text-slate-400">
                        Reference as <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1 rounded text-violet-700 dark:text-violet-400">{`{{.Collection.${form.outputKey}._id}}`}</code>
                    </p>
                )}
            </div>

            {showFilterRules(form.operation) && (
                <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-3">
                    <RuleEditor rules={form.filterRules} onChange={(r) => set('filterRules', r)}
                        label="Filter Rules" hint="Match documents where these fields equal the resolved values."
                        hints={hints} idPrefix={`${idPrefix}-filter`} />
                </div>
            )}

            {showDataRules(form.operation) && (
                <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-3">
                    <RuleEditor rules={form.dataRules} onChange={(r) => set('dataRules', r)}
                        label="Data Rules" hint="Set these fields on the inserted / updated document."
                        hints={hints} idPrefix={`${idPrefix}-data`} />
                </div>
            )}

            <div className="flex items-center gap-4">
                <div className="w-24">
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Order</label>
                    <input type="number" min={0} value={form.order}
                        onChange={(e) => set('order', parseInt(e.target.value, 10) || 0)}
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                    />
                </div>
                <div className="flex items-center gap-2 mt-4">
                    <input id={`${idPrefix}-enabled`} type="checkbox" checked={form.enabled}
                        onChange={(e) => set('enabled', e.target.checked)} className="rounded" />
                    <label htmlFor={`${idPrefix}-enabled`} className="text-sm text-gray-700 dark:text-slate-300">Enabled</label>
                </div>
            </div>

            {form.outputKey && (
                <div className="bg-violet-50 dark:bg-violet-950/20 border border-violet-200 dark:border-violet-900/40 rounded-lg p-3">
                    <div className="flex items-center gap-1.5 mb-2">
                        <Info className="w-3.5 h-3.5 text-violet-600 dark:text-violet-400" />
                        <span className="text-xs font-semibold text-violet-700 dark:text-violet-300">Template tokens</span>
                    </div>
                    <div className="space-y-1 text-xs font-mono text-violet-800 dark:text-violet-300">
                        {form.operation === 'find-many' ? (
                            <>
                                <div><code>{`{{range .Collection.${form.outputKey}}}`}</code></div>
                                <div className="ml-4"><code>{`{{._id}}`}</code></div>
                                <div><code>{`{{end}}`}</code></div>
                            </>
                        ) : (
                            <>
                                <div><code>{`{{.Collection.${form.outputKey}._id}}`}</code></div>
                                <div><code>{`{{.Collection.${form.outputKey}.<field>}}`}</code></div>
                            </>
                        )}
                    </div>
                </div>
            )}
        </div>
    )
}

// ─── panel ───────────────────────────────────────────────────────────────────

function CollectionMappingsPanel({ operationId, responseConfigId, pendingMapping, onPendingMappingChange }: Props) {
        const queryClient = useQueryClient()
        const queryKey = ['collectionMappings', responseConfigId]

        // addFormOpen is derived from the parent-controlled pendingMapping
        const addFormOpen = pendingMapping !== null
        const [addFormExpanded, setAddFormExpanded] = useState(true)

        // inline edit state — only one row open at a time
        const [editingId,  setEditingId]  = useState<string | null>(null)
        const [editForm,   setEditForm]   = useState<CollectionMappingInput>({ ...EMPTY_FORM })
        const [saveError,  setSaveError]  = useState<string | null>(null)

        const { data: mappings, isLoading } = useQuery<CollectionMapping[]>({
            queryKey,
            queryFn: () => collectionMappingsApi.listByResponse(operationId, responseConfigId),
        })

        const { data: operation } = useQuery<Operation>({
            queryKey: ['operation', operationId],
            queryFn: () => operationsApi.get(operationId),
            staleTime: 60_000,
        })

        const hints = hintsFromOperation(operation)
        // issue 3: always sort by order field
        const sortedMappings = (mappings ?? []).slice().sort((a, b) => a.order - b.order)

        const invalidate = () => queryClient.invalidateQueries({ queryKey })

        // mutations
        const updateMutation = useMutation({
            mutationFn: ({ id, data }: { id: string; data: CollectionMappingInput }) =>
                collectionMappingsApi.update(id, data),
            onSuccess: () => { invalidate(); setEditingId(null); setSaveError(null) },
            onError: (e) => setSaveError((e as Error).message),
        })

        const toggleMutation = useMutation({
            mutationFn: (m: CollectionMapping) => collectionMappingsApi.update(m.id, {
                collectionName: m.collectionName, name: m.name, operation: m.operation,
                filterRules: m.filterRules, dataRules: m.dataRules,
                outputKey: m.outputKey, order: m.order, enabled: !m.enabled,
            }),
            onSuccess: invalidate,
        })

        const deleteMutation = useMutation({
            mutationFn: (id: string) => collectionMappingsApi.delete(id),
            onSuccess: invalidate,
        })

        // issue 2: clicking a row opens its edit form and closes any other open one
        const handleRowClick = (m: CollectionMapping) => {
            if (editingId === m.id) {
                setEditingId(null)
            } else {
                setEditingId(m.id)
                setEditForm({
                    collectionName: m.collectionName, name: m.name ?? '',
                    operation: m.operation,
                    filterRules: m.filterRules ?? [], dataRules: m.dataRules ?? [],
                    outputKey: m.outputKey, order: m.order, enabled: m.enabled,
                })
                setSaveError(null)
            }
        }

        // open add form; close any open edit so list stays tidy
        const openAddForm = () => {
            onPendingMappingChange({ ...EMPTY_FORM, order: sortedMappings.length })
            setAddFormExpanded(true)
            setEditingId(null)
        }

        // issue 1: only collapse/expand — does NOT discard
        const toggleAddFormExpanded = () => setAddFormExpanded((v) => !v)

        // issue 5: explicit discard action (trash icon inside form)
        const discardPending = () => {
            onPendingMappingChange(null)
            setAddFormExpanded(true)
        }

        return (
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">

                {/* ── panel header ── */}
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-violet-100 dark:bg-violet-900/30 rounded-lg">
                            <Database className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                        </div>
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Collection Mappings</h2>
                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                                Map request fields to session-scoped collections. Output available as{' '}
                                <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">{'{{.Collection.<key>.*}}'}</code>
                            </p>
                        </div>
                    </div>
                    {/* Hide the button while a pending form is open — can't add two at once */}
                    {!addFormOpen && (
                        <button
                            onClick={openAddForm}
                            className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-violet-600 border border-violet-200 dark:border-violet-800 rounded-lg hover:bg-violet-50 dark:hover:bg-violet-900/30 transition-colors"
                        >
                            <Plus className="w-4 h-4" /> Add Mapping
                        </button>
                    )}
                </div>

                {/* ── existing mappings (sorted by order) ── */}
                {isLoading ? (
                    <div className="p-6">
                        <div className="animate-pulse space-y-3">
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                        </div>
                    </div>
                ) : sortedMappings.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {sortedMappings.map((m, idx) => {
                            const isExpanded = editingId === m.id
                            return (
                                <div key={m.id}>
                                    {/* ── row ── issue 2: whole left area is clickable ── */}
                                    <div className="flex items-center gap-3 px-4 py-3">
                                        {/* clickable region */}
                                        <button
                                            type="button"
                                            onClick={() => handleRowClick(m)}
                                            className="flex items-center gap-3 flex-1 min-w-0 text-left group"
                                        >
                                            <span className="w-5 text-center text-xs text-gray-400 dark:text-slate-500 flex-shrink-0 tabular-nums">{idx + 1}</span>
                                            <span className={clsx('text-xs font-semibold px-2 py-0.5 rounded-full flex-shrink-0', OP_COLORS[m.operation])}>
                                                {OP_LABELS[m.operation]}
                                            </span>
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2">
                                                    <span className="text-sm font-medium text-gray-900 dark:text-slate-100 group-hover:text-violet-700 dark:group-hover:text-violet-300 truncate transition-colors">
                                                        {m.name || m.collectionName}
                                                    </span>
                                                    {!m.enabled && (
                                                        <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 px-1.5 py-0.5 rounded-full flex-shrink-0">Disabled</span>
                                                    )}
                                                </div>
                                                <div className="flex items-center gap-2 mt-0.5 text-xs text-gray-500 dark:text-slate-400">
                                                    <code className="font-mono text-violet-700 dark:text-violet-400 bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded">{m.collectionName}</code>
                                                    <span>→</span>
                                                    <code className="font-mono text-violet-700 dark:text-violet-400 bg-gray-100 dark:bg-slate-800 px-1.5 py-0.5 rounded">{`{{.Collection.${m.outputKey}.*}}`}</code>
                                                </div>
                                            </div>
                                            {/* chevron shows expand state */}
                                            {isExpanded
                                                ? <ChevronDown  className="w-4 h-4 text-violet-500 flex-shrink-0" />
                                                : <ChevronRight className="w-4 h-4 text-gray-300 dark:text-slate-600 group-hover:text-violet-400 flex-shrink-0 transition-colors" />
                                            }
                                        </button>

                                        {/* action buttons — stopPropagation so they don't trigger row click */}
                                        <div className="flex items-center gap-1 flex-shrink-0">
                                            <button
                                                onClick={(e) => { e.stopPropagation(); toggleMutation.mutate(m) }}
                                                disabled={toggleMutation.isPending}
                                                className={clsx(
                                                    'p-1.5 rounded-lg transition-colors',
                                                    m.enabled
                                                        ? 'text-violet-600 hover:bg-violet-50 dark:hover:bg-violet-950/40'
                                                        : 'text-gray-400 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-slate-800'
                                                )}
                                                title={m.enabled ? 'Disable' : 'Enable'}
                                            >
                                                {m.enabled ? <ToggleRight className="w-4 h-4" /> : <ToggleLeft className="w-4 h-4" />}
                                            </button>
                                            <button
                                                onClick={(e) => {
                                                    e.stopPropagation()
                                                    if (confirm(`Delete mapping "${m.name || m.collectionName}"?`)) deleteMutation.mutate(m.id)
                                                }}
                                                className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                                title="Delete"
                                            >
                                                <Trash2 className="w-4 h-4" />
                                            </button>
                                        </div>
                                    </div>

                                    {/* ── inline edit form (issue 2: exclusive) ── */}
                                    {isExpanded && (
                                        <div className="mx-4 mb-3 border border-violet-200 dark:border-violet-800 rounded-lg bg-violet-50/40 dark:bg-violet-950/10 p-4 space-y-4">
                                            <MappingForm form={editForm} onChange={setEditForm} hints={hints} idPrefix={`edit-${m.id}`} />
                                            {saveError && <p className="text-sm text-red-600 dark:text-red-400">{saveError}</p>}
                                            <div className="flex items-center justify-end gap-2 pt-2 border-t border-violet-200 dark:border-violet-800">
                                                <button type="button" onClick={() => setEditingId(null)}
                                                    className="px-3 py-1.5 text-sm text-gray-600 dark:text-slate-300 border border-gray-300 dark:border-slate-600 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors">
                                                    Cancel
                                                </button>
                                                <button type="button" disabled={updateMutation.isPending}
                                                    onClick={() => updateMutation.mutate({ id: m.id, data: editForm })}
                                                    className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-white bg-violet-600 hover:bg-violet-700 disabled:opacity-50 rounded-lg transition-colors">
                                                    {updateMutation.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                                                    Update
                                                </button>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            )
                        })}
                    </div>
                ) : pendingMapping === null ? (
                    <div className="p-10 text-center">
                        <Database className="w-8 h-8 text-gray-300 dark:text-slate-700 mx-auto mb-3" />
                        <p className="text-sm text-gray-500 dark:text-slate-400">No collection mappings yet.</p>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">Click "Add Mapping" to map request fields to a session-scoped collection.</p>
                    </div>
                ) : null}

                {/* ── pending new-mapping form ── */}
                {addFormOpen && (
                    <div className="border-t border-amber-200 dark:border-amber-800/50 bg-amber-50/30 dark:bg-amber-950/5">

                        {/* issue 1: header only toggles expand/collapse — does NOT discard
                            issue 5: Trash2 (discard) sits on the right, same pattern as saved-mapper rows */}
                        <div className="flex items-center gap-2 px-4 py-3">
                            {/* left clickable area — collapse/expand only */}
                            <button
                                type="button"
                                onClick={toggleAddFormExpanded}
                                className="flex items-center gap-2 flex-1 min-w-0 text-left"
                            >
                                {addFormExpanded
                                    ? <ChevronDown  className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0" />
                                    : <ChevronRight className="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0" />
                                }
                                <span className="text-sm font-medium text-amber-800 dark:text-amber-300">New Mapping</span>
                                <span className="text-xs text-amber-600 dark:text-amber-500 bg-amber-100 dark:bg-amber-900/40 px-2 py-0.5 rounded-full">
                                    Unsaved — will be saved with the response
                                </span>
                            </button>

                            {/* issue 5: discard button inside the form — Trash2 icon, same style as saved-mapper delete */}
                            <button
                                type="button"
                                onClick={discardPending}
                                className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors flex-shrink-0"
                                title="Discard"
                            >
                                <Trash2 className="w-4 h-4" />
                            </button>
                        </div>

                        {/* form body — hidden when collapsed */}
                        {addFormExpanded && (
                            <div className="px-6 pb-6">
                                <MappingForm form={pendingMapping!} onChange={onPendingMappingChange} hints={hints} idPrefix="new" />
                            </div>
                        )}
                    </div>
                )}
            </div>
        )
}

export default CollectionMappingsPanel
