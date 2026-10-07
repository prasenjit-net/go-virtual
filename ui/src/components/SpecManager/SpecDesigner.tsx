import { useEffect, useMemo, useReducer, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Bot, Braces, ChevronDown, ChevronRight, Database, FileCode2, FileCog, FileText, Fingerprint, Loader2, Network, Plus, Redo2, Reply, Route, Save, Search, Settings2, ShieldCheck, Trash2, Undo2, Workflow, X } from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi, specsApi } from '../../services/api'
import type { CollectionMapping, Operation, ResponseConfig, Script, ScriptBinding, SignatureConfig, SpecMode, SpecWorkspace, ValidationRule } from '../../types'
import ResponseConfigIDE from '../ResponseDesigner/ResponseConfigIDE'
import CollectionResponseEditor from '../ResponseDesigner/CollectionResponseEditor'
import { CollectionMappingDraftEditor } from '../CollectionMapper/CollectionMappingsPanel'
import { ValidationRuleDraftEditor } from '../ValidationManager/ValidationRulesPanel'

type NodeKind = 'spec' | 'specSignature' | 'proxy' | 'ai' | 'group' | 'operation' | 'signature' | 'response' | 'validation' | 'script' | 'mapping'
type NodeRef = { kind: NodeKind; id?: string; operationId?: string; responseId?: string }
type NavNode = { key: string; label: string; kind: NodeKind; ref: NodeRef; detail?: string; group?: 'pipeline' | 'responses'; children?: NavNode[] }
type RecoveryDraft = { schema: 1; revision: string; workspace: SpecWorkspace; updatedAt: number }
type DraftState = { workspace: SpecWorkspace | null; past: SpecWorkspace[]; future: SpecWorkspace[] }
type DraftAction = { type: 'edit'; update: (workspace: SpecWorkspace) => SpecWorkspace } | { type: 'undo' } | { type: 'redo' } | { type: 'replace'; workspace: SpecWorkspace }
type ToastNotice = { tone: 'success' | 'error'; message: string }

function clone<T>(value: T): T { return structuredClone(value) }
function reducer(state: DraftState, action: DraftAction): DraftState {
    if (action.type === 'replace') return { workspace: action.workspace, past: [], future: [] }
    if (!state.workspace) return state
    if (action.type === 'undo') {
        const previous = state.past[state.past.length - 1]
        return previous ? { workspace: previous, past: state.past.slice(0, -1), future: [state.workspace, ...state.future].slice(0, 40) } : state
    }
    if (action.type === 'redo') {
        const next = state.future[0]
        return next ? { workspace: next, past: [...state.past, state.workspace].slice(-40), future: state.future.slice(1) } : state
    }
    const next = action.update(state.workspace)
    if (JSON.stringify(next) === JSON.stringify(state.workspace)) return state
    return { workspace: next, past: [...state.past, state.workspace].slice(-40), future: [] }
}

const dbPromise = typeof indexedDB === 'undefined' ? Promise.reject(new Error('IndexedDB unavailable')) : new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open('go-virtual-spec-designer', 1)
    request.onupgradeneeded = () => request.result.createObjectStore('drafts', { keyPath: 'key' })
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
})
async function readRecovery(id: string): Promise<RecoveryDraft | undefined> {
    const db = await dbPromise
    return new Promise((resolve, reject) => {
        const request = db.transaction('drafts').objectStore('drafts').get(id)
        request.onsuccess = () => resolve(request.result?.value)
        request.onerror = () => reject(request.error)
    })
}
async function writeRecovery(id: string, value: RecoveryDraft): Promise<void> {
    const db = await dbPromise
    await new Promise<void>((resolve, reject) => {
        const request = db.transaction('drafts', 'readwrite').objectStore('drafts').put({ key: id, value })
        request.onsuccess = () => resolve()
        request.onerror = () => reject(request.error)
    })
}
async function clearRecovery(id: string): Promise<void> {
    const db = await dbPromise
    await new Promise<void>((resolve, reject) => {
        const request = db.transaction('drafts', 'readwrite').objectStore('drafts').delete(id)
        request.onsuccess = () => resolve()
        request.onerror = () => reject(request.error)
    })
}

function buildNodes(workspace: SpecWorkspace): NavNode[] {
    const nodes: NavNode[] = [
        { key: 'spec', label: 'Spec overview', kind: 'spec', ref: { kind: 'spec' }, detail: workspace.spec.name },
        { key: 'spec-signature', label: 'Signature config', kind: 'specSignature', ref: { kind: 'specSignature' }, detail: `${workspace.spec.signatureHeaders?.length || 0} default headers` },
        { key: 'spec-proxy', label: 'Proxy', kind: 'proxy', ref: { kind: 'proxy' }, detail: workspace.spec.modePolicy.proxy.enabled ? 'Enabled' : 'Disabled' },
        { key: 'spec-ai', label: 'AI', kind: 'ai', ref: { kind: 'ai' }, detail: workspace.spec.modePolicy.ai.enabled ? 'Enabled' : 'Disabled' },
    ]
    const specPipeline: NavNode[] = [
        ...workspace.specValidations.map(entry => ({ key: `validation:spec:${entry.id}`, label: entry.name || 'Unnamed validation', kind: 'validation' as const, ref: { kind: 'validation' as const, id: entry.id }, detail: entry.enabled ? 'Enabled' : 'Disabled' })),
        ...workspace.specMappings.map(entry => ({ key: `mapping:spec:${entry.id}`, label: entry.name || entry.outputKey || 'Collection mapping', kind: 'mapping' as const, ref: { kind: 'mapping' as const, id: entry.id }, detail: `${entry.operation} · ${entry.collectionName}` })),
        ...workspace.specScripts.map(entry => ({ key: `script:spec:${entry.id}`, label: entry.outputKey || entry.scriptName || 'Script binding', kind: 'script' as const, ref: { kind: 'script' as const, id: entry.id }, detail: `order ${entry.order}` })),
    ]
    nodes.push({ key: 'group:spec-pipeline', label: 'Pipeline', kind: 'group', group: 'pipeline', ref: { kind: 'group' }, children: specPipeline })
    for (const entry of workspace.operations) {
        const op = entry.operation
        const opChildren: NavNode[] = [
            { key: `signature:${op.id}`, label: 'Request signature', kind: 'signature', ref: { kind: 'signature', operationId: op.id }, detail: 'Settings' },
            {
                key: `group:pipeline:${op.id}`, label: 'Pipeline', kind: 'group', group: 'pipeline', ref: { kind: 'group', operationId: op.id }, children: [
                    ...entry.validations.map(v => ({ key: `validation:${op.id}:${v.id}`, label: v.name || 'Unnamed validation', kind: 'validation' as const, ref: { kind: 'validation' as const, id: v.id, operationId: op.id }, detail: v.enabled ? 'Enabled' : 'Disabled' })),
                    ...entry.mappings.map(m => ({ key: `mapping:${op.id}:${m.id}`, label: m.name || m.outputKey || 'Collection mapping', kind: 'mapping' as const, ref: { kind: 'mapping' as const, id: m.id, operationId: op.id }, detail: `${m.operation} · ${m.collectionName}` })),
                    ...entry.scripts.map(s => ({ key: `script:${op.id}:${s.id}`, label: s.outputKey || s.scriptName || 'Script binding', kind: 'script' as const, ref: { kind: 'script' as const, id: s.id, operationId: op.id }, detail: `order ${s.order}` })),
                ],
            },
            {
                key: `group:responses:${op.id}`, label: 'Responses', kind: 'group', group: 'responses', ref: { kind: 'group', operationId: op.id }, children: entry.responses.map(({ response }) => ({
                    key: `response:${response.id}`, label: response.name || `${response.statusCode} response`, kind: 'response' as const,
                    ref: { kind: 'response' as const, id: response.id, operationId: op.id }, detail: `${response.statusCode} · ${response.kind || 'manual'}`,
                })),
            },
        ]
        nodes.push({ key: `operation:${op.id}`, label: `${op.method} ${op.path}`, kind: 'operation', ref: { kind: 'operation', id: op.id }, detail: op.summary, children: opChildren })
    }
    return nodes
}
function flattenNodes(nodes: NavNode[]): NavNode[] { return nodes.flatMap(node => [node, ...flattenNodes(node.children || [])]) }
function filterNodes(nodes: NavNode[], term: string): NavNode[] {
    if (!term) return nodes
    return nodes.flatMap(node => {
        const children = filterNodes(node.children || [], term)
        if (`${node.label} ${node.detail || ''}`.toLowerCase().includes(term)) return [node]
        return children.length ? [{ ...node, children }] : []
    })
}

function objectFor(workspace: SpecWorkspace, ref: NodeRef): unknown {
    if (ref.kind === 'spec' || ref.kind === 'specSignature' || ref.kind === 'proxy' || ref.kind === 'ai') return workspace.spec
    if ((ref.kind === 'validation' || ref.kind === 'script' || ref.kind === 'mapping') && !ref.operationId) {
        const list = ref.kind === 'validation' ? workspace.specValidations : ref.kind === 'script' ? workspace.specScripts : workspace.specMappings
        return list.find(item => item.id === ref.id) || null
    }
    const op = workspace.operations.find(item => item.operation.id === (ref.operationId || ref.id))
    if (!op) return null
    if (ref.kind === 'operation') return op.operation
    if (ref.kind === 'signature') return op.operation.signatureConfig ?? null
    if (ref.kind === 'response') return op.responses.find(item => item.response.id === ref.id)?.response || null
    if (ref.kind === 'validation') return op.validations.find(item => item.id === ref.id) || null
    if (ref.kind === 'script') {
        if (ref.responseId) return op.responses.find(item => item.response.id === ref.responseId)?.scripts.find(item => item.id === ref.id) || null
        return op.scripts.find(item => item.id === ref.id) || null
    }
    if (ref.kind === 'mapping') {
        if (ref.responseId) return op.responses.find(item => item.response.id === ref.responseId)?.mappings.find(item => item.id === ref.id) || null
        return op.mappings.find(item => item.id === ref.id) || null
    }
    return null
}

function patchObject(workspace: SpecWorkspace, ref: NodeRef, value: unknown): SpecWorkspace {
    const next = clone(workspace)
    if (ref.kind === 'spec' || ref.kind === 'specSignature' || ref.kind === 'proxy' || ref.kind === 'ai') { next.spec = { ...next.spec, ...(value as Partial<SpecWorkspace['spec']>), id: next.spec.id, content: next.spec.content, version: next.spec.version, createdAt: next.spec.createdAt }; return next }
    if ((ref.kind === 'validation' || ref.kind === 'script' || ref.kind === 'mapping') && !ref.operationId) {
        const list = ref.kind === 'validation' ? next.specValidations : ref.kind === 'script' ? next.specScripts : next.specMappings
        const item = list.find(entry => entry.id === ref.id)
        if (item) Object.assign(item, value, { id: item.id, specId: next.spec.id, operationId: undefined, responseConfigId: undefined })
        return next
    }
    const op = next.operations.find(item => item.operation.id === (ref.operationId || ref.id))
    if (!op) return next
    if (ref.kind === 'operation' && ref.id === op.operation.id) return next
    if (ref.kind === 'signature') { op.operation.signatureConfig = value as typeof op.operation.signatureConfig; return next }
    if (ref.kind === 'response') {
        const response = op.responses.find(item => item.response.id === ref.id)
        if (response) response.response = { ...response.response, ...(value as Partial<typeof response.response>), id: response.response.id, operationId: op.operation.id }
        return next
    }
    if (ref.responseId && (ref.kind === 'script' || ref.kind === 'mapping')) {
        const response = op.responses.find(item => item.response.id === ref.responseId)
        const list = ref.kind === 'script' ? response?.scripts : response?.mappings
        const item = list?.find(entry => entry.id === ref.id)
        if (item) Object.assign(item, value, { id: item.id, specId: undefined, operationId: undefined, responseConfigId: response?.response.id })
        return next
    }
    const list = ref.kind === 'validation' ? (ref.operationId ? op.validations : next.specValidations) : ref.kind === 'script' ? (ref.operationId ? op.scripts : next.specScripts) : (ref.operationId ? op.mappings : next.specMappings)
    const item = list.find(entry => entry.id === ref.id)
    if (item) Object.assign(item, value, { id: item.id })
    return next
}

function removeObject(workspace: SpecWorkspace, ref: NodeRef): SpecWorkspace {
    const next = clone(workspace)
    if (ref.kind === 'response') {
        const op = next.operations.find(item => item.operation.id === ref.operationId)
        if (op) op.responses = op.responses.filter(item => item.response.id !== ref.id)
        return next
    }
    const op = next.operations.find(item => item.operation.id === ref.operationId)
    const list = ref.kind === 'validation' ? (ref.operationId ? op?.validations : next.specValidations) : ref.kind === 'script' ? (ref.operationId ? op?.scripts : next.specScripts) : ref.kind === 'mapping' ? (ref.operationId ? op?.mappings : next.specMappings) : undefined
    if (list) {
        const index = list.findIndex(item => item.id === ref.id)
        if (index >= 0) list.splice(index, 1)
    }
    return next
}

function OperationOverview({ operation, onOpenSignature, onAddResponse, onAddValidation, onAddMapping }: {
    operation: Operation
    onOpenSignature: () => void
    onAddResponse: () => void
    onAddValidation: () => void
    onAddMapping: () => void
}) {
    return <section className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="border-b border-gray-200 p-5 dark:border-slate-800"><div className="flex flex-wrap items-center gap-2"><span className="rounded bg-primary-100 px-2 py-1 font-mono text-xs font-bold text-primary-800 dark:bg-primary-900/40 dark:text-primary-200">{operation.method}</span><h2 className="font-mono text-lg font-semibold">{operation.path}</h2></div><p className="mt-2 text-sm text-gray-500 dark:text-slate-400">{operation.summary || operation.description || 'Imported OpenAPI operation'}</p></div>
        <div className="grid gap-3 p-5 sm:grid-cols-3"><div className="rounded-lg bg-gray-50 p-3 dark:bg-slate-950"><div className="text-xs text-gray-500">Path inputs</div><div className="mt-1 text-sm font-medium">{operation.declaredPathParams?.join(', ') || 'None'}</div></div><div className="rounded-lg bg-gray-50 p-3 dark:bg-slate-950"><div className="text-xs text-gray-500">Query inputs</div><div className="mt-1 text-sm font-medium">{operation.declaredQueryParams?.join(', ') || 'None'}</div></div><div className="rounded-lg bg-gray-50 p-3 dark:bg-slate-950"><div className="text-xs text-gray-500">Request body</div><div className="mt-1 text-sm font-medium">{operation.hasRequestBody ? 'Configured' : 'None'}</div></div></div>
        <div className="flex flex-wrap gap-2 border-t border-gray-200 p-5 dark:border-slate-800"><button onClick={onOpenSignature} className="rounded border px-3 py-2 text-sm dark:border-slate-700">Request signature</button><button onClick={onAddResponse} className="rounded bg-primary-600 px-3 py-2 text-sm text-white">Add response</button><button onClick={onAddValidation} className="rounded border px-3 py-2 text-sm dark:border-slate-700">Add validation</button><button onClick={onAddMapping} className="rounded border px-3 py-2 text-sm dark:border-slate-700">Add collection mapping</button></div>
    </section>
}

function SignatureDraftEditor({ operation, onChange }: { operation: Operation; onChange: (config: SignatureConfig | null) => void }) {
    const config: SignatureConfig = operation.signatureConfig || { pathParams: [], queryParams: [], headersConfigured: false, headers: [], includeBody: null, bodyJsonPaths: [] }
    const update = (patch: Partial<SignatureConfig>) => onChange({ ...config, ...patch })
    const toggle = (field: 'pathParams' | 'queryParams' | 'headers', value: string) => update({ [field]: config[field].includes(value) ? config[field].filter(item => item !== value) : [...config[field], value] })
    const chips = (label: string, values: string[] | undefined, field: 'pathParams' | 'queryParams' | 'headers') => <div><div className="mb-2 text-sm font-medium">{label}</div>{values?.length ? <div className="flex flex-wrap gap-2">{values.map(value => <button key={value} type="button" onClick={() => toggle(field, value)} className={clsx('rounded-full border px-3 py-1 text-xs font-mono', config[field].includes(value) ? 'border-violet-400 bg-violet-50 text-violet-700 dark:border-violet-700 dark:bg-violet-950/40 dark:text-violet-300' : 'border-gray-300 text-gray-600 dark:border-slate-700 dark:text-slate-300')}>{value}</button>)}</div> : <p className="text-sm text-gray-400">No declared {label.toLowerCase()}.</p>}</div>
    return <section className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"><div className="border-b border-gray-200 p-5 dark:border-slate-800"><h2 className="text-lg font-semibold">Signature Configuration</h2><p className="mt-1 text-sm text-gray-500 dark:text-slate-400">Choose which request values identify a configured response.</p></div><div className="space-y-6 p-5">{chips('Path parameters', operation.declaredPathParams, 'pathParams')}{chips('Query parameters', operation.declaredQueryParams, 'queryParams')}<div><div className="mb-2 flex items-center justify-between"><div className="text-sm font-medium">Headers</div><button type="button" onClick={() => update({ headersConfigured: !config.headersConfigured, headers: config.headersConfigured ? [] : config.headers })} className="rounded border px-2 py-1 text-xs dark:border-slate-700">{config.headersConfigured ? 'Use defaults' : 'Customize headers'}</button></div>{config.headersConfigured ? chips('Declared headers', operation.declaredHeaderParams, 'headers') : <p className="text-sm text-gray-500 dark:text-slate-400">Uses the spec defaults until you customize this operation.</p>}</div><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={config.includeBody === true} disabled={!operation.hasRequestBody} onChange={event => update({ includeBody: event.target.checked, bodyJsonPaths: event.target.checked ? config.bodyJsonPaths : [] })} />Include request body</label></div></section>
}

function SpecSignatureDraftEditor({ headers, onChange }: { headers: string[]; onChange: (headers: string[]) => void }) {
    const [value, setValue] = useState(headers.join(', '))
    useEffect(() => setValue(headers.join(', ')), [headers])
    return <section className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"><div className="border-b border-gray-200 p-5 dark:border-slate-800"><h2 className="text-lg font-semibold">Default signature headers</h2><p className="mt-1 text-sm text-gray-500 dark:text-slate-400">These headers are included in response matching unless an operation customizes its signature.</p></div><div className="p-5"><label className="text-sm font-medium">Headers<input value={value} onChange={event => setValue(event.target.value)} onBlur={() => onChange(value.split(',').map(header => header.trim()).filter(Boolean))} placeholder="X-Tenant-Id, X-Region" className="mt-2 w-full rounded border border-gray-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-slate-700" /></label><p className="mt-2 text-xs text-gray-500 dark:text-slate-400">Use a comma-separated list. Changes apply to this workspace draft.</p></div></section>
}

function SpecModeDraftEditor({ mode, spec, onChange }: { mode: 'proxy' | 'ai'; spec: SpecWorkspace['spec']; onChange: (patch: Partial<SpecWorkspace['spec']>) => void }) {
    const policy = spec.modePolicy[mode]
    const enable = (enabled: boolean) => onChange({ mode: enabled ? mode : 'standard', modePolicy: { ...spec.modePolicy, [mode]: { ...policy, enabled } } })
    const label = mode === 'proxy' ? 'Proxy' : 'AI'
    return <section className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"><div className="border-b border-gray-200 p-5 dark:border-slate-800"><h2 className="text-lg font-semibold">{label} mode</h2><p className="mt-1 text-sm text-gray-500 dark:text-slate-400">Configure this spec’s {label.toLowerCase()} response behavior.</p></div><div className="space-y-5 p-5"><label className="flex items-center gap-2 text-sm font-medium"><input type="checkbox" checked={policy.enabled} onChange={event => enable(event.target.checked)} />Enable {label} mode</label>{mode === 'proxy' && <label className="block text-sm font-medium">Backend URI<input value={spec.backendUri} onChange={event => onChange({ backendUri: event.target.value })} placeholder="https://service.internal" className="mt-2 w-full rounded border border-gray-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-slate-700" /></label>}<p className="text-xs text-gray-500 dark:text-slate-400">{policy.enabled ? `${label} mode is enabled for this draft.` : `${label} mode is disabled for this draft.`}</p></div></section>
}

function ScriptBindingDraftEditor({ binding, onChange }: { binding: ScriptBinding; onChange: (binding: ScriptBinding) => void }) {
    const { data: scripts = [] } = useQuery<Script[]>({ queryKey: ['scripts'], queryFn: scriptsApi.list, staleTime: 60_000 })
    return <section className="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"><div className="border-b border-gray-200 p-5 dark:border-slate-800"><h2 className="text-lg font-semibold">Script Binding</h2><p className="mt-1 text-sm text-gray-500 dark:text-slate-400">This binding runs in the selected pipeline scope when the workspace is saved.</p></div><div className="grid gap-4 p-5 sm:grid-cols-2"><label className="text-sm">Script<select value={binding.scriptId} onChange={e => { const script = scripts.find(item => item.id === e.target.value); onChange({ ...binding, scriptId: e.target.value, scriptName: script?.name || '' }) }} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 dark:border-slate-700"><option value="">Select a script…</option>{scripts.map(script => <option key={script.id} value={script.id} disabled={!script.enabled}>{script.name}{script.enabled ? '' : ' (disabled)'}</option>)}</select></label><label className="text-sm">Output key<input value={binding.outputKey} onChange={e => onChange({ ...binding, outputKey: e.target.value.replace(/[^A-Za-z0-9_]/g, '') })} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 font-mono dark:border-slate-700" /></label><label className="text-sm">Order<input type="number" min="0" value={binding.order} onChange={e => onChange({ ...binding, order: Number(e.target.value) || 0 })} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 dark:border-slate-700" /></label><label className="mt-6 flex items-center gap-2 text-sm"><input type="checkbox" checked={binding.enabled} onChange={e => onChange({ ...binding, enabled: e.target.checked })} />Enabled</label></div></section>
}

export default function SpecDesigner() {
    const { specId = '' } = useParams<{ specId: string }>()
    const navigate = useNavigate()
    const queryClient = useQueryClient()
    const [searchParams, setSearchParams] = useSearchParams()
    const [state, dispatch] = useReducer(reducer, { workspace: null, past: [], future: [] })
    const [baseline, setBaseline] = useState('')
    const [initializedSpec, setInitializedSpec] = useState('')
    const [recovery, setRecovery] = useState<RecoveryDraft | null>(null)
    const [restorePrompt, setRestorePrompt] = useState(false)
    const [selected, setSelected] = useState(searchParams.get('item') || 'spec')
    const [search, setSearch] = useState('')
    const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set())
    const [addMenu, setAddMenu] = useState<string | null>(null)
    const addMenuRef = useRef<HTMLDivElement | null>(null)
    const [treeCollapsed, setTreeCollapsed] = useState(() => localStorage.getItem('spec-designer-tree-collapsed') === 'true')
    const [contextOpen, setContextOpen] = useState(() => localStorage.getItem('spec-designer-context-open') === 'true')
    const [error, setError] = useState('')
    const [toast, setToast] = useState<ToastNotice | null>(null)
    const closingAfterSave = useRef(false)
    const [mobilePane, setMobilePane] = useState<'tree' | 'editor' | 'context'>('editor')
    const recoveryWrite = useRef<number | undefined>(undefined)
    const workspaceQuery = useQuery({ queryKey: ['spec-workspace', specId], queryFn: () => specsApi.getWorkspace(specId), enabled: !!specId, staleTime: 0 })
    const workspace = state.workspace
    const dirty = !!workspace && !!baseline && JSON.stringify(workspace) !== baseline
    const nodes = useMemo(() => workspace ? buildNodes(workspace) : [], [workspace])
    const matchingNodes = useMemo(() => filterNodes(nodes, search.trim().toLowerCase()), [nodes, search])
    const selectedNode = flattenNodes(nodes).find(node => node.key === selected) || nodes[0]
    const selectedObject = workspace && selectedNode ? objectFor(workspace, selectedNode.ref) : null
    const selectedOperation = workspace && selectedNode
        ? workspace.operations.find(item => item.operation.id === (selectedNode.ref.operationId || (selectedNode.kind === 'operation' ? selectedNode.ref.id : undefined)))
        : undefined
    const contextAvailable = !!selectedOperation && ['response', 'mapping', 'signature'].includes(selectedNode?.kind || '')

    useEffect(() => {
        if (!toast) return
        const timer = window.setTimeout(() => setToast(null), toast.tone === 'success' ? 3_500 : 8_000)
        return () => window.clearTimeout(timer)
    }, [toast])

    useEffect(() => {
        if (!addMenu) return
        const dismiss = (event: PointerEvent) => {
            if (!addMenuRef.current?.contains(event.target as Node)) setAddMenu(null)
        }
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setAddMenu(null)
        }
        document.addEventListener('pointerdown', dismiss)
        document.addEventListener('keydown', onKeyDown)
        return () => {
            document.removeEventListener('pointerdown', dismiss)
            document.removeEventListener('keydown', onKeyDown)
        }
    }, [addMenu])

    useEffect(() => {
        if (!workspaceQuery.data || workspaceQuery.data.spec.id !== specId || initializedSpec === specId) return
        const serverWorkspace = workspaceQuery.data
        let cancelled = false
        void readRecovery(specId).then(stored => {
            if (cancelled) return
            setBaseline(JSON.stringify(serverWorkspace))
            if (stored && JSON.stringify(stored.workspace) !== JSON.stringify(serverWorkspace)) {
                setRecovery(stored)
                setRestorePrompt(true)
                dispatch({ type: 'replace', workspace: clone(serverWorkspace) })
            } else {
                dispatch({ type: 'replace', workspace: clone(serverWorkspace) })
            }
            setInitializedSpec(specId)
        }).catch(() => {
            if (cancelled) return
            setBaseline(JSON.stringify(serverWorkspace))
            dispatch({ type: 'replace', workspace: clone(serverWorkspace) })
            setInitializedSpec(specId)
        })
        return () => { cancelled = true }
    }, [workspaceQuery.data, specId, initializedSpec])

    useEffect(() => {
        if (!workspace || initializedSpec !== specId || !dirty) return
        window.clearTimeout(recoveryWrite.current)
        recoveryWrite.current = window.setTimeout(() => {
            void writeRecovery(specId, { schema: 1, revision: workspace.revision, workspace: clone(workspace), updatedAt: Date.now() }).catch(() => setError('Browser draft recovery is unavailable; keep this tab open until you save.'))
        }, 450)
        return () => window.clearTimeout(recoveryWrite.current)
    }, [workspace, dirty, initializedSpec, specId])

    useEffect(() => {
        const onKey = (event: KeyboardEvent) => {
            if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') { event.preventDefault(); if (dirty && !saveMutation.isPending) saveMutation.mutate() }
            if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'z' && !event.shiftKey) { event.preventDefault(); dispatch({ type: 'undo' }) }
            if ((event.metaKey || event.ctrlKey) && (event.key.toLowerCase() === 'y' || event.shiftKey && event.key.toLowerCase() === 'z')) { event.preventDefault(); dispatch({ type: 'redo' }) }
        }
        window.addEventListener('keydown', onKey)
        return () => window.removeEventListener('keydown', onKey)
    })

    const saveMutation = useMutation({
        mutationFn: () => {
            if (!workspace) throw new Error('Workspace is not loaded')
            return specsApi.saveWorkspace(specId, workspace)
        },
        onSuccess: async result => {
            const saved = result.workspace
            dispatch({ type: 'replace', workspace: clone(saved) })
            setBaseline(JSON.stringify(saved))
            setRecovery(null)
            setRestorePrompt(false)
            setError('')
            setToast({ tone: 'success', message: 'Workspace saved.' })
            let selectedKey = selected
            for (const [draftID, serverID] of Object.entries(result.idMap)) selectedKey = selectedKey.split(draftID).join(serverID)
            if (selectedKey !== selected) { setSelected(selectedKey); setSearchParams({ item: selectedKey }, { replace: true }) }
            await clearRecovery(specId).catch(() => undefined)
            await queryClient.invalidateQueries({ queryKey: ['spec-workspace', specId] })
            await queryClient.invalidateQueries({ queryKey: ['spec', specId] })
            await queryClient.invalidateQueries({ queryKey: ['specs'] })
            if (closingAfterSave.current) {
                closingAfterSave.current = false
                navigate('/specs')
            }
        },
        onError: (e: Error & { status?: number }) => {
            closingAfterSave.current = false
            const message = e.status === 409 ? 'The server workspace changed. Your local draft is preserved; reload the server copy or copy your work before resolving the conflict.' : e.message
            setError(message)
            setToast({ tone: 'error', message: `Save failed: ${message}` })
        },
    })

    const selectNode = (node: NavNode) => {
        if (node.kind === 'group') {
            setCollapsedGroups(current => {
                const next = new Set(current)
                if (next.has(node.key)) next.delete(node.key)
                else next.add(node.key)
                return next
            })
            return
        }
        setSelected(node.key)
        setSearchParams({ item: node.key }, { replace: true })
        setMobilePane('editor')
    }
    const updateSelected = (value: unknown) => {
        if (!selectedNode) return
        dispatch({ type: 'edit', update: current => patchObject(current, selectedNode.ref, value) })
        setError('')
    }
    function updateSpecField<K extends keyof SpecWorkspace['spec']>(key: K, value: SpecWorkspace['spec'][K]) {
        dispatch({ type: 'edit', update: current => ({ ...current, spec: { ...current.spec, [key]: value } }) })
    }
    const addResponse = (opId: string, kind: 'manual' | 'collection' = 'manual') => {
        const id = `draft-${crypto.randomUUID()}`
        const response: ResponseConfig = { id, operationId: opId, name: kind === 'collection' ? 'New collection response' : 'New response', description: '', tag: 'default', priority: 0, conditions: [], statusCode: 200, headers: {}, body: '', delay: 0, enabled: false, recorded: false, origin: 'manual', kind, ...(kind === 'collection' ? { collectionResponse: { primary: { collectionName: '', mode: 'find-one' }, additionalMappers: [], overrides: [], rootKind: 'object' } } : {}) }
        dispatch({ type: 'edit', update: current => ({ ...current, operations: current.operations.map(item => item.operation.id === opId ? { ...item, responses: [...item.responses, { response, scripts: [], mappings: [] }] } : item) }) })
        const key = `response:${id}`; setSelected(key); setSearchParams({ item: key }, { replace: true })
    }
    const addValidation = (opId?: string) => {
        const id = `draft-${crypto.randomUUID()}`
        const rule: ValidationRule = { id, ...(opId ? { operationId: opId } : { specId }), name: 'New validation', description: '', order: 0, enabled: false, conditionTree: undefined, onSuccess: {}, onFailure: {}, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
        dispatch({ type: 'edit', update: current => opId ? { ...current, operations: current.operations.map(item => item.operation.id === opId ? { ...item, validations: [...item.validations, rule] } : item) } : { ...current, specValidations: [...current.specValidations, rule] } })
        const key = opId ? `validation:${opId}:${id}` : `validation:spec:${id}`; setSelected(key); setSearchParams({ item: key }, { replace: true })
    }
    const addMapping = (opId?: string) => {
        const id = `draft-${crypto.randomUUID()}`
        const mapping: CollectionMapping = { id, ...(opId ? { operationId: opId } : { specId }), collectionName: '', name: 'New collection mapping', operation: 'find-one', filterRules: [], dataRules: [], outputKey: 'result', order: 0, enabled: false }
        dispatch({ type: 'edit', update: current => opId ? { ...current, operations: current.operations.map(item => item.operation.id === opId ? { ...item, mappings: [...item.mappings, mapping] } : item) } : { ...current, specMappings: [...current.specMappings, mapping] } })
        const key = opId ? `mapping:${opId}:${id}` : `mapping:spec:${id}`; setSelected(key); setSearchParams({ item: key }, { replace: true })
    }
    const addScript = (opId?: string) => {
        const id = `draft-${crypto.randomUUID()}`
        const script: ScriptBinding = { id, ...(opId ? { operationId: opId } : { specId }), scriptId: '', scriptName: '', outputKey: 'result', order: 0, enabled: false }
        dispatch({ type: 'edit', update: current => opId ? { ...current, operations: current.operations.map(item => item.operation.id === opId ? { ...item, scripts: [...item.scripts, script] } : item) } : { ...current, specScripts: [...current.specScripts, script] } })
        const key = opId ? `script:${opId}:${id}` : `script:spec:${id}`; setSelected(key); setSearchParams({ item: key }, { replace: true })
    }
    const removeNode = (node: NavNode) => {
        if (!workspace || !['response', 'validation', 'script', 'mapping'].includes(node.kind)) return
        if (!window.confirm(`Delete ${node.label} from this draft? It will be removed from the server when you save.`)) return
        dispatch({ type: 'edit', update: current => removeObject(current, node.ref) })
        const nextKey = node.ref.operationId ? `operation:${node.ref.operationId}` : 'spec'
        setSelected(nextKey); setSearchParams({ item: nextKey }, { replace: true })
    }
    const close = () => {
        if (dirty) { closingAfterSave.current = false; setShowCloseDialog(true); return }
        navigate('/specs')
    }
    const [showCloseDialog, setShowCloseDialog] = useState(false)
    useEffect(() => { setSelected(searchParams.get('item') || 'spec') }, [searchParams])
    const selectRecovery = (restore: boolean) => {
        if (restore && recovery) {
            dispatch({ type: 'replace', workspace: clone(recovery.workspace) })
            if (recovery.revision !== workspaceQuery.data?.revision) setError('This browser draft is based on an older server revision. Save will be blocked until you compare it with the current server version.')
        } else if (specId) void clearRecovery(specId).catch(() => undefined)
        setRestorePrompt(false)
        setRecovery(null)
    }
    const discardChanges = () => {
        if (!baseline) return
        dispatch({ type: 'replace', workspace: clone(JSON.parse(baseline) as SpecWorkspace) })
        setError('')
        setRecovery(null)
        void clearRecovery(specId).catch(() => undefined)
        setToast({ tone: 'success', message: 'Unsaved changes discarded.' })
    }

    if (workspaceQuery.isError) return <div className="m-6 rounded-lg border border-red-200 bg-red-50 p-4 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">{(workspaceQuery.error as Error).message}</div>
    if (workspaceQuery.isLoading || initializedSpec !== specId || !workspace) return <div className="h-full grid place-items-center"><div className="flex items-center gap-3 text-sm text-gray-500 dark:text-slate-400"><Loader2 className="h-5 w-5 animate-spin" />Loading spec workspace…</div></div>

    const renderTree = (node: NavNode, depth = 0) => {
        const NodeIcon = node.kind === 'group'
            ? node.group === 'responses' ? Reply : Workflow
            : node.kind === 'spec'
                ? FileCog
                : node.kind === 'specSignature' || node.kind === 'signature'
                    ? Fingerprint
                    : node.kind === 'proxy'
                        ? Network
                        : node.kind === 'ai'
                            ? Bot
                : node.kind === 'operation'
                    ? Route
                        : node.kind === 'response'
                            ? (node.detail?.includes('collection') ? Database : FileText)
                            : node.kind === 'validation'
                                ? ShieldCheck
                                : node.kind === 'script'
                                    ? FileCode2
                                    : Braces
        const iconClass = selected === node.key
            ? 'text-primary-600 dark:text-primary-300'
            : node.kind === 'response' && node.detail?.includes('collection')
                ? 'text-teal-600 dark:text-teal-400'
                : node.kind === 'validation'
                    ? 'text-violet-600 dark:text-violet-400'
                    : 'text-gray-400 dark:text-slate-500'

        const hasChildren = !!node.children?.length
        const isCollapsed = node.kind === 'group' && collapsedGroups.has(node.key) && !search.trim()
        return <div key={node.key}>
            <div className="group flex items-center gap-1">
                <button type="button" onClick={() => selectNode(node)} className={clsx('min-w-0 flex-1 rounded-md px-2 py-1.5 text-left text-xs', node.kind === 'group' && 'font-semibold uppercase tracking-wide text-[10px]', selected === node.key ? 'bg-primary-50 text-primary-800 dark:bg-primary-900/30 dark:text-primary-200' : 'text-gray-700 hover:bg-gray-100 dark:text-slate-300 dark:hover:bg-slate-800')} style={{ paddingLeft: `${8 + depth * 10}px` }} title={node.detail} aria-expanded={node.kind === 'group' ? !isCollapsed : undefined}>
                    <span className="flex items-center gap-2 truncate">{node.kind === 'group' && <ChevronDown className={clsx('h-3 w-3 shrink-0 transition-transform', isCollapsed && '-rotate-90')} />}<NodeIcon className={clsx('h-3.5 w-3.5 shrink-0', iconClass)} /><span className="truncate">{node.label}</span></span>
                    {node.detail && <span className="ml-4 block truncate text-[10px] opacity-60">{node.detail}</span>}
                </button>
                {node.kind === 'group' && <div className="relative" ref={addMenu === node.key ? addMenuRef : undefined}><button type="button" title={`Add to ${node.label.toLowerCase()}`} onClick={() => setAddMenu(current => current === node.key ? null : node.key)} className="rounded p-1 opacity-0 hover:bg-gray-200 group-hover:opacity-100 dark:hover:bg-slate-700"><Plus className="h-3.5 w-3.5" /></button>{addMenu === node.key && <div className="absolute right-0 z-20 mt-1 w-44 rounded-md border border-gray-200 bg-white p-1 shadow-lg dark:border-slate-700 dark:bg-slate-900">{node.group === 'pipeline' ? <><button type="button" onClick={() => { setAddMenu(null); addValidation(node.ref.operationId) }} className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-slate-800"><ShieldCheck className="h-3.5 w-3.5 text-violet-500" />Validation</button><button type="button" onClick={() => { setAddMenu(null); addMapping(node.ref.operationId) }} className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-slate-800"><Database className="h-3.5 w-3.5 text-teal-500" />Collection mapper</button><button type="button" onClick={() => { setAddMenu(null); addScript(node.ref.operationId) }} className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-slate-800"><FileCode2 className="h-3.5 w-3.5 text-indigo-500" />Script</button></> : <><button type="button" onClick={() => { setAddMenu(null); addResponse(node.ref.operationId!) }} className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-slate-800"><FileText className="h-3.5 w-3.5 text-primary-500" />Manual response</button><button type="button" onClick={() => { setAddMenu(null); addResponse(node.ref.operationId!, 'collection') }} className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs hover:bg-gray-100 dark:hover:bg-slate-800"><Database className="h-3.5 w-3.5 text-teal-500" />Collection response</button></>}</div>}</div>}
                {['response', 'validation', 'script', 'mapping'].includes(node.kind) && <button type="button" onClick={() => removeNode(node)} title={`Delete ${node.label} from draft`} className="rounded p-1 text-gray-400 opacity-0 hover:bg-red-50 hover:text-red-600 group-hover:opacity-100 dark:text-slate-500 dark:hover:bg-red-950/40 dark:hover:text-red-300"><Trash2 className="h-3.5 w-3.5" /></button>}
            </div>
            {hasChildren && !isCollapsed && <div className="ml-2 border-l border-gray-200 dark:border-slate-800">{node.children!.map(child => renderTree(child, depth + 1))}</div>}
        </div>
    }
    return <div className="relative flex h-full min-h-[calc(100vh-3.5rem)] flex-col bg-gray-50 dark:bg-slate-950 lg:min-h-screen">
        <header className="z-10 flex min-h-14 flex-wrap items-center gap-2 border-b border-gray-200 bg-white px-3 py-2 dark:border-slate-800 dark:bg-slate-900">
            <button onClick={close} className="rounded-md p-2 hover:bg-gray-100 dark:hover:bg-slate-800" aria-label="Close designer"><ArrowLeft className="h-4 w-4" /></button>
            <div className="min-w-0 flex-1"><div className="truncate text-sm font-semibold">{workspace.spec.name || 'Untitled spec'}</div><div className="mt-0.5 flex items-center gap-1.5 text-[10px]">{saveMutation.isPending ? <span className="font-medium text-primary-600 dark:text-primary-300">Saving changes…</span> : dirty ? <span className="rounded-full bg-amber-100 px-1.5 py-0.5 font-semibold text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">Unsaved changes · save required</span> : <span className="text-emerald-700 dark:text-emerald-300">All changes saved</span>}{workspaceQuery.isFetching ? <span className="text-gray-500 dark:text-slate-400">· refreshing</span> : null}</div></div>
            <button disabled={!state.past.length} onClick={() => dispatch({ type: 'undo' })} className="rounded p-2 disabled:opacity-30" title="Undo (Ctrl/Cmd+Z)"><Undo2 className="h-4 w-4" /></button>
            <button disabled={!state.future.length} onClick={() => dispatch({ type: 'redo' })} className="rounded p-2 disabled:opacity-30" title="Redo (Ctrl/Cmd+Shift+Z)"><Redo2 className="h-4 w-4" /></button>
            <button onClick={() => { setTreeCollapsed(v => !v); localStorage.setItem('spec-designer-tree-collapsed', String(!treeCollapsed)) }} className="hidden rounded p-2 hover:bg-gray-100 dark:hover:bg-slate-800 lg:block" title="Collapse navigation"><ChevronDown className={clsx('h-4 w-4 transition-transform', treeCollapsed && '-rotate-90')} /></button>
            {contextAvailable && <button onClick={() => { setContextOpen(v => !v); localStorage.setItem('spec-designer-context-open', String(!contextOpen)) }} className="hidden rounded p-2 hover:bg-gray-100 dark:hover:bg-slate-800 lg:block" title="Toggle context panel"><Settings2 className="h-4 w-4" /></button>}
            <button onClick={() => { if (treeCollapsed) { setTreeCollapsed(false); localStorage.setItem('spec-designer-tree-collapsed', 'false') } setMobilePane('tree') }} className="rounded px-2 py-1 text-xs lg:hidden">Navigate</button>
            {contextAvailable && <button onClick={() => { setContextOpen(true); setMobilePane('context') }} className="rounded px-2 py-1 text-xs lg:hidden">Context</button>}
            <button disabled={!dirty || saveMutation.isPending} onClick={discardChanges} className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-800 hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200 dark:hover:bg-amber-950/70">Reset</button>
            <button disabled={!dirty || saveMutation.isPending} onClick={() => saveMutation.mutate()} className="flex items-center gap-2 rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50"><Save className="h-4 w-4" />{saveMutation.isPending ? 'Saving…' : 'Save'}</button>
        </header>
        {error && <div role="alert" className="flex items-center justify-between border-b border-red-200 bg-red-50 px-4 py-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300"><span>{error}</span><button onClick={() => setError('')}><X className="h-4 w-4" /></button></div>}
        {toast && <div role="status" aria-live="polite" className={clsx('fixed right-4 top-20 z-50 flex max-w-md items-start gap-3 rounded-lg border px-4 py-3 text-sm shadow-lg', toast.tone === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/90 dark:text-emerald-200' : 'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950/90 dark:text-red-200')}><span className="flex-1">{toast.message}</span><button type="button" onClick={() => setToast(null)} aria-label="Dismiss notification" className="-mr-1 rounded p-0.5 hover:bg-black/10"><X className="h-4 w-4" /></button></div>}
        <div className="flex min-h-0 flex-1">
            <aside className={clsx('min-h-0 shrink-0 border-r border-gray-200 bg-white transition-[width] dark:border-slate-800 dark:bg-slate-900', treeCollapsed ? 'w-0 overflow-hidden lg:w-12 lg:p-1' : 'w-full lg:w-72', mobilePane !== 'tree' && 'hidden lg:block')}>
                <div className={clsx('flex h-full min-h-0 flex-col', treeCollapsed && 'lg:items-center')}>
                    {treeCollapsed ? <button onClick={() => { setTreeCollapsed(false); localStorage.setItem('spec-designer-tree-collapsed', 'false') }} title="Expand navigation" className="hidden p-2 lg:block"><ChevronRight className="h-4 w-4" /></button> : <>
                        <div className="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-slate-800"><span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500 dark:text-slate-400">Workspace</span><button onClick={close} className="rounded p-1 lg:hidden"><X className="h-4 w-4" /></button></div>
                        <label className="mx-2 my-2 flex items-center gap-2 rounded-md border border-gray-200 px-2 dark:border-slate-700"><Search className="h-3 w-3 text-gray-400" /><input value={search} onChange={e => setSearch(e.target.value)} className="min-w-0 flex-1 bg-transparent py-1.5 text-xs outline-none" placeholder="Find an operation or item" /></label>
                        <div className="min-h-0 flex-1 overflow-y-auto p-2">{matchingNodes.map(node => renderTree(node))}</div>
                    </>}
                </div>
            </aside>
            <main className={clsx('min-w-0 flex-1 overflow-y-auto p-4 sm:p-6', mobilePane !== 'editor' && 'hidden lg:block')}>
                {selectedNode && <div className="w-full space-y-5">
                    {selectedNode.kind === 'spec' && <section className="grid gap-4 rounded-xl border border-gray-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900 sm:grid-cols-2">
                        <label className="text-xs">Name<input value={workspace.spec.name} onChange={e => updateSpecField('name', e.target.value)} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 text-sm dark:border-slate-700" /></label>
                        <label className="text-xs">Base path<input value={workspace.spec.basePath} onChange={e => updateSpecField('basePath', e.target.value)} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-slate-700" /></label>
                        <label className="text-xs sm:col-span-2">Description<textarea value={workspace.spec.description} onChange={e => updateSpecField('description', e.target.value)} rows={3} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 text-sm dark:border-slate-700" /></label>
                        <label className="text-xs">Mode<select value={workspace.spec.mode} onChange={e => { const mode = e.target.value as SpecMode; updateSpecField('mode', mode); updateSpecField('modePolicy', { ...workspace.spec.modePolicy, configured: true, ai: { ...workspace.spec.modePolicy.ai, enabled: mode === 'ai' }, proxy: { ...workspace.spec.modePolicy.proxy, enabled: mode === 'proxy' } }) }} className="mt-1 block rounded border border-gray-300 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-950"><option value="standard">Standard</option><option value="ai">AI</option><option value="proxy">Proxy</option></select></label>
                        <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={workspace.spec.enabled} onChange={e => updateSpecField('enabled', e.target.checked)} />Enabled</label>
                        <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={workspace.spec.tracing} onChange={e => updateSpecField('tracing', e.target.checked)} />Capture traces</label>
                        <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={workspace.spec.useExampleFallback} onChange={e => updateSpecField('useExampleFallback', e.target.checked)} />Use OpenAPI examples as fallback</label>
                        <label className="text-xs sm:col-span-2">Backend URI<input value={workspace.spec.backendUri} onChange={e => updateSpecField('backendUri', e.target.value)} placeholder="https://service.internal" className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-slate-700" /></label>
                        <label className="text-xs sm:col-span-2">Enabled tags (comma separated)<input value={(workspace.spec.enabledTags || []).join(', ')} onChange={e => updateSpecField('enabledTags', e.target.value.split(',').map(tag => tag.trim()).filter(Boolean))} className="mt-1 w-full rounded border border-gray-300 bg-transparent px-3 py-2 text-sm dark:border-slate-700" /></label>
                        <div className="text-xs text-gray-500 dark:text-slate-400 sm:col-span-2">Imported OpenAPI contract · {workspace.spec.version} · {workspace.operations.length} operations. Contract paths, schemas, and examples are read-only.</div>
                    </section>}
                    {selectedNode.kind === 'specSignature' && <SpecSignatureDraftEditor headers={workspace.spec.signatureHeaders || []} onChange={headers => updateSpecField('signatureHeaders', headers)} />}
                    {selectedNode.kind === 'proxy' && <SpecModeDraftEditor mode="proxy" spec={workspace.spec} onChange={updateSelected as (patch: Partial<SpecWorkspace['spec']>) => void} />}
                    {selectedNode.kind === 'ai' && <SpecModeDraftEditor mode="ai" spec={workspace.spec} onChange={updateSelected as (patch: Partial<SpecWorkspace['spec']>) => void} />}
                    {selectedNode.kind === 'operation' && selectedOperation && <OperationOverview operation={selectedOperation.operation} onOpenSignature={() => { const key = `signature:${selectedOperation.operation.id}`; setSelected(key); setSearchParams({ item: key }, { replace: true }) }} onAddResponse={() => addResponse(selectedOperation.operation.id)} onAddValidation={() => addValidation(selectedOperation.operation.id)} onAddMapping={() => addMapping(selectedOperation.operation.id)} />}
                    {selectedNode.kind === 'signature' && selectedOperation && <SignatureDraftEditor operation={selectedOperation.operation} onChange={(signatureConfig) => updateSelected(signatureConfig)} />}
                    {selectedNode.kind === 'response' && selectedObject !== null && selectedOperation && ((selectedObject as ResponseConfig).kind === 'collection' ? <CollectionResponseEditor key={selectedNode.key} operationId={selectedOperation.operation.id} config={selectedObject as ResponseConfig} onClose={() => selectNode({ key: `operation:${selectedOperation.operation.id}`, label: 'Operation', kind: 'operation', ref: { kind: 'operation', id: selectedOperation.operation.id } })} onDraftChange={updateSelected} /> : <ResponseConfigIDE key={selectedNode.key} operationId={selectedOperation.operation.id} config={selectedObject as ResponseConfig} onSaved={() => undefined} onDraftChange={updateSelected} />)}
                    {selectedNode.kind === 'validation' && selectedObject !== null && <ValidationRuleDraftEditor key={selectedNode.key} rule={selectedObject as ValidationRule} onChange={updateSelected as (rule: ValidationRule) => void} />}
                    {selectedNode.kind === 'mapping' && selectedObject !== null && <CollectionMappingDraftEditor key={selectedNode.key} mapping={selectedObject as CollectionMapping} operation={selectedOperation?.operation} onChange={updateSelected as (mapping: CollectionMapping) => void} />}
                    {selectedNode.kind === 'script' && selectedObject !== null && <ScriptBindingDraftEditor key={selectedNode.key} binding={selectedObject as ScriptBinding} onChange={updateSelected as (binding: ScriptBinding) => void} />}
                </div>}
            </main>
            {contextAvailable && contextOpen && <aside className={clsx('min-h-0 shrink-0 border-l border-gray-200 bg-white dark:border-slate-800 dark:bg-slate-900', 'w-full lg:w-80 xl:w-96', mobilePane !== 'context' && 'hidden lg:block')}>
                <div className="flex h-full min-h-0 flex-col"><div className="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-slate-800"><span className="text-[10px] font-semibold uppercase tracking-wider text-gray-500 dark:text-slate-400">OpenAPI context</span><button onClick={() => setMobilePane('editor')} className="rounded p-1 lg:hidden"><X className="h-4 w-4" /></button></div><div className="min-h-0 flex-1 overflow-auto p-4 text-sm"><div className="mb-4"><div className="font-mono font-semibold">{selectedOperation?.operation.method} {selectedOperation?.operation.path}</div><p className="mt-1 text-xs text-gray-500 dark:text-slate-400">Imported request fields available to this editor.</p></div><div className="space-y-3"><div><div className="text-xs font-semibold uppercase tracking-wide text-gray-500">Path parameters</div><div className="mt-1 font-mono text-xs">{selectedOperation?.operation.declaredPathParams?.join(', ') || 'None'}</div></div><div><div className="text-xs font-semibold uppercase tracking-wide text-gray-500">Query parameters</div><div className="mt-1 font-mono text-xs">{selectedOperation?.operation.declaredQueryParams?.join(', ') || 'None'}</div></div><div><div className="text-xs font-semibold uppercase tracking-wide text-gray-500">Headers</div><div className="mt-1 font-mono text-xs">{selectedOperation?.operation.declaredHeaderParams?.join(', ') || 'None'}</div></div>{selectedNode?.kind === 'response' && <div className="rounded-lg border border-primary-100 bg-primary-50 p-3 text-xs text-primary-800 dark:border-primary-900/50 dark:bg-primary-950/30 dark:text-primary-200">Response schemas and named examples remain defined by the uploaded OpenAPI contract. Use the response editor’s example chooser to select an available example.</div>}</div></div></div>
            </aside>}
        </div>
        {restorePrompt && recovery && <div className="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4"><div role="dialog" aria-modal="true" className="w-full max-w-lg rounded-xl bg-white p-5 shadow-xl dark:bg-slate-900"><h2 className="text-lg font-semibold">Browser draft found</h2><p className="my-3 text-sm text-gray-600 dark:text-slate-300">A local draft from {new Date(recovery.updatedAt).toLocaleString()} is available. {recovery.revision === workspace.revision ? 'It uses this server revision.' : 'It was started from an older server revision; restoring it will require conflict resolution before saving.'}</p><div className="flex justify-end gap-2"><button onClick={() => selectRecovery(false)} className="rounded border px-3 py-2 text-sm dark:border-slate-700">Use server version</button><button onClick={() => selectRecovery(true)} className="rounded bg-primary-600 px-3 py-2 text-sm text-white">Restore local draft</button></div></div></div>}
        {showCloseDialog && <div className="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4"><div role="dialog" aria-modal="true" className="w-full max-w-md rounded-xl bg-white p-5 shadow-xl dark:bg-slate-900"><h2 className="text-lg font-semibold">Unsaved workspace changes</h2><p className="my-3 text-sm text-gray-600 dark:text-slate-300">Save your draft before closing the designer?</p><div className="flex justify-end gap-2"><button onClick={() => { setShowCloseDialog(false); closingAfterSave.current = false }} className="rounded border px-3 py-2 text-sm dark:border-slate-700">Cancel</button><button onClick={() => { void clearRecovery(specId).catch(() => undefined); setShowCloseDialog(false); navigate('/specs') }} className="rounded border border-red-200 px-3 py-2 text-sm text-red-600 dark:border-red-900 dark:text-red-300">Discard</button><button onClick={() => { closingAfterSave.current = true; setShowCloseDialog(false); saveMutation.mutate() }} disabled={saveMutation.isPending} className="rounded bg-primary-600 px-3 py-2 text-sm text-white disabled:opacity-50">Save and close</button></div></div></div>}
        <div className="sr-only" aria-live="polite">{saveMutation.isPending ? 'Saving workspace' : dirty ? 'Workspace has unsaved changes' : 'Workspace saved'}</div>
    </div>
}
