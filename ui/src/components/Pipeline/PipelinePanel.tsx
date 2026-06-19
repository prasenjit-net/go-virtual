import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    DndContext,
    DragOverlay,
    PointerSensor,
    closestCenter,
    useSensor,
    useSensors,
    type DragEndEvent,
    type DragStartEvent,
} from '@dnd-kit/core'
import {
    SortableContext,
    arrayMove,
    useSortable,
    verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
    GripVertical, Plus, Trash2, ToggleLeft, ToggleRight,
    ChevronUp, ChevronDown, Code2, ShieldCheck, Database,
    Layers, X, Loader2, ExternalLink,
} from 'lucide-react'
import clsx from 'clsx'
import { Link } from 'react-router-dom'
import {
    pipelineApi,
    scriptsApi,
    scriptBindingsApi,
    specScriptBindingsApi,
    responseScriptBindingsApi,
    validationRulesApi,
    collectionMappingsApi,
} from '../../services/api'
import ConditionEditor from '../shared/ConditionEditor'
import type {
    PipelineStep,
    PipelineScope,
    PipelineReorderItem,
    PipelineStepType,
    Script,
    ScriptBindingInput,
    ValidationInput,
    CollectionMappingInput,
    CollectionOpType,
    FieldMappingRule,
    ConditionNode,
} from '../../types'

// ── Constants ────────────────────────────────────────────────────────────────

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

const EMPTY_RULE: FieldMappingRule = { targetField: '', sourceType: 'literal', sourceKey: '' }

const EMPTY_COLL_FORM: CollectionMappingInput = {
    collectionName: '', name: '', operation: 'insert',
    filterRules: [], dataRules: [], outputKey: '', order: 0, enabled: true,
}

// ── Type badge ────────────────────────────────────────────────────────────────

function TypeBadge({ type }: { type: PipelineStepType }) {
    if (type === 'script') return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 whitespace-nowrap flex-shrink-0">
            <Code2 className="w-3 h-3" />SCRIPT
        </span>
    )
    if (type === 'validation') return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 whitespace-nowrap flex-shrink-0">
            <ShieldCheck className="w-3 h-3" />VALIDATION
        </span>
    )
    return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300 whitespace-nowrap flex-shrink-0">
            <Database className="w-3 h-3" />COLLECTION
        </span>
    )
}

function stepLabel(step: PipelineStep): string {
    if (step.type === 'script' && step.script) return step.script.scriptName || step.script.scriptId
    if (step.type === 'validation' && step.validation) return step.validation.name
    if (step.type === 'collection' && step.collection) return step.collection.name || step.collection.collectionName
    return '—'
}

function stepSubtext(step: PipelineStep): string {
    if (step.type === 'script' && step.script) return `→ .script.${step.script.outputKey}`
    if (step.type === 'validation' && step.validation) return 'condition rule'
    if (step.type === 'collection' && step.collection) {
        const m = step.collection
        return `${m.operation} ${m.collectionName} → .collection.${m.outputKey}`
    }
    return ''
}

// ── Field rule editor (for collection mapping modal) ──────────────────────────

function RuleEditor({ rules, onChange, label }: {
    rules: FieldMappingRule[]
    onChange: (r: FieldMappingRule[]) => void
    label: string
}) {
    const add    = () => onChange([...rules, { ...EMPTY_RULE }])
    const remove = (i: number) => onChange(rules.filter((_, idx) => idx !== i))
    const update = (i: number, patch: Partial<FieldMappingRule>) =>
        onChange(rules.map((r, idx) => idx === i ? { ...r, ...patch } : r))

    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <label className="text-xs font-medium text-gray-600 dark:text-slate-400">{label}</label>
                <button type="button" onClick={add} className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-0.5">
                    <Plus className="w-3 h-3" /> Add
                </button>
            </div>
            {rules.length === 0 && (
                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No rules</p>
            )}
            <div className="space-y-1.5">
                {rules.map((rule, i) => (
                    <div key={i} className="flex gap-1 items-center">
                        <input
                            type="text"
                            value={rule.targetField}
                            placeholder="target field"
                            onChange={e => update(i, { targetField: e.target.value })}
                            className="flex-1 min-w-0 px-2 py-1 text-xs border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                        />
                        <span className="text-gray-400 text-xs">=</span>
                        <select
                            value={rule.sourceType}
                            onChange={e => update(i, { sourceType: e.target.value as FieldMappingRule['sourceType'], sourceKey: '' })}
                            className="px-1.5 py-1 text-xs border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                        >
                            {SOURCE_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                        </select>
                        <input
                            type="text"
                            value={rule.sourceKey}
                            placeholder="value / key"
                            onChange={e => update(i, { sourceKey: e.target.value })}
                            className="flex-1 min-w-0 px-2 py-1 text-xs border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                        />
                        <button type="button" onClick={() => remove(i)} className="p-0.5 text-gray-400 hover:text-red-500">
                            <X className="w-3 h-3" />
                        </button>
                    </div>
                ))}
            </div>
        </div>
    )
}

// ── PropertyMapEditor (for validation modal) ──────────────────────────────────

function PropertyMapEditor({ label, value, onChange }: {
    label: string
    value: Record<string, string>
    onChange: (v: Record<string, string>) => void
}) {
    const entries = Object.entries(value)
    return (
        <div>
            <div className="flex items-center justify-between mb-1">
                <label className="text-xs font-medium text-gray-600 dark:text-slate-400">{label}</label>
                <button type="button" onClick={() => onChange({ ...value, '': '' })}
                    className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-0.5">
                    <Plus className="w-3 h-3" /> Add
                </button>
            </div>
            {entries.length === 0 && (
                <p className="text-xs text-gray-400 dark:text-slate-500 italic">No properties</p>
            )}
            <div className="space-y-1">
                {entries.map(([k, v], i) => (
                    <div key={i} className="flex items-center gap-1">
                        <input type="text" value={k} placeholder="key"
                            onChange={e => {
                                const next: Record<string, string> = {}
                                for (const [ek, ev] of entries) next[ek === k ? e.target.value : ek] = ev
                                onChange(next)
                            }}
                            className="flex-1 min-w-0 px-2 py-1 text-xs border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                        />
                        <span className="text-xs text-gray-400">=</span>
                        <input type="text" value={v} placeholder="value"
                            onChange={e => onChange({ ...value, [k]: e.target.value })}
                            className="flex-1 min-w-0 px-2 py-1 text-xs border border-gray-300 dark:border-slate-600 rounded bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100"
                        />
                        <button type="button"
                            onClick={() => { const n = { ...value }; delete n[k]; onChange(n) }}
                            className="p-0.5 text-gray-400 hover:text-red-500">
                            <X className="w-3 h-3" />
                        </button>
                    </div>
                ))}
            </div>
        </div>
    )
}

// ── Sortable step card ─────────────────────────────────────────────────────────

interface StepCardProps {
    step: PipelineStep
    index: number
    total: number
    scope: PipelineScope
    onToggle: () => void
    onDelete: () => void
    onEdit: () => void
    onMove?: (dir: 'up' | 'down') => void
    isDragOverlay?: boolean
    dragHandleListeners?: ReturnType<typeof useSortable>['listeners']
    dragHandleAttributes?: ReturnType<typeof useSortable>['attributes']
    innerRef?: (el: HTMLDivElement | null) => void
    style?: React.CSSProperties
}

function StepCard({
    step, index, total, scope,
    onToggle, onDelete, onEdit, onMove,
    isDragOverlay = false,
    dragHandleListeners, dragHandleAttributes,
    innerRef, style,
}: StepCardProps) {
    const showDrag = scope !== 'response'

    return (
        <div
            ref={innerRef}
            style={style}
            className={clsx(
                'flex items-center gap-3 px-4 py-3',
                isDragOverlay && 'shadow-xl rounded-lg bg-white dark:bg-slate-900 border border-primary-200 dark:border-primary-700 opacity-95',
                !step.script?.enabled && !step.validation?.enabled && !step.collection?.enabled && 'opacity-60',
            )}
        >
            {/* Drag handle or up/down */}
            {showDrag ? (
                <button
                    {...dragHandleListeners}
                    {...dragHandleAttributes}
                    className="flex-shrink-0 cursor-grab text-gray-300 dark:text-slate-600 hover:text-gray-500 dark:hover:text-slate-400 transition-colors"
                    tabIndex={-1}
                >
                    <GripVertical className="w-4 h-4" />
                </button>
            ) : (
                <div className="flex flex-col gap-0.5 flex-shrink-0">
                    <button onClick={() => onMove?.('up')} disabled={index === 0}
                        className="p-0.5 text-gray-300 dark:text-slate-600 hover:text-gray-600 dark:hover:text-slate-300 disabled:opacity-30 transition-colors">
                        <ChevronUp className="w-4 h-4" />
                    </button>
                    <button onClick={() => onMove?.('down')} disabled={index === total - 1}
                        className="p-0.5 text-gray-300 dark:text-slate-600 hover:text-gray-600 dark:hover:text-slate-300 disabled:opacity-30 transition-colors">
                        <ChevronDown className="w-4 h-4" />
                    </button>
                </div>
            )}

            {/* Position number */}
            <span className="w-5 text-center text-xs text-gray-400 dark:text-slate-500 flex-shrink-0 tabular-nums">
                {index + 1}
            </span>

            {/* Type badge */}
            <TypeBadge type={step.type} />

            {/* Name + subtext */}
            <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-sm font-medium text-gray-900 dark:text-slate-100 truncate">
                        {stepLabel(step)}
                    </span>
                    {step.type === 'script' && step.script && (
                        <Link
                            to={`/scripts/${step.script.scriptId}/edit`}
                            className="text-gray-400 hover:text-primary-500 transition-colors"
                            title="Open script"
                        >
                            <ExternalLink className="w-3 h-3" />
                        </Link>
                    )}
                </div>
                <p className="text-xs text-gray-500 dark:text-slate-400 font-mono truncate">
                    {stepSubtext(step)}
                </p>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-1 flex-shrink-0">
                {step.type !== 'script' && (
                    <button onClick={onEdit}
                        className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-primary-600 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-950/30 transition-colors"
                        title="Edit">
                        <ChevronDown className="w-4 h-4 rotate-270" />
                    </button>
                )}
                <button
                    onClick={onToggle}
                    className={clsx(
                        'p-1.5 rounded-lg transition-colors',
                        (step.script?.enabled || step.validation?.enabled || step.collection?.enabled)
                            ? 'text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/40'
                            : 'text-gray-400 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-slate-800',
                    )}
                    title="Toggle enabled"
                >
                    {(step.script?.enabled || step.validation?.enabled || step.collection?.enabled)
                        ? <ToggleRight className="w-4 h-4" />
                        : <ToggleLeft className="w-4 h-4" />
                    }
                </button>
                <button
                    onClick={onDelete}
                    className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                    title="Remove"
                >
                    <Trash2 className="w-4 h-4" />
                </button>
            </div>
        </div>
    )
}

function SortableStepCard(props: Omit<StepCardProps, 'dragHandleListeners' | 'dragHandleAttributes' | 'innerRef' | 'style'>) {
    const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: stepId(props.step) })
    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.3 : 1,
    }
    return (
        <StepCard
            {...props}
            innerRef={setNodeRef}
            style={style}
            dragHandleListeners={listeners}
            dragHandleAttributes={attributes}
        />
    )
}

function stepId(step: PipelineStep): string {
    if (step.script) return `script:${step.script.id}`
    if (step.validation) return `validation:${step.validation.id}`
    if (step.collection) return `collection:${step.collection.id}`
    return `unknown:${step.order}`
}

function stepEntityId(step: PipelineStep): string {
    return step.script?.id ?? step.validation?.id ?? step.collection?.id ?? ''
}

// ── Main component ────────────────────────────────────────────────────────────

interface Props {
    scope: PipelineScope
    scopeId: string
    /** Required when scope === 'response' — the parent operation's ID */
    operationId?: string
}

type AddModalType = 'script' | 'validation' | 'collection' | null

interface ScriptForm {
    scriptId: string
    outputKey: string
    order: number
    enabled: boolean
}

interface ValidationForm {
    name: string
    order: number
    enabled: boolean
    conditionTree: ConditionNode | undefined
    onSuccess: Record<string, string>
    onFailure: Record<string, string>
}

const EMPTY_VALIDATION_FORM: ValidationForm = {
    name: '', order: 0, enabled: true,
    conditionTree: undefined,
    onSuccess: {}, onFailure: {},
}

export default function PipelinePanel({ scope, scopeId, operationId }: Props) {
    const queryClient = useQueryClient()
    const queryKey = ['pipeline', scope, scopeId]

    const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
    const [activeStep, setActiveStep] = useState<PipelineStep | null>(null)
    const [addModal, setAddModal] = useState<AddModalType>(null)
    const [editingStep, setEditingStep] = useState<PipelineStep | null>(null)

    // Script form state
    const [scriptForm, setScriptForm] = useState<ScriptForm>({ scriptId: '', outputKey: '', order: 0, enabled: true })
    const [scriptError, setScriptError] = useState<string | null>(null)

    // Validation form state
    const [valForm, setValForm] = useState<ValidationForm>(EMPTY_VALIDATION_FORM)
    const [valError, setValError] = useState<string | null>(null)

    // Collection form state
    const [collForm, setCollForm] = useState<CollectionMappingInput>(EMPTY_COLL_FORM)
    const [collError, setCollError] = useState<string | null>(null)

    // ── Data ──
    const { data, isLoading } = useQuery({
        queryKey,
        queryFn: () => pipelineApi.list(scope, scopeId),
        enabled: !!scopeId,
    })
    const steps = data?.steps ?? []

    const { data: allScripts } = useQuery<Script[]>({
        queryKey: ['scripts'],
        queryFn: scriptsApi.list,
    })

    const invalidate = () => queryClient.invalidateQueries({ queryKey })

    // ── Reorder ──
    const reorderMutation = useMutation({
        mutationFn: (items: PipelineReorderItem[]) => pipelineApi.reorder(scope, scopeId, items),
        onSuccess: invalidate,
    })

    const handleDragStart = (e: DragStartEvent) => {
        const id = e.active.id as string
        setActiveStep(steps.find((s) => stepId(s) === id) ?? null)
    }

    const handleDragEnd = (e: DragEndEvent) => {
        setActiveStep(null)
        const { active, over } = e
        if (!over || active.id === over.id) return
        const oldIdx = steps.findIndex((s) => stepId(s) === active.id)
        const newIdx = steps.findIndex((s) => stepId(s) === over.id)
        if (oldIdx === -1 || newIdx === -1) return
        const reordered = arrayMove(steps, oldIdx, newIdx)
        reorderMutation.mutate(reordered.map((s) => ({
            type: s.type,
            id: stepEntityId(s),
        })))
    }

    // ── Up/down move (response scope) ──
    const moveStep = (step: PipelineStep, dir: 'up' | 'down') => {
        const idx = steps.findIndex((s) => stepId(s) === stepId(step))
        if (dir === 'up' && idx === 0) return
        if (dir === 'down' && idx === steps.length - 1) return
        const swapIdx = dir === 'up' ? idx - 1 : idx + 1
        const reordered = [...steps]
        ;[reordered[idx], reordered[swapIdx]] = [reordered[swapIdx], reordered[idx]]
        reorderMutation.mutate(reordered.map((s) => ({
            type: s.type,
            id: stepEntityId(s),
        })))
    }

    // ── Toggle enabled ──
    const toggleMutation = useMutation<void, Error, PipelineStep>({
        mutationFn: async (step) => {
            if (step.type === 'script' && step.script) {
                const b = step.script
                const payload: ScriptBindingInput = {
                    scriptId: b.scriptId,
                    outputKey: b.outputKey,
                    order: b.order,
                    enabled: !b.enabled,
                }
                if (scope === 'spec') await specScriptBindingsApi.update(scopeId, b.id, payload)
                else if (scope === 'operation') await scriptBindingsApi.update(scopeId, b.id, payload)
                else await responseScriptBindingsApi.update(operationId!, scopeId, b.id, payload)
            } else if (step.type === 'validation' && step.validation) {
                const r = step.validation
                await validationRulesApi.update(r.id, {
                    name: r.name,
                    order: r.order,
                    enabled: !r.enabled,
                    conditionTree: r.conditionTree,
                    onSuccess: r.onSuccess,
                    onFailure: r.onFailure,
                })
            } else if (step.type === 'collection' && step.collection) {
                const m = step.collection
                await collectionMappingsApi.update(m.id, {
                    collectionName: m.collectionName,
                    name: m.name,
                    operation: m.operation,
                    filterRules: m.filterRules,
                    dataRules: m.dataRules,
                    outputKey: m.outputKey,
                    order: m.order,
                    enabled: !m.enabled,
                })
            }
        },
        onSuccess: invalidate,
    })

    // ── Delete ──
    const deleteMutation = useMutation({
        mutationFn: (step: PipelineStep) => {
            if (step.type === 'script' && step.script) {
                const b = step.script
                if (scope === 'spec') return specScriptBindingsApi.delete(scopeId, b.id)
                if (scope === 'operation') return scriptBindingsApi.delete(scopeId, b.id)
                return responseScriptBindingsApi.delete(operationId!, scopeId, b.id)
            }
            if (step.type === 'validation' && step.validation) return validationRulesApi.delete(step.validation.id)
            if (step.type === 'collection' && step.collection) return collectionMappingsApi.delete(step.collection.id)
            return Promise.resolve()
        },
        onSuccess: invalidate,
    })

    // ── Add Script ──
    const addScriptMutation = useMutation({
        mutationFn: () => {
            if (scope === 'spec') return specScriptBindingsApi.create(scopeId, scriptForm)
            if (scope === 'operation') return scriptBindingsApi.create(scopeId, scriptForm)
            return responseScriptBindingsApi.create(operationId!, scopeId, scriptForm)
        },
        onSuccess: () => {
            invalidate()
            setAddModal(null)
            setScriptForm({ scriptId: '', outputKey: '', order: steps.length * 10, enabled: true })
            setScriptError(null)
        },
        onError: (e) => setScriptError((e as Error).message),
    })

    // ── Add Validation ──
    const addValMutation = useMutation({
        mutationFn: () => {
            const payload: ValidationInput = {
                name: valForm.name,
                order: valForm.order,
                enabled: valForm.enabled,
                conditionTree: valForm.conditionTree,
                onSuccess: valForm.onSuccess,
                onFailure: valForm.onFailure,
            }
            if (scope === 'spec') return validationRulesApi.createForSpec(scopeId, payload)
            return validationRulesApi.createForOperation(scopeId, payload)
        },
        onSuccess: () => {
            invalidate()
            setAddModal(null)
            setValForm({ ...EMPTY_VALIDATION_FORM, order: steps.length * 10 })
            setValError(null)
        },
        onError: (e) => setValError((e as Error).message),
    })

    // ── Edit Validation ──
    const editValMutation = useMutation<void, Error>({
        mutationFn: async () => {
            if (!editingStep?.validation) return
            await validationRulesApi.update(editingStep.validation.id, {
                name: valForm.name,
                order: valForm.order,
                enabled: valForm.enabled,
                conditionTree: valForm.conditionTree,
                onSuccess: valForm.onSuccess,
                onFailure: valForm.onFailure,
            })
        },
        onSuccess: () => {
            invalidate()
            setAddModal(null)
            setEditingStep(null)
            setValError(null)
        },
        onError: (e) => setValError((e as Error).message),
    })

    // ── Add Collection ──
    const addCollMutation = useMutation<void, Error>({
        mutationFn: async () => {
            if (scope === 'spec') await collectionMappingsApi.createForSpec(scopeId, collForm)
            else if (scope === 'operation') await collectionMappingsApi.createForOperation(scopeId, collForm)
            else await collectionMappingsApi.create(operationId!, scopeId, collForm)
        },
        onSuccess: () => {
            invalidate()
            setAddModal(null)
            setCollForm({ ...EMPTY_COLL_FORM, order: steps.length * 10 })
            setCollError(null)
        },
        onError: (e) => setCollError((e as Error).message),
    })

    // ── Edit Collection ──
    const editCollMutation = useMutation<void, Error>({
        mutationFn: async () => {
            if (!editingStep?.collection) return
            await collectionMappingsApi.update(editingStep.collection.id, collForm)
        },
        onSuccess: () => {
            invalidate()
            setAddModal(null)
            setEditingStep(null)
            setCollError(null)
        },
        onError: (e) => setCollError((e as Error).message),
    })

    // ── Modal openers ──
    const openAddScript = () => {
        setEditingStep(null)
        setScriptForm({ scriptId: allScripts?.[0]?.id ?? '', outputKey: '', order: steps.length * 10, enabled: true })
        setScriptError(null)
        setAddModal('script')
    }

    const openAddValidation = () => {
        setEditingStep(null)
        setValForm({ ...EMPTY_VALIDATION_FORM, order: steps.length * 10 })
        setValError(null)
        setAddModal('validation')
    }

    const openAddCollection = () => {
        setEditingStep(null)
        setCollForm({ ...EMPTY_COLL_FORM, order: steps.length * 10 })
        setCollError(null)
        setAddModal('collection')
    }

    const openEdit = (step: PipelineStep) => {
        setEditingStep(step)
        if (step.type === 'validation' && step.validation) {
            setValForm({
                name: step.validation.name,
                order: step.validation.order,
                enabled: step.validation.enabled,
                conditionTree: step.validation.conditionTree,
                onSuccess: step.validation.onSuccess ?? {},
                onFailure: step.validation.onFailure ?? {},
            })
            setValError(null)
            setAddModal('validation')
        } else if (step.type === 'collection' && step.collection) {
            const m = step.collection
            setCollForm({
                collectionName: m.collectionName,
                name: m.name ?? '',
                operation: m.operation,
                filterRules: m.filterRules ?? [],
                dataRules: m.dataRules ?? [],
                outputKey: m.outputKey,
                order: m.order,
                enabled: m.enabled,
            })
            setCollError(null)
            setAddModal('collection')
        }
    }

    const closeModal = () => {
        setAddModal(null)
        setEditingStep(null)
    }

    const isEditing = !!editingStep
    const showFilterRules = (op: CollectionOpType) => op !== 'insert'
    const showDataRules = (op: CollectionOpType) => op === 'insert' || op === 'update' || op === 'upsert'

    // ── Render ──
    const stepList = (
        <div className="divide-y divide-gray-100 dark:divide-slate-800">
            {steps.map((step, idx) => {
                if (scope === 'response') {
                    return (
                        <StepCard
                            key={stepId(step)}
                            step={step} index={idx} total={steps.length}
                            scope={scope}
                            onToggle={() => toggleMutation.mutate(step)}
                            onDelete={() => { if (confirm('Remove this step?')) deleteMutation.mutate(step) }}
                            onEdit={() => openEdit(step)}
                            onMove={(dir) => moveStep(step, dir)}
                        />
                    )
                }
                return (
                    <SortableStepCard
                        key={stepId(step)}
                        step={step} index={idx} total={steps.length}
                        scope={scope}
                        onToggle={() => toggleMutation.mutate(step)}
                        onDelete={() => { if (confirm('Remove this step?')) deleteMutation.mutate(step) }}
                        onEdit={() => openEdit(step)}
                    />
                )
            })}
        </div>
    )

    return (
        <>
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                {/* Header */}
                <div className="p-5 border-b border-gray-200 dark:border-slate-800 flex flex-wrap gap-3 items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-slate-100 dark:bg-slate-800 rounded-lg">
                            <Layers className="w-5 h-5 text-slate-600 dark:text-slate-400" />
                        </div>
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Processing Pipeline</h2>
                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                                Scripts, validations, and collection mappings run in the order shown.{' '}
                                {scope !== 'response' && 'Drag rows to reorder.'}
                                {' '}A failing validation aborts remaining steps at this scope.
                            </p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2 flex-wrap">
                        <button
                            onClick={openAddScript}
                            className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-indigo-700 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/30 transition-colors"
                        >
                            <Plus className="w-3.5 h-3.5" /><Code2 className="w-3.5 h-3.5" /> Script
                        </button>
                        {scope !== 'response' && (
                            <button
                                onClick={openAddValidation}
                                className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-amber-700 dark:text-amber-300 border border-amber-200 dark:border-amber-800 rounded-lg hover:bg-amber-50 dark:hover:bg-amber-900/30 transition-colors"
                            >
                                <Plus className="w-3.5 h-3.5" /><ShieldCheck className="w-3.5 h-3.5" /> Validation
                            </button>
                        )}
                        <button
                            onClick={openAddCollection}
                            className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium text-teal-700 dark:text-teal-300 border border-teal-200 dark:border-teal-800 rounded-lg hover:bg-teal-50 dark:hover:bg-teal-900/30 transition-colors"
                        >
                            <Plus className="w-3.5 h-3.5" /><Database className="w-3.5 h-3.5" /> Collection
                        </button>
                    </div>
                </div>

                {/* Step list */}
                {isLoading ? (
                    <div className="p-6">
                        <div className="animate-pulse space-y-3">
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                            <div className="h-12 bg-gray-200 dark:bg-slate-800 rounded-lg" />
                        </div>
                    </div>
                ) : steps.length === 0 ? (
                    <div className="p-10 text-center">
                        <Layers className="w-8 h-8 text-gray-300 dark:text-slate-700 mx-auto mb-3" />
                        <p className="text-sm text-gray-500 dark:text-slate-400 mb-1">No pipeline steps configured</p>
                        <p className="text-xs text-gray-400 dark:text-slate-500">
                            Add scripts, validations, or collection mappings using the buttons above.
                        </p>
                    </div>
                ) : scope === 'response' ? (
                    stepList
                ) : (
                    <DndContext
                        sensors={sensors}
                        collisionDetection={closestCenter}
                        onDragStart={handleDragStart}
                        onDragEnd={handleDragEnd}
                    >
                        <SortableContext items={steps.map(stepId)} strategy={verticalListSortingStrategy}>
                            {stepList}
                        </SortableContext>
                        <DragOverlay>
                            {activeStep && (
                                <StepCard
                                    step={activeStep}
                                    index={steps.findIndex((s) => stepId(s) === stepId(activeStep))}
                                    total={steps.length}
                                    scope={scope}
                                    isDragOverlay
                                    onToggle={() => {}}
                                    onDelete={() => {}}
                                    onEdit={() => {}}
                                />
                            )}
                        </DragOverlay>
                    </DndContext>
                )}
            </div>

            {/* ── Script modal ── */}
            {addModal === 'script' && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-md p-6">
                        <div className="flex items-center justify-between mb-5">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Attach Script</h3>
                            <button onClick={closeModal} className="p-1.5 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors">
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Script <span className="text-red-500">*</span></label>
                                {allScripts && allScripts.length > 0 ? (
                                    <select value={scriptForm.scriptId}
                                        onChange={e => setScriptForm(f => ({ ...f, scriptId: e.target.value }))}
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm">
                                        <option value="">Select a script…</option>
                                        {allScripts.map(s => (
                                            <option key={s.id} value={s.id} disabled={!s.enabled}>
                                                {s.name}{!s.enabled ? ' (disabled)' : ''}
                                            </option>
                                        ))}
                                    </select>
                                ) : (
                                    <p className="text-sm text-gray-500 dark:text-slate-400 p-3 border border-gray-200 dark:border-slate-700 rounded-lg">
                                        No scripts available. <Link to="/scripts/new" className="text-primary-600 hover:underline">Create one first</Link>.
                                    </p>
                                )}
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Output Key <span className="text-red-500">*</span></label>
                                <input type="text" value={scriptForm.outputKey}
                                    onChange={e => setScriptForm(f => ({ ...f, outputKey: e.target.value.replace(/[^a-zA-Z0-9_]/g, '') }))}
                                    placeholder="e.g. userData"
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Order</label>
                                <input type="number" value={scriptForm.order} min={0}
                                    onChange={e => setScriptForm(f => ({ ...f, order: parseInt(e.target.value) || 0 }))}
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                />
                            </div>
                            <div className="flex items-center gap-3">
                                <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                                <button type="button" onClick={() => setScriptForm(f => ({ ...f, enabled: !f.enabled }))}>
                                    {scriptForm.enabled ? <ToggleRight className="w-7 h-7 text-emerald-600" /> : <ToggleLeft className="w-7 h-7 text-gray-400 dark:text-slate-500" />}
                                </button>
                            </div>
                            {scriptError && <p className="text-sm text-red-600 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3">{scriptError}</p>}
                        </div>
                        <div className="flex gap-3 mt-6">
                            <button
                                onClick={() => addScriptMutation.mutate()}
                                disabled={addScriptMutation.isPending || !scriptForm.scriptId || !scriptForm.outputKey}
                                className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors">
                                {addScriptMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                                Attach
                            </button>
                            <button onClick={closeModal} className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors">Cancel</button>
                        </div>
                    </div>
                </div>
            )}

            {/* ── Validation modal ── */}
            {addModal === 'validation' && (
                <div className="fixed inset-0 bg-black/50 flex items-start justify-center z-50 p-4 overflow-y-auto">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-2xl p-6 my-8">
                        <div className="flex items-center justify-between mb-5">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100">
                                {isEditing ? 'Edit Validation Rule' : 'Add Validation Rule'}
                            </h3>
                            <button onClick={closeModal} className="p-1.5 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors">
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="space-y-4">
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Name <span className="text-red-500">*</span></label>
                                    <input type="text" value={valForm.name}
                                        onChange={e => setValForm(f => ({ ...f, name: e.target.value.replace(/[^a-zA-Z0-9_]/g, '') }))}
                                        placeholder="e.g. auth_valid"
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono"
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Order</label>
                                    <input type="number" value={valForm.order} min={0}
                                        onChange={e => setValForm(f => ({ ...f, order: parseInt(e.target.value) || 0 }))}
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                    />
                                </div>
                            </div>
                            <div className="flex items-center gap-3">
                                <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                                <button type="button" onClick={() => setValForm(f => ({ ...f, enabled: !f.enabled }))}>
                                    {valForm.enabled ? <ToggleRight className="w-7 h-7 text-emerald-600" /> : <ToggleLeft className="w-7 h-7 text-gray-400 dark:text-slate-500" />}
                                </button>
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">Condition</label>
                                <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-3">
                                    <ConditionEditor
                                        value={valForm.conditionTree}
                                        onChange={tree => setValForm(f => ({ ...f, conditionTree: tree }))}
                                        emptyHint="No conditions — this rule always passes."
                                        compact
                                    />
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-4">
                                <PropertyMapEditor label="On Success Properties" value={valForm.onSuccess}
                                    onChange={v => setValForm(f => ({ ...f, onSuccess: v }))} />
                                <PropertyMapEditor label="On Failure Properties" value={valForm.onFailure}
                                    onChange={v => setValForm(f => ({ ...f, onFailure: v }))} />
                            </div>
                            {valError && <p className="text-sm text-red-600 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3">{valError}</p>}
                        </div>
                        <div className="flex gap-3 mt-6">
                            <button
                                onClick={() => isEditing ? editValMutation.mutate() : addValMutation.mutate()}
                                disabled={(isEditing ? editValMutation.isPending : addValMutation.isPending) || !valForm.name}
                                className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors">
                                {(isEditing ? editValMutation.isPending : addValMutation.isPending) ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
                                {isEditing ? 'Save Changes' : 'Add Rule'}
                            </button>
                            <button onClick={closeModal} className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors">Cancel</button>
                        </div>
                    </div>
                </div>
            )}

            {/* ── Collection modal ── */}
            {addModal === 'collection' && (
                <div className="fixed inset-0 bg-black/50 flex items-start justify-center z-50 p-4 overflow-y-auto">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-2xl p-6 my-8">
                        <div className="flex items-center justify-between mb-5">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-slate-100">
                                {isEditing ? 'Edit Collection Mapping' : 'Add Collection Mapping'}
                            </h3>
                            <button onClick={closeModal} className="p-1.5 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors">
                                <X className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="space-y-4">
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Collection Name <span className="text-red-500">*</span></label>
                                    <input type="text" value={collForm.collectionName}
                                        onChange={e => setCollForm(f => ({ ...f, collectionName: e.target.value }))}
                                        placeholder="e.g. users"
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono"
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Display Name</label>
                                    <input type="text" value={collForm.name ?? ''}
                                        onChange={e => setCollForm(f => ({ ...f, name: e.target.value }))}
                                        placeholder="optional label"
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                    />
                                </div>
                            </div>
                            <div className="grid grid-cols-3 gap-4">
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Operation <span className="text-red-500">*</span></label>
                                    <select value={collForm.operation}
                                        onChange={e => setCollForm(f => ({ ...f, operation: e.target.value as CollectionOpType }))}
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm">
                                        {OP_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                                    </select>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Output Key <span className="text-red-500">*</span></label>
                                    <input type="text" value={collForm.outputKey}
                                        onChange={e => setCollForm(f => ({ ...f, outputKey: e.target.value.replace(/[^a-zA-Z0-9_]/g, '') }))}
                                        placeholder="e.g. result"
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm font-mono"
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Order</label>
                                    <input type="number" value={collForm.order} min={0}
                                        onChange={e => setCollForm(f => ({ ...f, order: parseInt(e.target.value) || 0 }))}
                                        className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                    />
                                </div>
                            </div>
                            <div className="flex items-center gap-3">
                                <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                                <button type="button" onClick={() => setCollForm(f => ({ ...f, enabled: !f.enabled }))}>
                                    {collForm.enabled ? <ToggleRight className="w-7 h-7 text-emerald-600" /> : <ToggleLeft className="w-7 h-7 text-gray-400 dark:text-slate-500" />}
                                </button>
                            </div>
                            {showFilterRules(collForm.operation) && (
                                <RuleEditor
                                    rules={collForm.filterRules}
                                    onChange={r => setCollForm(f => ({ ...f, filterRules: r }))}
                                    label="Filter Rules"
                                />
                            )}
                            {showDataRules(collForm.operation) && (
                                <RuleEditor
                                    rules={collForm.dataRules}
                                    onChange={r => setCollForm(f => ({ ...f, dataRules: r }))}
                                    label="Data Rules"
                                />
                            )}
                            {collError && <p className="text-sm text-red-600 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3">{collError}</p>}
                        </div>
                        <div className="flex gap-3 mt-6">
                            <button
                                onClick={() => isEditing ? editCollMutation.mutate() : addCollMutation.mutate()}
                                disabled={(isEditing ? editCollMutation.isPending : addCollMutation.isPending) || !collForm.collectionName || !collForm.outputKey}
                                className="flex-1 inline-flex items-center justify-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors">
                                {(isEditing ? editCollMutation.isPending : addCollMutation.isPending) ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
                                {isEditing ? 'Save Changes' : 'Add Mapping'}
                            </button>
                            <button onClick={closeModal} className="px-4 py-2 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors">Cancel</button>
                        </div>
                    </div>
                </div>
            )}
        </>
    )
}
