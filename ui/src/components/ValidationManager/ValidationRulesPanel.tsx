import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Plus, Trash2, Edit2, ToggleLeft, ToggleRight, ChevronDown, ChevronRight,
    ShieldCheck, Loader2, AlertCircle, X, Check
} from 'lucide-react'
import clsx from 'clsx'
import { validationRulesApi } from '../../services/api'
import ConditionEditor from '../shared/ConditionEditor'
import type { ConditionNode, ValidationInput, ValidationRule } from '../../types'

// ---- Props ----

interface SpecProps {
    scope: 'spec'
    specId: string
}

interface OperationProps {
    scope: 'operation'
    operationId: string
}

type Props = SpecProps | OperationProps

// ---- PropertyMapEditor ----

function PropertyMapEditor({
    label,
    value,
    onChange,
}: {
    label: string
    value: Record<string, string>
    onChange: (v: Record<string, string>) => void
}) {
    const entries = Object.entries(value)
    const addEntry = () => onChange({ ...value, '': '' })
    const removeEntry = (key: string) => {
        const next = { ...value }
        delete next[key]
        onChange(next)
    }
    const updateKey = (oldKey: string, newKey: string) => {
        const next: Record<string, string> = {}
        for (const [k, v] of Object.entries(value)) {
            next[k === oldKey ? newKey : k] = v
        }
        onChange(next)
    }
    const updateVal = (key: string, newVal: string) => onChange({ ...value, [key]: newVal })

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <label className="text-xs font-medium text-gray-600 dark:text-slate-400">{label}</label>
                <button
                    type="button"
                    onClick={addEntry}
                    className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
                >
                    <Plus className="w-3 h-3" /> Add
                </button>
            </div>
            {entries.length === 0 && (
                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No properties defined</p>
            )}
            <div className="space-y-1">
                {entries.map(([k, v], i) => (
                    <div key={i} className="flex items-center gap-1">
                        <input
                            type="text"
                            value={k}
                            placeholder="key"
                            onChange={e => updateKey(k, e.target.value)}
                            className="flex-1 px-2 py-1 text-xs border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                        />
                        <span className="text-gray-400 dark:text-slate-500 text-xs">=</span>
                        <input
                            type="text"
                            value={v}
                            placeholder="value"
                            onChange={e => updateVal(k, e.target.value)}
                            className="flex-1 px-2 py-1 text-xs border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                        />
                        <button type="button" onClick={() => removeEntry(k)} className="text-gray-400 hover:text-red-500">
                            <X className="w-3.5 h-3.5" />
                        </button>
                    </div>
                ))}
            </div>
        </div>
    )
}

// ---- RuleModal ----

const NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/

interface RuleModalProps {
    rule?: ValidationRule
    onSave: (input: ValidationInput) => void
    onClose: () => void
    isSaving: boolean
    saveError: string | null
}

function RuleModal({ rule, onSave, onClose, isSaving, saveError }: RuleModalProps) {
    const [name, setName] = useState(rule?.name ?? '')
    const [description, setDescription] = useState(rule?.description ?? '')
    const [order, setOrder] = useState(rule?.order ?? 0)
    const [enabled, setEnabled] = useState(rule?.enabled ?? true)
    const [conditionTree, setConditionTree] = useState<ConditionNode | undefined>(rule?.conditionTree)
    const [onSuccess, setOnSuccess] = useState<Record<string, string>>(rule?.onSuccess ?? {})
    const [onFailure, setOnFailure] = useState<Record<string, string>>(rule?.onFailure ?? {})
    const [nameError, setNameError] = useState<string | null>(null)

    const handleSubmit = () => {
        if (!name.trim()) { setNameError('Name is required'); return }
        if (!NAME_RE.test(name)) { setNameError('Name must match ^[a-zA-Z_][a-zA-Z0-9_]*$'); return }
        setNameError(null)
        onSave({ name, description, order, enabled, conditionTree, onSuccess, onFailure })
    }

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
            <div className="bg-white dark:bg-slate-800 rounded-xl shadow-2xl w-full max-w-2xl max-h-[90vh] flex flex-col border border-gray-200 dark:border-slate-700">
                {/* Header */}
                <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-slate-700">
                    <div className="flex items-center gap-2">
                        <ShieldCheck className="w-5 h-5 text-indigo-500" />
                        <h2 className="text-base font-semibold text-gray-900 dark:text-white">
                            {rule ? 'Edit Validation Rule' : 'New Validation Rule'}
                        </h2>
                    </div>
                    <button onClick={onClose} className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors">
                        <X className="w-4 h-4" />
                    </button>
                </div>

                {/* Body */}
                <div className="flex-1 overflow-y-auto px-5 py-4 space-y-4">
                    {/* Name + Order + Enabled */}
                    <div className="flex items-start gap-3">
                        <div className="flex-1">
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Name <span className="text-red-500">*</span>
                            </label>
                            <input
                                type="text"
                                value={name}
                                onChange={e => { setName(e.target.value); setNameError(null) }}
                                placeholder="e.g. auth_check"
                                className={clsx(
                                    'w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100',
                                    nameError
                                        ? 'border-red-500 focus:ring-red-500'
                                        : 'border-gray-300 dark:border-slate-700 focus:ring-indigo-500'
                                )}
                            />
                            {nameError && <p className="text-xs text-red-500 mt-1">{nameError}</p>}
                            <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                                Used as key in templates: <code className="font-mono">{`{{.Validation.${name || 'name'}.status}}`}</code>
                            </p>
                        </div>
                        <div className="w-24">
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Order</label>
                            <input
                                type="number"
                                value={order}
                                onChange={e => setOrder(Number(e.target.value))}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg text-sm bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100"
                            />
                        </div>
                        <div className="pt-6">
                            <button
                                type="button"
                                onClick={() => setEnabled(e => !e)}
                                className={clsx(
                                    'flex items-center gap-1.5 text-sm font-medium transition-colors',
                                    enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-400 dark:text-slate-500'
                                )}
                            >
                                {enabled
                                    ? <ToggleRight className="w-5 h-5" />
                                    : <ToggleLeft className="w-5 h-5" />}
                                {enabled ? 'Enabled' : 'Disabled'}
                            </button>
                        </div>
                    </div>

                    {/* Description */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">Description</label>
                        <input
                            type="text"
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            placeholder="Optional description"
                            className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg text-sm bg-white dark:bg-slate-900 text-gray-900 dark:text-slate-100"
                        />
                    </div>

                    {/* Condition Tree */}
                    <div>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mb-2">
                            When condition is <strong>true</strong> → onPass properties injected.
                            When <strong>false</strong> → onFail properties injected.
                        </p>
                        <ConditionEditor
                            label="Condition (when does this rule pass?)"
                            value={conditionTree}
                            onChange={setConditionTree}
                            emptyHint="No condition — rule always passes."
                        />
                    </div>

                    {/* OnSuccess / OnFailure */}
                    <div className="space-y-3">
                        <div className="border border-green-200 dark:border-green-800/50 rounded-lg p-3 bg-green-50 dark:bg-green-900/10">
                            <PropertyMapEditor
                                label="On Pass — properties injected when condition is true"
                                value={onSuccess}
                                onChange={setOnSuccess}
                            />
                        </div>
                        <div className="border border-red-200 dark:border-red-800/50 rounded-lg p-3 bg-red-50 dark:bg-red-900/10">
                            <PropertyMapEditor
                                label="On Fail — properties injected when condition is false"
                                value={onFailure}
                                onChange={setOnFailure}
                            />
                        </div>
                    </div>

                    {saveError && (
                        <div className="flex items-center gap-2 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-300">
                            <AlertCircle className="w-4 h-4 flex-shrink-0" />
                            {saveError}
                        </div>
                    )}
                </div>

                {/* Footer */}
                <div className="flex items-center justify-end gap-2 px-5 py-3 border-t border-gray-200 dark:border-slate-700">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-4 py-2 text-sm text-gray-600 dark:text-slate-400 hover:text-gray-800 dark:hover:text-slate-200"
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        onClick={handleSubmit}
                        disabled={isSaving}
                        className="px-4 py-2 text-sm font-medium bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg disabled:opacity-60 flex items-center gap-1.5"
                    >
                        {isSaving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}
                        {rule ? 'Update' : 'Create'}
                    </button>
                </div>
            </div>
        </div>
    )
}

// ---- Main Panel ----

export default function ValidationRulesPanel(props: Props) {
    const queryClient = useQueryClient()
    const [modalOpen, setModalOpen] = useState(false)
    const [editingRule, setEditingRule] = useState<ValidationRule | null>(null)
    const [expandedId, setExpandedId] = useState<string | null>(null)
    const [mutationError, setMutationError] = useState<string | null>(null)

    const isSpec = props.scope === 'spec'
    const id = isSpec ? (props as SpecProps).specId : (props as OperationProps).operationId
    const queryKey = isSpec ? ['specValidations', id] : ['operationValidations', id]

    const { data: rules = [], isLoading } = useQuery({
        queryKey,
        queryFn: () => isSpec
            ? validationRulesApi.listBySpec(id)
            : validationRulesApi.listByOperation(id),
    })

    const createMutation = useMutation({
        mutationFn: (input: ValidationInput) => isSpec
            ? validationRulesApi.createForSpec(id, input)
            : validationRulesApi.createForOperation(id, input),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey })
            setModalOpen(false)
            setMutationError(null)
        },
        onError: (err: Error) => setMutationError(err.message),
    })

    const updateMutation = useMutation({
        mutationFn: ({ ruleId, input }: { ruleId: string; input: ValidationInput }) =>
            validationRulesApi.update(ruleId, input),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey })
            setModalOpen(false)
            setEditingRule(null)
            setMutationError(null)
        },
        onError: (err: Error) => setMutationError(err.message),
    })

    const deleteMutation = useMutation({
        mutationFn: (ruleId: string) => validationRulesApi.delete(ruleId),
        onSuccess: () => queryClient.invalidateQueries({ queryKey }),
    })

    const toggleMutation = useMutation({
        mutationFn: ({ rule, enabled }: { rule: ValidationRule; enabled: boolean }) =>
            validationRulesApi.update(rule.id, {
                name: rule.name,
                description: rule.description,
                order: rule.order,
                enabled,
                conditionTree: rule.conditionTree,
                onSuccess: rule.onSuccess,
                onFailure: rule.onFailure,
            }),
        onSuccess: () => queryClient.invalidateQueries({ queryKey }),
    })

    const sorted = [...rules].sort((a, b) => a.order - b.order)

    const openCreate = () => { setEditingRule(null); setMutationError(null); setModalOpen(true) }
    const openEdit = (rule: ValidationRule) => { setEditingRule(rule); setMutationError(null); setModalOpen(true) }
    const closeModal = () => { setModalOpen(false); setEditingRule(null); setMutationError(null) }

    const handleSave = (input: ValidationInput) => {
        if (editingRule) {
            updateMutation.mutate({ ruleId: editingRule.id, input })
        } else {
            createMutation.mutate(input)
        }
    }

    const isSaving = createMutation.isPending || updateMutation.isPending

    return (
        <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 mt-6">
            {/* Panel header */}
            <div className="p-4 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <ShieldCheck className="w-4 h-4 text-indigo-500" />
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100">Validation Rules</h3>
                    <span className="text-xs px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400">
                        {isSpec ? 'spec-scoped' : 'operation-scoped'}
                    </span>
                    {rules.length > 0 && (
                        <span className="text-xs px-1.5 py-0.5 rounded-full bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300">
                            {rules.length}
                        </span>
                    )}
                </div>
                <button
                    type="button"
                    onClick={openCreate}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg transition-colors"
                >
                    <Plus className="w-3.5 h-3.5" /> New Rule
                </button>
            </div>

            {/* Body */}
            <div className="p-4">
                {isLoading && (
                    <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-slate-500">
                        <Loader2 className="w-4 h-4 animate-spin" /> Loading…
                    </div>
                )}

                {!isLoading && sorted.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-8 text-center">
                        <ShieldCheck className="w-8 h-8 text-gray-200 dark:text-slate-700 mb-2" />
                        <p className="text-sm text-gray-400 dark:text-slate-500">No validation rules yet</p>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                            Rules run at step ⑥ of the request pipeline and inject properties into
                            <code className="font-mono mx-1">{`{{.Validation.<name>.*}}`}</code> templates.
                        </p>
                    </div>
                )}

                {!isLoading && sorted.length > 0 && (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {sorted.map(rule => {
                            const isExpanded = expandedId === rule.id
                            return (
                                <div key={rule.id} className="py-2">
                                    <div className="flex items-center gap-2">
                                        {/* Order badge */}
                                        <span className="text-xs font-mono text-gray-400 dark:text-slate-500 w-6 text-right flex-shrink-0">
                                            {rule.order}
                                        </span>

                                        {/* Expand toggle */}
                                        <button
                                            type="button"
                                            onClick={() => setExpandedId(isExpanded ? null : rule.id)}
                                            className="text-gray-400 hover:text-gray-600 dark:hover:text-slate-300"
                                        >
                                            {isExpanded
                                                ? <ChevronDown className="w-3.5 h-3.5" />
                                                : <ChevronRight className="w-3.5 h-3.5" />}
                                        </button>

                                        {/* Name */}
                                        <code className="text-sm font-mono font-medium text-gray-800 dark:text-slate-200 flex-1">
                                            {rule.name}
                                        </code>

                                        {/* Description */}
                                        {rule.description && (
                                            <span className="text-xs text-gray-400 dark:text-slate-500 truncate max-w-xs hidden sm:block">
                                                {rule.description}
                                            </span>
                                        )}

                                        {/* Toggle enabled */}
                                        <button
                                            type="button"
                                            onClick={() => toggleMutation.mutate({ rule, enabled: !rule.enabled })}
                                            className={clsx(
                                                'transition-colors',
                                                rule.enabled
                                                    ? 'text-green-500 hover:text-green-600'
                                                    : 'text-gray-300 dark:text-slate-600 hover:text-gray-400'
                                            )}
                                        >
                                            {rule.enabled
                                                ? <ToggleRight className="w-4 h-4" />
                                                : <ToggleLeft className="w-4 h-4" />}
                                        </button>

                                        {/* Edit */}
                                        <button
                                            type="button"
                                            onClick={() => openEdit(rule)}
                                            className="p-1 text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400 rounded"
                                        >
                                            <Edit2 className="w-3.5 h-3.5" />
                                        </button>

                                        {/* Delete */}
                                        <button
                                            type="button"
                                            onClick={() => deleteMutation.mutate(rule.id)}
                                            className="p-1 text-gray-400 hover:text-red-500 rounded"
                                        >
                                            <Trash2 className="w-3.5 h-3.5" />
                                        </button>
                                    </div>

                                    {/* Expanded detail */}
                                    {isExpanded && (
                                        <div className="mt-2 ml-14 text-xs text-gray-500 dark:text-slate-400 space-y-1">
                                            {rule.description && (
                                                <p>{rule.description}</p>
                                            )}
                                            <div className="flex items-center gap-4">
                                                {rule.onSuccess && Object.keys(rule.onSuccess).length > 0 && (
                                                    <span className="text-green-600 dark:text-green-400">
                                                        Pass → {Object.entries(rule.onSuccess).map(([k, v]) => `${k}=${v}`).join(', ')}
                                                    </span>
                                                )}
                                                {rule.onFailure && Object.keys(rule.onFailure).length > 0 && (
                                                    <span className="text-red-500 dark:text-red-400">
                                                        Fail → {Object.entries(rule.onFailure).map(([k, v]) => `${k}=${v}`).join(', ')}
                                                    </span>
                                                )}
                                            </div>
                                            <p className="text-gray-400 dark:text-slate-500">
                                                Template: <code className="font-mono">{`{{.Validation.${rule.name}.status}}`}</code>
                                            </p>
                                        </div>
                                    )}
                                </div>
                            )
                        })}
                    </div>
                )}
            </div>

            {/* Modal */}
            {modalOpen && (
                <RuleModal
                    rule={editingRule ?? undefined}
                    onSave={handleSave}
                    onClose={closeModal}
                    isSaving={isSaving}
                    saveError={mutationError}
                />
            )}
        </div>
    )
}
