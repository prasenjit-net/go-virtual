import { useState, forwardRef, useImperativeHandle } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Plus, Trash2, ToggleLeft, ToggleRight,
    Database, X, Loader2, Pencil, Info, ChevronDown,
} from 'lucide-react'
import clsx from 'clsx'
import { collectionMappingsApi, operationsApi } from '../../services/api'
import type { CollectionMapping, CollectionMappingInput, CollectionOpType, FieldMappingRule, Operation } from '../../types'

interface Props {
    operationId: string
    responseConfigId: string
}

export interface CollectionMappingsPanelHandle {
    savePending: () => Promise<void>
}

const OP_OPTIONS: { value: CollectionOpType; label: string }[] = [
    { value: 'insert', label: 'Insert' },
    { value: 'find-one', label: 'Find One' },
    { value: 'find-many', label: 'Find Many' },
    { value: 'update', label: 'Update' },
    { value: 'upsert', label: 'Upsert' },
    { value: 'delete', label: 'Delete' },
]

const SOURCE_OPTIONS: { value: FieldMappingRule['sourceType']; label: string }[] = [
    { value: 'literal', label: 'Literal' },
    { value: 'path', label: 'Path param' },
    { value: 'query', label: 'Query param' },
    { value: 'header', label: 'Header' },
    { value: 'body', label: 'Body (gjson)' },
    { value: 'session', label: 'Session key' },
    { value: 'store', label: 'Global store' },
]

const OP_LABELS: Record<CollectionOpType, string> = {
    insert: 'Insert',
    'find-one': 'Find One',
    'find-many': 'Find Many',
    update: 'Update',
    upsert: 'Upsert',
    delete: 'Delete',
}

const OP_COLORS: Record<CollectionOpType, string> = {
    insert: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    'find-one': 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    'find-many': 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
    update: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    upsert: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    delete: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

function showFilterRules(op: CollectionOpType) { return op !== 'insert' }
function showDataRules(op: CollectionOpType) { return op === 'insert' || op === 'update' || op === 'upsert' }

const EMPTY_RULE: FieldMappingRule = { targetField: '', sourceType: 'literal', sourceKey: '' }

const EMPTY_FORM: CollectionMappingInput = {
    collectionName: '',
    name: '',
    operation: 'insert',
    filterRules: [],
    dataRules: [],
    outputKey: '',
    order: 0,
    enabled: true,
}

interface OperationHints {
    pathParams: string[]
    queryParams: string[]
    headerParams: string[]
    bodyFields: string[]
}

function hintsFromOperation(op: Operation | undefined): OperationHints {
    return {
        pathParams: op?.declaredPathParams ?? [],
        queryParams: op?.declaredQueryParams ?? [],
        headerParams: op?.declaredHeaderParams ?? [],
        bodyFields: op?.declaredBodyFields ?? [],
    }
}

function sourceKeyOptionsFor(sourceType: FieldMappingRule['sourceType'], hints: OperationHints): string[] {
    switch (sourceType) {
        case 'path': return hints.pathParams
        case 'query': return hints.queryParams
        case 'header': return hints.headerParams
        case 'body': return hints.bodyFields
        default: return []
    }
}

function RuleEditor({
    rules,
    onChange,
    label,
    hint,
    hints,
    ruleIndex,
}: {
    rules: FieldMappingRule[]
    onChange: (rules: FieldMappingRule[]) => void
    label: string
    hint: string
    hints: OperationHints
    ruleIndex: string
}) {
    const addRule = () => onChange([...rules, { ...EMPTY_RULE }])
    const removeRule = (i: number) => onChange(rules.filter((_, idx) => idx !== i))
    const updateRule = (i: number, patch: Partial<FieldMappingRule>) => {
        onChange(rules.map((r, idx) => idx === i ? { ...r, ...patch } : r))
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-2">
                <div>
                    <span className="text-xs font-semibold text-gray-700 dark:text-slate-300 uppercase tracking-wide">{label}</span>
                    <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">{hint}</p>
                </div>
                <button type="button" onClick={addRule} className="text-xs text-primary-600 dark:text-primary-400 hover:underline flex items-center gap-1">
                    <Plus className="w-3 h-3" /> Add rule
                </button>
            </div>
            {rules.length === 0 ? (
                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No rules — click "Add rule" to add one.</p>
            ) : (
                <div className="space-y-2">
                    {rules.map((rule, i) => {
                        const sourceOptions = sourceKeyOptionsFor(rule.sourceType, hints)
                        const datalistId = `${ruleIndex}-src-${i}`
                        return (
                            <div key={i} className="flex items-center gap-2">
                                <input
                                    type="text"
                                    value={rule.targetField}
                                    onChange={(e) => updateRule(i, { targetField: e.target.value })}
                                    placeholder="Field name"
                                    className="flex-1 min-w-0 text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                                />
                                <select
                                    value={rule.sourceType}
                                    onChange={(e) => updateRule(i, { sourceType: e.target.value as FieldMappingRule['sourceType'], sourceKey: '' })}
                                    className="text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                                >
                                    {SOURCE_OPTIONS.map((o) => (
                                        <option key={o.value} value={o.value}>{o.label}</option>
                                    ))}
                                </select>
                                <div className="flex-1 min-w-0">
                                    <input
                                        type="text"
                                        list={sourceOptions.length > 0 ? datalistId : undefined}
                                        value={rule.sourceKey}
                                        onChange={(e) => updateRule(i, { sourceKey: e.target.value })}
                                        placeholder={rule.sourceType === 'literal' ? 'value' : 'key / path'}
                                        className="w-full text-xs px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                                    />
                                    {sourceOptions.length > 0 && (
                                        <datalist id={datalistId}>
                                            {sourceOptions.map((opt) => <option key={opt} value={opt} />)}
                                        </datalist>
                                    )}
                                </div>
                                <button type="button" onClick={() => removeRule(i)} className="p-1 text-gray-400 hover:text-red-500 dark:hover:text-red-400 flex-shrink-0">
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

interface MappingFormProps {
    form: CollectionMappingInput
    onChange: (form: CollectionMappingInput) => void
    hints: OperationHints
    idPrefix: string
}

function MappingForm({ form, onChange, hints, idPrefix }: MappingFormProps) {
    const set = <K extends keyof CollectionMappingInput>(k: K, v: CollectionMappingInput[K]) =>
        onChange({ ...form, [k]: v })

    return (
        <div className="space-y-4">
            <div>
                <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Name <span className="text-gray-400">(optional)</span></label>
                <input
                    type="text"
                    value={form.name ?? ''}
                    onChange={(e) => set('name', e.target.value)}
                    placeholder="e.g. Create User"
                    className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500"
                />
            </div>

            <div className="grid grid-cols-2 gap-3">
                <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Collection <span className="text-red-500">*</span></label>
                    <input
                        type="text"
                        value={form.collectionName}
                        onChange={(e) => set('collectionName', e.target.value)}
                        placeholder="e.g. users"
                        required
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 font-mono"
                    />
                </div>
                <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Operation <span className="text-red-500">*</span></label>
                    <select
                        value={form.operation}
                        onChange={(e) => onChange({ ...form, operation: e.target.value as CollectionOpType })}
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                    >
                        {OP_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                    </select>
                </div>
            </div>

            <div>
                <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Output key <span className="text-red-500">*</span></label>
                <input
                    type="text"
                    value={form.outputKey}
                    onChange={(e) => set('outputKey', e.target.value)}
                    placeholder="e.g. user"
                    required
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
                    <RuleEditor
                        rules={form.filterRules}
                        onChange={(r) => set('filterRules', r)}
                        label="Filter Rules"
                        hint="Match documents where these fields equal the resolved values."
                        hints={hints}
                        ruleIndex={`${idPrefix}-filter`}
                    />
                </div>
            )}

            {showDataRules(form.operation) && (
                <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-3">
                    <RuleEditor
                        rules={form.dataRules}
                        onChange={(r) => set('dataRules', r)}
                        label="Data Rules"
                        hint="Set these fields on the inserted / updated document."
                        hints={hints}
                        ruleIndex={`${idPrefix}-data`}
                    />
                </div>
            )}

            <div className="flex items-center gap-4">
                <div className="w-24">
                    <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Order</label>
                    <input
                        type="number"
                        min={0}
                        value={form.order}
                        onChange={(e) => set('order', parseInt(e.target.value, 10) || 0)}
                        className="w-full text-sm px-3 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                    />
                </div>
                <div className="flex items-center gap-2 mt-4">
                    <input
                        id={`${idPrefix}-enabled`}
                        type="checkbox"
                        checked={form.enabled}
                        onChange={(e) => set('enabled', e.target.checked)}
                        className="rounded"
                    />
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

const CollectionMappingsPanel = forwardRef<CollectionMappingsPanelHandle, Props>(
    function CollectionMappingsPanel({ operationId, responseConfigId }, ref) {
        const queryClient = useQueryClient()
        const queryKey = ['collectionMappings', responseConfigId]

        const [addFormOpen, setAddFormOpen] = useState(false)
        const [pendingForm, setPendingForm] = useState<CollectionMappingInput>({ ...EMPTY_FORM })
        const [editingId, setEditingId] = useState<string | null>(null)
        const [editForm, setEditForm] = useState<CollectionMappingInput>({ ...EMPTY_FORM })
        const [saveError, setSaveError] = useState<string | null>(null)

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
        const sortedMappings = mappings ? [...mappings].sort((a, b) => a.order - b.order) : []

        const invalidate = () => queryClient.invalidateQueries({ queryKey })

        useImperativeHandle(ref, () => ({
            savePending: async () => {
                if (!addFormOpen) return
                if (!pendingForm.collectionName || !pendingForm.outputKey || !pendingForm.operation) return
                await collectionMappingsApi.create(operationId, responseConfigId, {
                    ...pendingForm,
                    order: pendingForm.order ?? sortedMappings.length,
                })
                invalidate()
                setAddFormOpen(false)
                setPendingForm({ ...EMPTY_FORM })
            },
        }), [addFormOpen, pendingForm, operationId, responseConfigId, sortedMappings.length])

        const updateMutation = useMutation({
            mutationFn: ({ id, data }: { id: string; data: CollectionMappingInput }) =>
                collectionMappingsApi.update(id, data),
            onSuccess: () => { invalidate(); setEditingId(null); setSaveError(null) },
            onError: (e) => setSaveError((e as Error).message),
        })

        const toggleMutation = useMutation({
            mutationFn: (m: CollectionMapping) => collectionMappingsApi.update(m.id, {
                collectionName: m.collectionName,
                name: m.name,
                operation: m.operation,
                filterRules: m.filterRules,
                dataRules: m.dataRules,
                outputKey: m.outputKey,
                order: m.order,
                enabled: !m.enabled,
            }),
            onSuccess: invalidate,
        })

        const deleteMutation = useMutation({
            mutationFn: (id: string) => collectionMappingsApi.delete(id),
            onSuccess: invalidate,
        })

        const openEdit = (m: CollectionMapping) => {
            if (editingId === m.id) { setEditingId(null); return }
            setEditingId(m.id)
            setEditForm({
                collectionName: m.collectionName,
                name: m.name ?? '',
                operation: m.operation,
                filterRules: m.filterRules ?? [],
                dataRules: m.dataRules ?? [],
                outputKey: m.outputKey,
                order: m.order,
                enabled: m.enabled,
            })
            setSaveError(null)
        }

        const handleAddClick = () => {
            if (addFormOpen) {
                setAddFormOpen(false)
                setPendingForm({ ...EMPTY_FORM })
            } else {
                setPendingForm({ ...EMPTY_FORM, order: sortedMappings.length })
                setAddFormOpen(true)
                setEditingId(null)
            }
        }

        return (
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                {/* Header */}
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
                    <button
                        onClick={handleAddClick}
                        className={clsx(
                            'inline-flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg border transition-colors',
                            addFormOpen
                                ? 'text-gray-600 border-gray-300 dark:border-slate-600 hover:bg-gray-50 dark:hover:bg-slate-800'
                                : 'text-violet-600 border-violet-200 dark:border-violet-800 hover:bg-violet-50 dark:hover:bg-violet-900/30'
                        )}
                    >
                        {addFormOpen ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
                        {addFormOpen ? 'Discard' : 'Add Mapping'}
                    </button>
                </div>

                {/* Existing mappings list */}
                {isLoading ? (
                    <div className="p-6">
                        <div className="animate-pulse space-y-3">
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                        </div>
                    </div>
                ) : sortedMappings.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {sortedMappings.map((m, idx) => (
                            <div key={m.id}>
                                {/* Row */}
                                <div className="flex items-center gap-3 px-6 py-3">
                                    <span className="w-6 text-center text-xs text-gray-400 dark:text-slate-500 flex-shrink-0 tabular-nums">{idx + 1}</span>
                                    <span className={clsx('text-xs font-semibold px-2 py-0.5 rounded-full flex-shrink-0', OP_COLORS[m.operation])}>
                                        {OP_LABELS[m.operation]}
                                    </span>
                                    <div className="flex-1 min-w-0">
                                        <div className="flex items-center gap-2">
                                            <span className="text-sm font-medium text-gray-900 dark:text-slate-100 truncate">
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
                                    <div className="flex items-center gap-1 flex-shrink-0">
                                        <button
                                            onClick={() => openEdit(m)}
                                            className={clsx(
                                                'p-1.5 rounded-lg transition-colors',
                                                editingId === m.id
                                                    ? 'text-violet-600 bg-violet-50 dark:bg-violet-950/40'
                                                    : 'text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800'
                                            )}
                                            title="Edit"
                                        >
                                            <Pencil className="w-4 h-4" />
                                        </button>
                                        <button
                                            onClick={() => toggleMutation.mutate(m)}
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
                                            onClick={() => { if (confirm(`Delete mapping "${m.name || m.collectionName}"?`)) deleteMutation.mutate(m.id) }}
                                            className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                            title="Delete"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                    </div>
                                </div>

                                {/* Inline edit form */}
                                {editingId === m.id && (
                                    <div className="mx-4 mb-3 border border-violet-200 dark:border-violet-800 rounded-lg bg-violet-50/40 dark:bg-violet-950/10 p-4 space-y-4">
                                        <MappingForm
                                            form={editForm}
                                            onChange={setEditForm}
                                            hints={hints}
                                            idPrefix={`edit-${m.id}`}
                                        />
                                        {saveError && <p className="text-sm text-red-600 dark:text-red-400">{saveError}</p>}
                                        <div className="flex items-center justify-end gap-2 pt-2 border-t border-violet-200 dark:border-violet-800">
                                            <button
                                                type="button"
                                                onClick={() => setEditingId(null)}
                                                className="px-3 py-1.5 text-sm text-gray-600 dark:text-slate-300 border border-gray-300 dark:border-slate-600 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                                            >
                                                Cancel
                                            </button>
                                            <button
                                                type="button"
                                                disabled={updateMutation.isPending}
                                                onClick={() => updateMutation.mutate({ id: m.id, data: editForm })}
                                                className="inline-flex items-center gap-2 px-3 py-1.5 text-sm text-white bg-violet-600 hover:bg-violet-700 disabled:opacity-50 rounded-lg transition-colors"
                                            >
                                                {updateMutation.isPending && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                                                Update
                                            </button>
                                        </div>
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>
                ) : !addFormOpen ? (
                    <div className="p-10 text-center">
                        <Database className="w-8 h-8 text-gray-300 dark:text-slate-700 mx-auto mb-3" />
                        <p className="text-sm text-gray-500 dark:text-slate-400">No collection mappings yet.</p>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">Click "Add Mapping" to map request fields to a session-scoped collection.</p>
                    </div>
                ) : null}

                {/* Pending new mapping form */}
                {addFormOpen && (
                    <div className="border-t border-amber-200 dark:border-amber-800/50 bg-amber-50/50 dark:bg-amber-950/10">
                        {/* Collapsible header */}
                        <button
                            type="button"
                            onClick={() => setAddFormOpen(false)}
                            className="w-full flex items-center justify-between px-6 py-3 text-left hover:bg-amber-50 dark:hover:bg-amber-950/20 transition-colors"
                        >
                            <div className="flex items-center gap-2">
                                <span className="text-sm font-medium text-amber-800 dark:text-amber-300">New Mapping</span>
                                <span className="text-xs text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/40 px-2 py-0.5 rounded-full">
                                    Unsaved — will be saved with the response
                                </span>
                            </div>
                            <ChevronDown className="w-4 h-4 text-amber-600 dark:text-amber-400" />
                        </button>

                        <div className="px-6 pb-6">
                            <MappingForm
                                form={pendingForm}
                                onChange={setPendingForm}
                                hints={hints}
                                idPrefix="new"
                            />
                        </div>
                    </div>
                )}
            </div>
        )
    }
)

export default CollectionMappingsPanel
