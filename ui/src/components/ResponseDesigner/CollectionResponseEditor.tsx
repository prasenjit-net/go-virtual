import { MappingHintsProvider, MappingHintControls, SuggestedFieldInput, shapeHints } from '../shared/MappingHints'
import MappingDefaultInput, { isValidDefaultJSON } from '../shared/MappingDefaultInput'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
    AlertCircle,
    ArrowLeft,
    Database,
    FileJson,
    GitBranch,
    Layers,
    List,
    Loader2,
    Plus,
    Save,
    Settings,
    Trash2,
} from 'lucide-react'
import clsx from 'clsx'
import { operationsApi, responsesApi, tagsApi } from '../../services/api'
import type {
    CollectionFilter,
    CollectionResponseConfig,
    FieldOverride,
    NamedQuery,
    CollectionOpType,
    PrimaryResponseMapper,
    ResponseConfig,
    ResponseConfigInput,
    RootKind,
    SpecExample,
    ValueBinding,
    ValueSource,
} from '../../types'
import ConditionEditor, { conditionsToTree } from '../shared/ConditionEditor'

interface CollectionResponseEditorProps {
    operationId: string
    config: ResponseConfig | null
    onClose: () => void
}

// ── Binding row editing state ───────────────────────────────────────────────
// A ValueBinding is edited as one row: a source picker plus either a "key"
// text field (path/query/header/body/document/primary/mapper) or a raw JSON
// text field (literal).

interface BindingRowState {
    targetPath: string
    source: ValueSource | ''
    key: string
    literal: string
    defaultText?: string
    skipWhenMissing?: boolean
}

function literalToText(v: unknown): string {
    if (v === undefined) return ''
    try {
        return JSON.stringify(v)
    } catch {
        return String(v)
    }
}

function parseLiteralInput(text: string): unknown {
    const trimmed = text.trim()
    if (trimmed === '') return null
    try {
        return JSON.parse(trimmed)
    } catch {
        return trimmed
    }
}

function emptyRow(source: ValueSource = 'path'): BindingRowState {
    return { targetPath: '', source, key: '', literal: '' }
}

function rowFromBinding(targetPath: string, value: ValueBinding): BindingRowState {
    return {
        targetPath,
        source: value.source,
        skipWhenMissing: value.skipWhenMissing,
        defaultText: Object.prototype.hasOwnProperty.call(value, 'defaultValue') ? literalToText(value.defaultValue) : undefined,
        key: value.source === 'literal' ? '' : (value.key || ''),
        literal: value.source === 'literal' ? literalToText(value.value) : '',
    }
}

function rowsFromFilters(filters?: CollectionFilter[]): BindingRowState[] {
    return (filters || []).map((f) => rowFromBinding(f.targetPath, f.value))
}

function rowsFromOverrides(overrides?: FieldOverride[]): BindingRowState[] {
    return (overrides || []).map((o) => rowFromBinding(o.targetPath, o.value))
}

function rowToBinding(row: BindingRowState): ValueBinding {
    if (row.source === 'literal') {
        return { source: 'literal', value: parseLiteralInput(row.literal) }
    }
    return { skipWhenMissing: row.skipWhenMissing || undefined, source: row.source || 'path', key: row.key.trim(), ...(row.defaultText !== undefined ? { defaultValue: JSON.parse(row.defaultText) } : {}) }
}

interface MapperRowState {
    outputKey: string
    mode: CollectionOpType
    collectionName: string
    filters: BindingRowState[]
    data: BindingRowState[]
}

function emptyMapper(): MapperRowState {
    return { outputKey: '', mode: 'find-one', collectionName: '', filters: [], data: [] }
}

function mappersFromConfig(mappers?: NamedQuery[]): MapperRowState[] {
    return (mappers || []).map((m) => ({
        outputKey: m.outputKey,
        mode: m.mode,
        collectionName: m.collectionName,
        filters: rowsFromFilters(m.filterRules),
        data: rowsFromFilters(m.dataRules),
    }))
}

const FILTER_SOURCES: { value: ValueSource; label: string }[] = [
    { value: 'path', label: 'Path' },
    { value: 'query', label: 'Query' },
    { value: 'header', label: 'Header' },
    { value: 'body', label: 'Body (GJSON)' },
    { value: 'literal', label: 'Literal (JSON)' },
]
const MAPPER_FILTER_SOURCES: { value: ValueSource; label: string }[] = [
    ...FILTER_SOURCES,
    { value: 'primary', label: 'Primary document' },
]
const OVERRIDE_SOURCES: { value: ValueSource; label: string }[] = [
    { value: 'document', label: 'Document field' },
    { value: 'mapper', label: 'Mapper output' },
    ...FILTER_SOURCES,
]

const inputClass =
    'w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed text-sm'
const labelClass = 'block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1'

// ── Binding row component ───────────────────────────────────────────────────

function BindingRowContent({
    row,
    sources,
    keyPlaceholder,
    targetPathLabel,
    targetPathPlaceholder,
    topLevel,
    onChange,
    onRemove,
}: {
    row: BindingRowState
    collectionName?: string
    topLevel?: boolean
    sources: { value: ValueSource; label: string }[]
    keyPlaceholder: (source: ValueSource | '') => string
    targetPathLabel: string
    targetPathPlaceholder: string
    onChange: (next: BindingRowState) => void
    onRemove: () => void
}) {
    return (
        <div className="flex flex-wrap items-start gap-2 bg-gray-50 dark:bg-slate-900 rounded-lg p-2.5">
            <div className="w-40 flex-shrink-0">
                <SuggestedFieldInput hintSource={targetPathLabel === 'Collection field' ? 'document' : 'target'} topLevel={topLevel || targetPathLabel === 'Collection field'}
                    value={row.targetPath}
                    onChange={(e) => onChange({ ...row, targetPath: e.target.value })}
                    placeholder={targetPathPlaceholder}
                    title={targetPathLabel}
                    className={inputClass}
                />
            </div>
            <span className="mt-2 text-gray-400 dark:text-slate-500 text-sm">←</span>
            <div className="w-36 flex-shrink-0">
                <select
                    value={row.source}
                    onChange={(e) => onChange({ ...row, source: e.target.value as ValueSource, defaultText: e.target.value === 'literal' ? undefined : row.defaultText, skipWhenMissing: e.target.value === 'literal' ? undefined : row.skipWhenMissing })}
                    className={inputClass}
                >
                    {sources.map((s) => (
                        <option key={s.value} value={s.value}>{s.label}</option>
                    ))}
                </select>
            </div>
            <div className="min-w-[10rem] flex-1">
                {row.source === 'literal' ? (
                    <SuggestedFieldInput hintSource={targetPathLabel === 'Collection field' ? 'document' : 'target'} valueKey={row.targetPath} jsonValue
                        value={row.literal}
                        onChange={(e) => onChange({ ...row, literal: e.target.value })}
                        placeholder='true, 42, "text", null'
                        className={`${inputClass} font-mono`}
                    />
                ) : (
                    <SuggestedFieldInput hintSource={row.source}
                        value={row.key}
                        onChange={(e) => onChange({ ...row, key: e.target.value })}
                        placeholder={keyPlaceholder(row.source)}
                        className={`${inputClass} font-mono`}
                    />
                )}
            </div>
            <button
                type="button"
                onClick={onRemove}
                className="mt-1.5 p-1 text-gray-400 dark:text-slate-500 hover:text-red-600 flex-shrink-0"
                title="Remove"
            >
                <Trash2 className="w-4 h-4" />
            </button>
            {row.source !== 'literal' && <MappingDefaultInput hintSource={row.source} valueKey={row.key} json value={row.defaultText} skipWhenMissing={row.skipWhenMissing} onChange={(defaultText, skipWhenMissing) => onChange({ ...row, defaultText, skipWhenMissing })} />}
        </div>
    )
}

function BindingRow(props: Parameters<typeof BindingRowContent>[0]) {
    return <MappingHintsProvider collectionName={props.collectionName}><BindingRowContent {...props} /></MappingHintsProvider>
}

function filterKeyPlaceholder(source: ValueSource | ''): string {
    switch (source) {
        case 'path': return 'id'
        case 'query': return 'status'
        case 'header': return 'X-Tenant-Id'
        case 'body': return 'customer.email'
        case 'primary': return 'planId'
        case 'document': return 'profile.name'
        case 'mapper': return 'addresses or plan.label'
        default: return 'key'
    }
}

const writesData = (mode: CollectionOpType) => ['insert', 'update', 'upsert'].includes(mode)

function DataRulesEditor({ rows, onChange, allowPrimary, collectionName }: {
    collectionName?: string
    rows: BindingRowState[]
    onChange: (rows: BindingRowState[]) => void
    allowPrimary: boolean
}) {
    return <div className="space-y-2">
        <div className="flex items-center justify-between">
            <span className={labelClass}>Data fields</span>
            <button type="button" onClick={() => onChange([...rows, emptyRow('body')])} className="text-xs text-primary-700 dark:text-primary-300 hover:underline">+ Add field</button>
        </div>
        {rows.length === 0 && <p className="text-xs text-gray-500 dark:text-slate-400">Add the fields this operation writes.</p>}
        {rows.map((row, i) => <BindingRow key={i} row={row} collectionName={collectionName} topLevel
            sources={allowPrimary ? MAPPER_FILTER_SOURCES : FILTER_SOURCES}
            keyPlaceholder={filterKeyPlaceholder} targetPathLabel="Collection field" targetPathPlaceholder="status"
            onChange={(next) => onChange(rows.map((r, j) => j === i ? next : r))}
            onRemove={() => onChange(rows.filter((_, j) => j !== i))} />)}
    </div>
}

type EditorTab = 'metadata' | 'conditions' | 'query' | 'mappers' | 'headers' | 'output'

export default function CollectionResponseEditor({ operationId, config, onClose }: CollectionResponseEditorProps) {
    const cr = config?.collectionResponse

    const [activeTab, setActiveTab] = useState<EditorTab>('metadata')

    const [name, setName] = useState(config?.name || '')
    const [description, setDescription] = useState(config?.description || '')
    const [priority, setPriority] = useState(config?.priority || 0)
    const [delay, setDelay] = useState(config?.delay || 0)
    const [enabled, setEnabled] = useState(config?.enabled ?? true)
    const [tag, setTag] = useState(config?.tag || 'default')
    const [conditionTree, setConditionTree] = useState(() =>
        config?.conditionTree ?? conditionsToTree(config?.conditions ?? [])
    )
    const [headers, setHeaders] = useState<Record<string, string>>(config?.headers || {})
    const [headerKey, setHeaderKey] = useState('')
    const [headerValue, setHeaderValue] = useState('')
    const [matchOnEmpty, setMatchOnEmpty] = useState(cr?.matchOnEmpty ?? false)
    const [fallbackToExample, setFallbackToExample] = useState(cr?.fallbackToExample ?? true)

    const [primaryMode, setPrimaryMode] = useState<PrimaryResponseMapper['mode']>(cr?.primary.mode)
    const primaryWrites = !!primaryMode && writesData(primaryMode)
    const conditionOnly = primaryMode === 'insert' || primaryMode === 'upsert'
    const [primaryData, setPrimaryData] = useState<BindingRowState[]>(() => rowsFromFilters(cr?.primary.dataRules))
    const [primaryCollectionName, setPrimaryCollectionName] = useState(cr?.primary.collectionName || '')
    const [primaryFilters, setPrimaryFilters] = useState<BindingRowState[]>(() => rowsFromFilters(cr?.primary.filterRules))
    // null preserves the saved/default choice until examples have loaded.
    const [exampleSelection, setExampleSelection] = useState<string | null>(config ? null : '')
    const [manualRootKind, setManualRootKind] = useState<RootKind>(cr?.rootKind || 'object')
    const [mappers, setMappers] = useState<MapperRowState[]>(() => mappersFromConfig(cr?.additionalMappers))
    const [overrides, setOverrides] = useState<BindingRowState[]>(() => rowsFromOverrides(cr?.overrides))

    const [error, setError] = useState('')

    const queryClient = useQueryClient()

    const { data: tags } = useQuery({ queryKey: ['tags'], queryFn: tagsApi.list })
    const tagOptions = (tags && tags.length > 0) ? tags : [{ name: 'default' }]

    const { data: specExamples, isPending: examplesLoading, isError: examplesError } = useQuery<SpecExample[]>({
        queryKey: ['specExamples', operationId],
        queryFn: () => operationsApi.getSpecExamples(operationId),
        enabled: !!operationId,
    })

    const exampleKey = (example: SpecExample) => JSON.stringify([example.statusCode, example.exampleName || ''])
    const chosenExample = exampleSelection === null
        ? specExamples?.find((example) => example.statusCode === config?.statusCode
            && (!cr?.templateRef || example.exampleName === cr.templateRef))
        : specExamples?.find((example) => exampleKey(example) === exampleSelection)
    const selectionUnavailable = !examplesLoading && !examplesError && !chosenExample
        && (exampleSelection === null || exampleSelection !== '')
    const templateValue = useMemo(() => {
        if (!chosenExample?.bodyExample) return undefined
        try {
            return JSON.parse(chosenExample.bodyExample)
        } catch {
            return undefined
        }
    }, [chosenExample])
    const isIdentityMode = !chosenExample || templateValue === undefined
    const derivedRootKind: RootKind = isIdentityMode ? manualRootKind : (Array.isArray(templateValue) ? 'array' : 'object')

    const buildCollectionResponse = (): CollectionResponseConfig => ({
        primary: {
            mode: primaryMode,
            dataRules: primaryWrites ? primaryData.filter((r) => r.targetPath.trim()).map((r) => ({ targetPath: r.targetPath.trim(), value: rowToBinding(r) })) : undefined,
            collectionName: primaryCollectionName.trim(),
            filterRules: (primaryMode === 'insert' ? [] : primaryFilters)
                .filter((r) => r.targetPath.trim())
                .map((r) => ({ targetPath: r.targetPath.trim(), value: rowToBinding(r) })),
        },
        additionalMappers: mappers
            .map((m) => ({
                outputKey: m.outputKey.trim(),
                mode: m.mode,
                dataRules: writesData(m.mode) ? m.data.filter((r) => r.targetPath.trim()).map((r) => ({ targetPath: r.targetPath.trim(), value: rowToBinding(r) })) : undefined,
                collectionName: m.collectionName.trim(),
                filterRules: m.filters
                    .filter((r) => r.targetPath.trim())
                    .map((r) => ({ targetPath: r.targetPath.trim(), value: rowToBinding(r) })),
            })),
        overrides: overrides
            .filter((r) => r.targetPath.trim())
            .map((r) => ({ targetPath: r.targetPath.trim(), value: rowToBinding(r) })),
        templateRef: chosenExample?.exampleName || undefined,
        rootKind: isIdentityMode ? manualRootKind : undefined,
        matchOnEmpty,
        fallbackToExample,
    })

    const createMutation = useMutation({
        mutationFn: (data: ResponseConfigInput) => responsesApi.create(operationId, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
        onError: (err: Error) => setError(err.message),
    })
    const updateMutation = useMutation({
        mutationFn: (data: Partial<ResponseConfigInput>) => responsesApi.update(config!.id, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
        onError: (err: Error) => setError(err.message),
    })

    const handleSave = () => {
        setError('')
        if (!name.trim()) {
            setError('Name is required')
            setActiveTab('metadata')
            return
        }
        if (examplesLoading || examplesError || !chosenExample) {
            setError('Select a response example in Metadata before saving.')
            setActiveTab('metadata')
            return
        }
        const statusCode = chosenExample.statusCode
        if (!primaryCollectionName.trim()) {
            setError('A primary collection name is required')
            setActiveTab('query')
            return
        }

        const bindingGroups: [EditorTab, BindingRowState[]][] = [
            ['query', [...(primaryMode === 'insert' ? [] : primaryFilters), ...(primaryWrites ? primaryData : [])]],
            ['mappers', mappers.flatMap((mapper) => [...mapper.filters, ...mapper.data])],
            ['output', overrides],
        ]
        for (const [tab, rows] of bindingGroups) {
            if (rows.some((row) => row.source !== 'literal' && !isValidDefaultJSON(row.defaultText))) {
                setError('Default values must be valid JSON. Put text values in double quotes.')
                setActiveTab(tab)
                return
            }
        }
        const collectionResponse = buildCollectionResponse()
        if ((primaryWrites || primaryMode === 'find-one') && derivedRootKind !== 'object'
            || primaryMode === 'find-many' && derivedRootKind !== 'array') {
            setError('The main operation must match the response root shape. Insert, Update, and Upsert require an object.')
            setActiveTab('query')
            return
        }
        if (primaryWrites && (!collectionResponse.primary.dataRules?.length || primaryMode !== 'insert' && !collectionResponse.primary.filterRules?.length)) {
            setError('Insert requires data fields. Update and Upsert require query filters and data fields.')
            setActiveTab('query')
            return
        }
        const outputKeys = new Set<string>()
        for (const mapper of collectionResponse.additionalMappers || []) {
            if (!mapper.outputKey || !mapper.collectionName || outputKeys.has(mapper.outputKey)) {
                setError('Each additional mapper needs a collection and a unique output key.')
                setActiveTab('mappers')
                return
            }
            outputKeys.add(mapper.outputKey)
            if (writesData(mapper.mode) && !mapper.dataRules?.length
                || ['update', 'upsert', 'delete'].includes(mapper.mode) && !mapper.filterRules?.length) {
                setError(`Mapper ${mapper.outputKey}: ${mapper.mode} requires ${writesData(mapper.mode) ? 'data fields' : 'filters'}${['update', 'upsert'].includes(mapper.mode) ? ' and filters' : ''}.`)
                setActiveTab('mappers')
                return
            }
        }

        if (config) {
            updateMutation.mutate({
                collectionResponse,
                name: name.trim(),
                description: description.trim(),
                tag,
                statusCode,
                priority,
                delay,
                enabled,
                conditionTree,
                headers,
            })
        } else {
            const data: ResponseConfigInput = {
                name: name.trim(),
                description: description.trim(),
                tag,
                statusCode,
                priority,
                delay,
                enabled,
                conditions: [],
                conditionTree,
                headers,
                body: '',
                kind: 'collection',
                collectionResponse,
            }
            createMutation.mutate(data)
        }
    }

    const addHeader = () => {
        if (headerKey.trim()) {
            setHeaders({ ...headers, [headerKey.trim()]: headerValue })
            setHeaderKey('')
            setHeaderValue('')
        }
    }
    const removeHeader = (key: string) => {
        const next = { ...headers }
        delete next[key]
        setHeaders(next)
    }

    const isSaving = createMutation.isPending || updateMutation.isPending

    const tabs: { id: EditorTab; label: string; icon: typeof Settings }[] = [
        { id: 'metadata', label: 'Metadata', icon: Settings },
        { id: 'conditions', label: 'Conditions', icon: GitBranch },
        { id: 'query', label: 'Query', icon: Database },
        { id: 'mappers', label: 'Additional Mappers', icon: Layers },
        { id: 'headers', label: 'Headers', icon: List },
        { id: 'output', label: 'Output', icon: FileJson },
    ]

    return (
        <MappingHintsProvider operationId={operationId} scope="response" collectionName={primaryCollectionName} statusCode={chosenExample?.statusCode} templateRef={chosenExample?.exampleName} responseId={config?.id}
            extra={shapeHints(templateValue, 'target')}
            collections={[
                { name: primaryCollectionName, source: 'primary' },
                ...mappers.map(m => ({ name: m.collectionName, source: 'mapper', prefix: m.outputKey, array: m.mode === 'find-many' })),
            ]}>
        <div className="flex flex-col h-full overflow-hidden bg-gray-50 dark:bg-slate-950">
            {/* ── Top bar ───────────────────────────────────────────────────── */}
            <div className="bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-800 flex items-center gap-2 px-3 h-12 flex-shrink-0">
                <button
                    onClick={onClose}
                    className="p-1.5 rounded-md text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                    title="Back"
                >
                    <ArrowLeft className="w-4 h-4" />
                </button>
                <div className="w-px h-5 bg-gray-200 dark:bg-slate-700" />
                <Database className="w-4 h-4 text-teal-600 dark:text-teal-400 flex-shrink-0" />
                <span className="text-sm font-semibold text-gray-800 dark:text-slate-200 truncate max-w-xs">
                    {name || (config ? 'Collection Response' : 'New Collection Response')}
                </span>
                <span className="px-2 py-0.5 text-xs bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300 rounded-full flex-shrink-0">
                    Collection
                </span>

                <div className="ml-auto flex items-center gap-2">
                    <button
                        onClick={handleSave}
                        disabled={isSaving}
                        title="Save"
                        className="px-3 py-1 text-xs bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50 inline-flex items-center gap-1.5"
                    >
                        {isSaving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
                        {isSaving ? 'Saving…' : 'Save'}
                    </button>
                </div>
            </div>

            {error && (
                <div className="flex-shrink-0 px-4 py-1.5 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/40 text-xs text-red-700 dark:text-red-300 flex items-center gap-2">
                    <AlertCircle className="w-3.5 h-3.5 flex-shrink-0" />
                    {error}
                </div>
            )}

            {/* ── Tabs ──────────────────────────────────────────────────────── */}
            <div className="flex flex-1 min-h-0 overflow-hidden">
                <div className="flex flex-1 flex-col min-w-0 min-h-0 overflow-hidden">
                    <div className="flex-shrink-0 flex items-center border-b border-gray-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-1 overflow-x-auto">
                        {tabs.map(({ id, label, icon: Icon }) => (
                            <button
                                key={id}
                                onClick={() => setActiveTab(id)}
                                className={clsx(
                                    'flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap',
                                    activeTab === id
                                        ? 'border-primary-600 text-primary-600'
                                        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300',
                                )}
                            >
                                <Icon className="w-3 h-3" /> {label}
                            </button>
                        ))}
                    </div>

                    <div className="flex-1 min-h-0 overflow-y-auto">
                        {activeTab === 'metadata' && (
                            <div className="p-4 space-y-4 max-w-2xl">
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className={labelClass}>Name *</label>
                                        <input value={name} onChange={(e) => setName(e.target.value)} className={inputClass} placeholder="Users by id" />
                                    </div>
                                    <div>
                                        <label className={labelClass}>Description</label>
                                        <input value={description} onChange={(e) => setDescription(e.target.value)} className={inputClass} placeholder="Returns a user document from the users collection" />
                                    </div>
                                </div>

                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className={labelClass}>Tag</label>
                                        <select value={tag} onChange={(e) => setTag(e.target.value)} className={inputClass}>
                                            {tagOptions.map((t: { name: string }) => (
                                                <option key={t.name} value={t.name}>{t.name}</option>
                                            ))}
                                        </select>
                                    </div>
                                    <div>
                                        <label htmlFor="collection-response-example" className={labelClass}>Response example *</label>
                                        <select
                                            id="collection-response-example"
                                            required
                                            value={chosenExample ? exampleKey(chosenExample) : ''}
                                            onChange={(e) => setExampleSelection(e.target.value)}
                                            disabled={examplesLoading || examplesError || !specExamples?.length}
                                            aria-describedby="collection-response-example-help"
                                            className={inputClass}
                                        >
                                            <option value="" disabled>
                                                {examplesLoading ? 'Loading examples…' : selectionUnavailable ? 'Saved example unavailable — choose an example' : 'Select a response example'}
                                            </option>
                                            {(specExamples || []).map((example) => (
                                                <option key={exampleKey(example)} value={exampleKey(example)}>
                                                    {example.statusCode} — {example.exampleName || 'Default example'}{example.exampleSummary ? ` — ${example.exampleSummary}` : ''}
                                                </option>
                                            ))}
                                        </select>
                                        <p id="collection-response-example-help" className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                                            {examplesError ? 'Could not load response examples. Reopen the editor to retry.'
                                                : !examplesLoading && !specExamples?.length ? 'No response examples are available. Add a response example to the operation’s spec before saving.'
                                                : 'Required. Choose from all response examples; the selected example sets the status code and output template.'}
                                        </p>
                                    </div>
                                </div>

                                <div className="grid grid-cols-3 gap-4">
                                    <div>
                                        <label className={labelClass}>Priority</label>
                                        <input type="number" value={priority} onChange={(e) => setPriority(parseInt(e.target.value) || 0)} className={inputClass} min={0} />
                                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">Lower = higher priority</p>
                                    </div>
                                    <div>
                                        <label className={labelClass}>Delay (ms)</label>
                                        <input type="number" value={delay} onChange={(e) => setDelay(parseInt(e.target.value) || 0)} className={inputClass} min={0} />
                                    </div>
                                    <div>
                                        <label className={labelClass}>Enabled</label>
                                        <button
                                            type="button"
                                            onClick={() => setEnabled(!enabled)}
                                            className={clsx(
                                                'w-full px-3 py-2 rounded-lg border text-sm',
                                                enabled
                                                    ? 'bg-green-50 border-green-300 text-green-700 dark:bg-green-950/40 dark:border-green-900/50 dark:text-green-300'
                                                    : 'bg-gray-50 border-gray-300 text-gray-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-400',
                                            )}
                                        >
                                            {enabled ? 'Yes' : 'No'}
                                        </button>
                                    </div>
                                </div>
                            </div>
                        )}

                        {activeTab === 'conditions' && (
                            <div className="p-4 space-y-4 max-w-2xl">
                                <MappingHintControls /><ConditionEditor
                                    label="Conditions"
                                    value={conditionTree}
                                    onChange={setConditionTree}
                                    emptyHint={conditionOnly ? "No conditions — this response is eligible for any request." : "No explicit conditions — this response can match any request whose primary query returns data."}
                                />

                                {conditionOnly ? <p className="text-sm text-gray-600 dark:text-slate-300">Only response conditions determine selection. The main operation runs after this response is selected; Upsert filters do not affect selection.</p> : <div className="flex items-center justify-between rounded-lg border border-teal-200 dark:border-teal-900/50 bg-teal-50 dark:bg-teal-950/20 px-3 py-2.5">
                                    <div>
                                        <div className="text-sm font-medium text-teal-800 dark:text-teal-300">Primary query returns data</div>
                                        <div className="text-xs text-teal-700 dark:text-teal-400">Always required to match, in addition to any conditions above.</div>
                                    </div>
                                    <label className="flex items-center gap-2 text-xs text-teal-800 dark:text-teal-300 flex-shrink-0 ml-3">
                                        <input type="checkbox" checked={matchOnEmpty} onChange={(e) => setMatchOnEmpty(e.target.checked)} />
                                        Match even when empty (render null / [])
                                    </label>
                                </div>}
                            </div>
                        )}

                        {activeTab === 'query' && (
                            <div className="p-4 space-y-4">
                                <div className="grid grid-cols-2 gap-4 max-w-2xl">
                                    <div>
                                        <label className={labelClass}>Collection *</label>
                                        <input value={primaryCollectionName} onChange={(e) => setPrimaryCollectionName(e.target.value)} className={`${inputClass} font-mono`} placeholder="users" />
                                    </div>
                                    <div>
                                        <label className={labelClass}>Main operation</label>
                                        <select className={inputClass} value={primaryMode || 'auto'} onChange={(e) => {
                                            const mode = e.target.value === 'auto' ? undefined : e.target.value as PrimaryResponseMapper['mode']
                                            setPrimaryMode(mode)
                                            if (!mode || !writesData(mode)) setPrimaryData([])
                                            if (isIdentityMode && mode) setManualRootKind(mode === 'find-many' ? 'array' : 'object')
                                        }}>
                                            <option value="auto">Automatic ({derivedRootKind === 'array' ? 'Find Many' : 'Find One'})</option>
                                            <option value="find-one" disabled={!isIdentityMode && derivedRootKind !== 'object'}>Find One</option>
                                            <option value="find-many" disabled={!isIdentityMode && derivedRootKind !== 'array'}>Find Many</option>
                                            <option value="update" disabled={!isIdentityMode && derivedRootKind !== 'object'}>Update</option>
                                            <option value="insert" disabled={!isIdentityMode && derivedRootKind !== 'object'}>Insert</option>
                                            <option value="upsert" disabled={!isIdentityMode && derivedRootKind !== 'object'}>Upsert</option>
                                        </select>
                                        <p className="text-xs text-gray-500 dark:text-slate-400 mt-1">
                                            {conditionOnly ? 'Only conditions select this response. After selection, Insert creates a document; Upsert uses its filters to update or insert.' : primaryMode === 'update' ? 'Query filters select this response. Then the first matched document is updated and rendered.' : 'The query result determines whether this response is selected.'}
                                            {derivedRootKind === 'array' && ' Insert, Update, and Upsert require an object response root.'}
                                        </p>
                                    </div>
                                </div>

                                {isIdentityMode && (
                                    <div className="max-w-2xl">
                                        <label className={labelClass}>Root shape</label>
                                        <div className="flex gap-2">
                                            {(['object', 'array'] as RootKind[]).map((k) => (
                                                <button
                                                    key={k}
                                                    type="button"
                                                    onClick={() => { setManualRootKind(k); if (primaryMode && k !== manualRootKind) { setPrimaryMode(k === 'array' ? 'find-many' : 'find-one'); setPrimaryData([]) } }}
                                                    className={clsx(
                                                        'px-3 py-1.5 rounded-lg border text-sm',
                                                        manualRootKind === k
                                                            ? 'bg-primary-50 border-primary-400 text-primary-700 dark:bg-primary-900/40 dark:border-primary-700 dark:text-primary-300'
                                                            : 'bg-white border-gray-300 text-gray-600 dark:bg-slate-950 dark:border-slate-700 dark:text-slate-300',
                                                    )}
                                                >
                                                    {k === 'object' ? 'Object (Find One)' : 'Array (Find Many)'}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                )}

                                {primaryWrites && <DataRulesEditor rows={primaryData} onChange={setPrimaryData} allowPrimary={primaryMode === 'update'} />}
                                {primaryMode !== 'insert' && <div>
                                    <div className="flex items-center justify-between mb-2">
                                        <label className={labelClass}>Query Filters</label>
                                        <button type="button" onClick={() => setPrimaryFilters([...primaryFilters, emptyRow()])} className="inline-flex items-center gap-1 text-xs font-medium text-primary-700 dark:text-primary-300 hover:underline">
                                            <Plus className="w-3.5 h-3.5" /> Add filter
                                        </button>
                                    </div>
                                    {primaryFilters.length === 0 && (
                                        <p className="text-xs text-gray-400 dark:text-slate-500">No filters — matches the first document (object root) or all documents (array root).</p>
                                    )}
                                    <div className="space-y-2">
                                        {primaryFilters.map((row, i) => (
                                            <BindingRow
                                                key={i}
                                                row={row}
                                                sources={FILTER_SOURCES}
                                                keyPlaceholder={filterKeyPlaceholder}
                                                targetPathLabel="Collection field"
                                                targetPathPlaceholder="_id"
                                                onChange={(next) => setPrimaryFilters(primaryFilters.map((r, j) => (j === i ? next : r)))}
                                                onRemove={() => setPrimaryFilters(primaryFilters.filter((_, j) => j !== i))}
                                            />
                                        ))}
                                    </div>
                                </div>}
                            </div>
                        )}

                        {activeTab === 'mappers' && (
                            <div className="p-4 space-y-4">
                                <div className="flex items-start gap-2 rounded-lg border border-indigo-200 dark:border-indigo-900/50 bg-indigo-50 dark:bg-indigo-950/20 px-3 py-2.5">
                                    <Layers className="w-4 h-4 text-indigo-600 dark:text-indigo-400 flex-shrink-0 mt-0.5" />
                                    <div>
                                        <div className="text-sm font-medium text-indigo-800 dark:text-indigo-300">Additional data mappers</div>
                                        <div className="text-xs text-indigo-700 dark:text-indigo-400">
                                            Run after the main operation, in the order shown, only when this response is selected. Matching output keys fill response root fields automatically. Use Mapper output overrides for other target paths. Earlier writes remain if a later operation fails.
                                        </div>
                                    </div>
                                </div>

                                <div className="flex items-center justify-end">
                                    <button type="button" onClick={() => setMappers([...mappers, emptyMapper()])} className="inline-flex items-center gap-1 text-xs font-medium text-primary-700 dark:text-primary-300 hover:underline flex-shrink-0">
                                        <Plus className="w-3.5 h-3.5" /> Add mapper
                                    </button>
                                </div>
                                {mappers.length === 0 && <p className="text-xs text-gray-400 dark:text-slate-500">No additional mappers configured.</p>}
                                <div className="space-y-3">
                                    {mappers.map((m, i) => (
                                        <div key={i} className="rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden">
                                            <div className="flex items-center gap-2 bg-indigo-50/60 dark:bg-indigo-950/20 px-3 py-1.5 border-b border-gray-200 dark:border-slate-800">
                                                <Layers className="w-3.5 h-3.5 text-indigo-600 dark:text-indigo-400 flex-shrink-0" />
                                                <span className="text-xs font-semibold text-indigo-700 dark:text-indigo-300">
                                                    Additional mapper {m.outputKey ? `— ${m.outputKey}` : `#${i + 1}`}
                                                </span>
                                            </div>
                                            <div className="p-3 space-y-3">
                                                <div className="flex flex-wrap items-end gap-2">
                                                    <div className="w-40">
                                                        <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Output key</label>
                                                        <input
                                                            value={m.outputKey}
                                                            onChange={(e) => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, outputKey: e.target.value } : mm)))}
                                                            placeholder="plan"
                                                            className={`${inputClass} font-mono`}
                                                        />
                                                    </div>
                                                    <div className="w-36">
                                                        <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Mode</label>
                                                        <select
                                                            value={m.mode}
                                                            onChange={(e) => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, mode: e.target.value as CollectionOpType, filters: e.target.value === 'insert' ? [] : mm.filters, data: writesData(e.target.value as CollectionOpType) ? mm.data : [] } : mm)))}
                                                            className={inputClass}
                                                        >
                                                            <option value="find-one">Find One</option>
                                                            <option value="find-many">Find Many</option>
                                                            <option value="insert">Insert</option>
                                                            <option value="update">Update</option>
                                                            <option value="upsert">Upsert</option>
                                                            <option value="delete">Delete</option>
                                                        </select>
                                                    </div>
                                                    <div className="flex-1 min-w-[10rem]">
                                                        <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Collection</label>
                                                        <input
                                                            value={m.collectionName}
                                                            onChange={(e) => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, collectionName: e.target.value } : mm)))}
                                                            placeholder="plans"
                                                            className={`${inputClass} font-mono`}
                                                        />
                                                    </div>
                                                    <button type="button" disabled={i === 0} className="px-2 py-1 text-sm text-gray-600 dark:text-slate-300 disabled:opacity-30" title="Move mapper up" aria-label="Move mapper up" onClick={() => setMappers((current) => { const next = [...current]; [next[i - 1], next[i]] = [next[i], next[i - 1]]; return next })}>↑</button>
                                                    <button type="button" disabled={i === mappers.length - 1} className="px-2 py-1 text-sm text-gray-600 dark:text-slate-300 disabled:opacity-30" title="Move mapper down" aria-label="Move mapper down" onClick={() => setMappers((current) => { const next = [...current]; [next[i + 1], next[i]] = [next[i], next[i + 1]]; return next })}>↓</button>
                                                    <button type="button" onClick={() => setMappers(mappers.filter((_, j) => j !== i))} className="p-2 text-gray-400 dark:text-slate-500 hover:text-red-600" title="Remove mapper">
                                                        <Trash2 className="w-4 h-4" />
                                                    </button>
                                                </div>
                                                {writesData(m.mode) && <DataRulesEditor collectionName={m.collectionName} rows={m.data} allowPrimary={derivedRootKind === 'object'} onChange={(data) => setMappers(mappers.map((mm, j) => j === i ? { ...mm, data } : mm))} />}
                                                {m.mode !== 'insert' && <div>
                                                    <div className="flex items-center justify-between mb-1">
                                                        <span className="text-xs font-medium text-gray-500 dark:text-slate-400">Filters</span>
                                                        <button
                                                            type="button"
                                                            onClick={() => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, filters: [...mm.filters, emptyRow()] } : mm)))}
                                                            className="inline-flex items-center gap-1 text-xs font-medium text-primary-700 dark:text-primary-300 hover:underline"
                                                        >
                                                            <Plus className="w-3 h-3" /> Add filter
                                                        </button>
                                                    </div>
                                                    <div className="space-y-2">
                                                        {m.filters.map((row, k) => (
                                                            <BindingRow
                                                                collectionName={m.collectionName}
                                                                key={k}
                                                                row={row}
                                                                sources={derivedRootKind === 'object' ? MAPPER_FILTER_SOURCES : FILTER_SOURCES}
                                                                keyPlaceholder={filterKeyPlaceholder}
                                                                targetPathLabel="Collection field"
                                                                targetPathPlaceholder="_id"
                                                                onChange={(next) => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, filters: mm.filters.map((r, l) => (l === k ? next : r)) } : mm)))}
                                                                onRemove={() => setMappers(mappers.map((mm, j) => (j === i ? { ...mm, filters: mm.filters.filter((_, l) => l !== k) } : mm)))}
                                                            />
                                                        ))}
                                                    </div>
                                                </div>}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}

                        {activeTab === 'headers' && (
                            <div className="p-4 space-y-2 max-w-2xl">
                                <div className="flex gap-2 mb-2">
                                    <input value={headerKey} onChange={(e) => setHeaderKey(e.target.value)} placeholder="Header name" className={`${inputClass} flex-1`} />
                                    <input value={headerValue} onChange={(e) => setHeaderValue(e.target.value)} placeholder="Header value" className={`${inputClass} flex-1`} />
                                    <button type="button" onClick={addHeader} className="px-4 py-2 bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-200 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-700 text-sm">Add</button>
                                </div>
                                {Object.entries(headers).length > 0 ? (
                                    <div className="bg-gray-50 dark:bg-slate-800 rounded-lg p-3 space-y-1">
                                        {Object.entries(headers).map(([key, value]) => (
                                            <div key={key} className="flex items-center justify-between text-sm">
                                                <span><span className="font-medium text-gray-900 dark:text-slate-100">{key}:</span> <span className="text-gray-700 dark:text-slate-300">{value}</span></span>
                                                <button type="button" onClick={() => removeHeader(key)} className="text-gray-400 dark:text-slate-500 hover:text-red-600"><Trash2 className="w-4 h-4" /></button>
                                            </div>
                                        ))}
                                    </div>
                                ) : (
                                    <p className="text-xs text-gray-400 dark:text-slate-500">No response headers configured.</p>
                                )}
                            </div>
                        )}

                        {activeTab === 'output' && (
                            <div className="p-4 space-y-4">
                                <div className="flex items-center justify-between">
                                    <p className="text-xs text-gray-400 dark:text-slate-500 max-w-xl">
                                        Root fields first use additional mapper outputs with matching names, then document fields. Overrides take priority. Use a mapper key such as addresses for its whole output or addresses.0.city for a nested value. Objects and arrays include only fields defined by the response schema/example.
                                    </p>
                                    <button type="button" onClick={() => setOverrides([...overrides, emptyRow('document')])} className="inline-flex items-center gap-1 text-xs font-medium text-primary-700 dark:text-primary-300 hover:underline flex-shrink-0 ml-3">
                                        <Plus className="w-3.5 h-3.5" /> Add override
                                    </button>
                                </div>
                                <label className="flex items-center gap-2 text-xs text-gray-600 dark:text-slate-300">
                                    <input type="checkbox" checked={fallbackToExample} onChange={(e) => setFallbackToExample(e.target.checked)} />
                                    When a field has no matching document value, keep the spec's example value (otherwise render null)
                                </label>
                                {overrides.length === 0 && <p className="text-xs text-gray-400 dark:text-slate-500">No overrides — every field fills by convention.</p>}
                                <div className="space-y-2">
                                    {overrides.map((row, i) => (
                                        <BindingRow
                                            key={i}
                                            row={row}
                                            sources={OVERRIDE_SOURCES}
                                            keyPlaceholder={filterKeyPlaceholder}
                                            targetPathLabel="Response field path"
                                            targetPathPlaceholder="customer.name"
                                            onChange={(next) => setOverrides(overrides.map((r, j) => (j === i ? next : r)))}
                                            onRemove={() => setOverrides(overrides.filter((_, j) => j !== i))}
                                        />
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </div>
        </MappingHintsProvider>
    )
}
