/**
 * Shared condition tree editor.
 *
 * Works with ConditionNode trees (AND / OR / NOT groups + leaf Conditions).
 * Used by: ResponseConfigEditor (response conditions), SpecDetail (proxy/AI conditions),
 * ValidationRulesPanel (validation condition trees).
 *
 * Conversion helpers are exported so callers that store flat Condition[] can
 * convert to/from the tree representation.
 */

import { useState } from 'react'
import { Plus, X, ChevronDown, ChevronRight } from 'lucide-react'
import clsx from 'clsx'
import type { Condition, ConditionNode, ConditionOperator } from '../../types'

// ---- Operator / source tables ------------------------------------------------

export const CONDITION_OPERATORS: { value: ConditionOperator; label: string; group?: string }[] = [
    { value: 'eq',           label: 'Equals' },
    { value: 'contains',     label: 'Contains' },
    { value: 'startsWith',   label: 'Starts With' },
    { value: 'endsWith',     label: 'Ends With' },
    { value: 'regex',        label: 'Regex' },
    { value: 'exists',       label: 'Exists' },
    { value: 'gt',           label: 'Greater Than' },
    { value: 'lt',           label: 'Less Than' },
    { value: 'gte',          label: '>=' },
    { value: 'lte',          label: '<=' },
    { value: 'dateEq',       label: 'Date Equals',     group: 'date' },
    { value: 'dateBefore',   label: 'Date Before',     group: 'date' },
    { value: 'dateAfter',    label: 'Date After',      group: 'date' },
    { value: 'dateLte',      label: 'Date ≤',          group: 'date' },
    { value: 'dateGte',      label: 'Date ≥',          group: 'date' },
    { value: 'dateInPast',   label: 'Date In Past',    group: 'date' },
    { value: 'dateInFuture', label: 'Date In Future',  group: 'date' },
    { value: 'dateToday',    label: 'Date Is Today',   group: 'date' },
    { value: 'dateBetween',  label: 'Date Between',    group: 'date' },
]

export const ALL_SOURCES = [
    'path', 'query', 'header', 'body', 'signature', 'script', 'validation', 'collection',
] as const
export type ConditionSource = (typeof ALL_SOURCES)[number]

// Sources shown in limited contexts (proxy / AI conditions on SpecDetail)
export const BASIC_SOURCES = ['path', 'query', 'header', 'body', 'script'] as const

const DATE_OPERATORS = new Set<ConditionOperator>([
    'dateEq', 'dateBefore', 'dateAfter', 'dateLte', 'dateGte',
    'dateInPast', 'dateInFuture', 'dateToday', 'dateBetween',
])
const DATE_NO_VALUE_OPERATORS = new Set<ConditionOperator>([
    'dateInPast', 'dateInFuture', 'dateToday',
])

const DATE_TOKENS = [
    { token: 'today',      desc: 'Current date at midnight' },
    { token: 'yesterday',  desc: 'Yesterday at midnight' },
    { token: 'tomorrow',   desc: 'Tomorrow at midnight' },
    { token: 'now',        desc: 'Current date+time' },
    { token: 'now+1d',     desc: '1 day from now' },
    { token: 'now-1d',     desc: '1 day ago' },
    { token: 'now+7d',     desc: '7 days from now' },
    { token: 'now-7d',     desc: '7 days ago' },
]

// ---- Helpers -----------------------------------------------------------------

export function emptyCondition(source: ConditionSource = 'header'): Condition {
    return { source, key: '', operator: 'eq', value: '', negate: false }
}

/** Convert a flat []Condition (all-AND) into a ConditionNode tree. */
export function conditionsToTree(conditions: Condition[]): ConditionNode | undefined {
    if (!conditions || conditions.length === 0) return undefined
    if (conditions.length === 1) return { condition: conditions[0] }
    return { operator: 'AND', children: conditions.map(c => ({ condition: c })) }
}

/** Convert a ConditionNode tree back into a flat []Condition if it is a simple AND-of-leaves tree. */
export function treeToConditions(tree: ConditionNode | undefined): Condition[] {
    if (!tree) return []
    if (tree.condition) return [tree.condition]
    if (tree.operator === 'AND' && tree.children) {
        const leaves = tree.children.flatMap(n => (n.condition ? [n.condition] : []))
        if (leaves.length === tree.children.length) return leaves
    }
    return [] // complex tree — cannot flatten
}

// ---- Leaf row ----------------------------------------------------------------

interface LeafRowProps {
    cond: Condition
    onChange: (updates: Partial<Condition>) => void
    onRemove?: () => void
    sources?: readonly ConditionSource[]
    depth?: number
}

export function ConditionLeafRow({ cond, onChange, onRemove, sources = ALL_SOURCES, depth = 0 }: LeafRowProps) {
    const showDatePicker = DATE_OPERATORS.has(cond.operator) && !DATE_NO_VALUE_OPERATORS.has(cond.operator)
    const valueDisabled = DATE_NO_VALUE_OPERATORS.has(cond.operator) || cond.operator === 'exists'
    const sourcePlaceholders: Partial<Record<ConditionSource, string>> = {
        signature:  'computed signature',
        script:     'outputKey.fieldName',
        validation: 'ruleName.status',
        collection: 'outputKey.fieldName',
    }
    const sourcePlaceholder = sourcePlaceholders[cond.source] ?? 'key'

    return (
        <div
            className={clsx(
                'flex items-center gap-1.5 flex-wrap',
                depth > 0 && 'ml-4 pl-3 border-l-2 border-indigo-200 dark:border-indigo-800/50'
            )}
        >
            {/* Source */}
            <select
                value={cond.source}
                onChange={e => onChange({ source: e.target.value as ConditionSource, key: '' })}
                className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
            >
                {sources.map(s => <option key={s} value={s}>{s}</option>)}
            </select>

            {/* Key */}
            <input
                type="text"
                value={cond.key}
                onChange={e => onChange({ key: e.target.value })}
                placeholder={sourcePlaceholder}
                disabled={cond.source === 'signature'}
                className="w-32 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60"
            />

            {/* Operator */}
            <select
                value={cond.operator}
                onChange={e => onChange({ operator: e.target.value as ConditionOperator })}
                className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
            >
                <optgroup label="String / Numeric">
                    {CONDITION_OPERATORS.filter(o => !o.group).map(o =>
                        <option key={o.value} value={o.value}>{o.label}</option>
                    )}
                </optgroup>
                <optgroup label="Date">
                    {CONDITION_OPERATORS.filter(o => o.group === 'date').map(o =>
                        <option key={o.value} value={o.value}>{o.label}</option>
                    )}
                </optgroup>
            </select>

            {/* NOT toggle */}
            <button
                type="button"
                title="Negate — invert the condition result"
                onClick={() => onChange({ negate: !cond.negate })}
                className={clsx(
                    'px-2 py-1.5 rounded border text-xs font-semibold transition-colors',
                    cond.negate
                        ? 'bg-red-100 dark:bg-red-900/40 border-red-400 dark:border-red-500 text-red-700 dark:text-red-300'
                        : 'border-gray-300 dark:border-slate-700 text-gray-400 dark:text-slate-500 hover:border-red-400 hover:text-red-500'
                )}
            >
                NOT
            </button>

            {/* Value — with date token autocomplete */}
            {showDatePicker && (
                <datalist id={`date-tokens-${depth}`}>
                    {DATE_TOKENS.map(t => <option key={t.token} value={t.token}>{t.desc}</option>)}
                </datalist>
            )}
            <input
                type="text"
                autoComplete="off"
                list={showDatePicker ? `date-tokens-${depth}` : undefined}
                value={cond.value}
                onChange={e => onChange({ value: e.target.value })}
                placeholder={
                    valueDisabled ? '—'
                    : cond.operator === 'dateBetween' ? 'from,to  e.g. today,now+7d'
                    : DATE_OPERATORS.has(cond.operator) ? 'e.g. today, now+7d, 2025-01-01'
                    : 'value'
                }
                disabled={valueDisabled}
                className="flex-1 min-w-[100px] px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60"
            />

            {onRemove && (
                <button type="button" onClick={onRemove} className="text-gray-400 hover:text-red-500 transition-colors">
                    <X className="w-4 h-4" />
                </button>
            )}
        </div>
    )
}

// ---- Recursive tree node ----------------------------------------------------

interface TreeNodeProps {
    node: ConditionNode
    onChange: (n: ConditionNode) => void
    onRemove?: () => void
    depth?: number
    sources?: readonly ConditionSource[]
}

function TreeNode({ node, onChange, onRemove, depth = 0, sources }: TreeNodeProps) {
    const [collapsed, setCollapsed] = useState(false)

    if (node.condition) {
        return (
            <ConditionLeafRow
                cond={node.condition}
                onChange={updates => onChange({ condition: { ...node.condition!, ...updates } })}
                onRemove={onRemove}
                sources={sources}
                depth={depth}
            />
        )
    }

    const op = (node.operator ?? 'AND') as 'AND' | 'OR' | 'NOT'
    const children = node.children ?? []

    const updateChild = (i: number, child: ConditionNode) => {
        const next = [...children]; next[i] = child
        onChange({ operator: op, children: next })
    }
    const removeChild = (i: number) => {
        const next = children.filter((_, idx) => idx !== i)
        if (next.length === 0) {
            // Replace group with empty leaf so the UI doesn't vanish
            onChange({ condition: emptyCondition() })
        } else if (next.length === 1 && next[0].condition && op !== 'NOT') {
            onChange(next[0])
        } else {
            onChange({ operator: op, children: next })
        }
    }
    const addChild = () => onChange({ operator: op, children: [...children, { condition: emptyCondition() }] })
    const cycleOp = () => {
        const seq: Array<'AND' | 'OR' | 'NOT'> = ['AND', 'OR', 'NOT']
        const next = seq[(seq.indexOf(op) + 1) % seq.length]
        onChange({ operator: next, children: op === 'NOT' ? children.slice(0, 1) : children })
    }

    const opColors = {
        AND: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 border-blue-300 dark:border-blue-700',
        OR:  'bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 border-orange-300 dark:border-orange-700',
        NOT: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 border-red-300 dark:border-red-700',
    }

    return (
        <div
            className={clsx(
                'rounded-lg border',
                depth === 0 ? 'border-gray-200 dark:border-slate-700' : 'border-gray-200 dark:border-slate-700',
                depth > 0 && 'ml-4'
            )}
        >
            {/* Group header */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-50 dark:bg-slate-800/50 rounded-t-lg border-b border-gray-200 dark:border-slate-700">
                <button type="button" onClick={() => setCollapsed(c => !c)} className="text-gray-400 hover:text-gray-600 dark:hover:text-slate-300">
                    {collapsed ? <ChevronRight className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                </button>
                <button
                    type="button"
                    onClick={cycleOp}
                    title="Click to change logical operator"
                    className={clsx('px-2 py-0.5 rounded border text-xs font-bold transition-colors', opColors[op])}
                >
                    {op}
                </button>
                <span className="text-xs text-gray-400 dark:text-slate-500">
                    {op === 'AND' ? 'All must match' : op === 'OR' ? 'Any must match' : 'Must NOT match'}
                </span>
                <div className="ml-auto flex items-center gap-1">
                    {op !== 'NOT' && (
                        <>
                            <button
                                type="button"
                                onClick={addChild}
                                className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-0.5"
                            >
                                <Plus className="w-3 h-3" /> condition
                            </button>
                            <span className="text-gray-300 dark:text-slate-600">|</span>
                            <button
                                type="button"
                                onClick={() => onChange({
                                    operator: op,
                                    children: [...children, { operator: 'AND', children: [{ condition: emptyCondition() }] }],
                                })}
                                className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-0.5"
                            >
                                <Plus className="w-3 h-3" /> group
                            </button>
                        </>
                    )}
                    {onRemove && (
                        <>
                            <span className="text-gray-300 dark:text-slate-600">|</span>
                            <button type="button" onClick={onRemove} className="text-gray-400 hover:text-red-500">
                                <X className="w-3.5 h-3.5" />
                            </button>
                        </>
                    )}
                </div>
            </div>

            {/* Children */}
            {!collapsed && (
                <div className="p-3 space-y-2">
                    {children.map((child, i) => (
                        <TreeNode
                            key={i}
                            node={child}
                            onChange={c => updateChild(i, c)}
                            onRemove={() => removeChild(i)}
                            depth={depth + 1}
                            sources={sources}
                        />
                    ))}
                    {children.length === 0 && (
                        <p className="text-xs text-gray-400 dark:text-slate-500 italic">Empty group</p>
                    )}
                </div>
            )}
        </div>
    )
}

// ---- Public component --------------------------------------------------------

export interface ConditionEditorProps {
    /** Current tree value. undefined = no conditions (always matches). */
    value: ConditionNode | undefined
    onChange: (v: ConditionNode | undefined) => void
    /** Which sources to expose. Defaults to ALL_SOURCES. */
    sources?: readonly ConditionSource[]
    /** Label shown above the editor */
    label?: string
    /** Hint shown when the tree is empty */
    emptyHint?: string
    /** Compact mode for narrow layouts */
    compact?: boolean
}

export default function ConditionEditor({
    value,
    onChange,
    sources = ALL_SOURCES,
    label,
    emptyHint = 'No conditions — this always matches.',
    compact = false,
}: ConditionEditorProps) {
    const addRoot = () => onChange({ condition: emptyCondition() })
    const addGroup = (op: 'AND' | 'OR') =>
        onChange({ operator: op, children: [{ condition: emptyCondition() }, { condition: emptyCondition() }] })

    return (
        <div>
            {label && (
                <div className="flex items-center justify-between mb-2">
                    <label className={clsx('font-medium text-gray-700 dark:text-slate-300', compact ? 'text-xs' : 'text-sm')}>
                        {label}
                    </label>
                </div>
            )}

            {value ? (
                <div className="space-y-2">
                    <TreeNode
                        node={value}
                        onChange={onChange}
                        onRemove={() => onChange(undefined)}
                        depth={0}
                        sources={sources}
                    />
                </div>
            ) : (
                <div className={clsx(
                    'rounded-lg border border-dashed border-gray-300 dark:border-slate-700',
                    compact ? 'px-3 py-2' : 'px-4 py-3'
                )}>
                    <p className={clsx('text-gray-400 dark:text-slate-500 mb-2', compact ? 'text-xs' : 'text-sm')}>
                        {emptyHint}
                    </p>
                    <div className="flex items-center gap-2">
                        <button
                            type="button"
                            onClick={addRoot}
                            className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
                        >
                            <Plus className="w-3 h-3" /> Add condition
                        </button>
                        <button
                            type="button"
                            onClick={() => addGroup('AND')}
                            className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
                        >
                            <Plus className="w-3 h-3" /> AND group
                        </button>
                        <button
                            type="button"
                            onClick={() => addGroup('OR')}
                            className="text-xs text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
                        >
                            <Plus className="w-3 h-3" /> OR group
                        </button>
                    </div>
                </div>
            )}
        </div>
    )
}
