import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
    ArrowLeft,
    Bot,
    FileCode2,
    ChevronRight,
    Sparkles,
    Radio,
    Save,
    Fingerprint,
    Plus,
    Trash2,
} from 'lucide-react'
import clsx from 'clsx'
import { specsApi, operationsApi, tagsApi, aiApi } from '../../services/api'
import ScriptBindingsPanel from '../ScriptManager/ScriptBindingsPanel'
import CollectionMappingsPanel from '../CollectionMapper/CollectionMappingsPanel'
import ConditionEditor, { conditionsToTree, BASIC_SOURCES } from '../shared/ConditionEditor'
import type {
    AIStatus,
    ConditionNode,
    ModePolicy,
    OperationSummary,
    Spec,
} from '../../types'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
    HEAD: 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300',
    OPTIONS: 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300',
}

const createDefaultPolicy = (): ModePolicy => ({
    configured: true,
    ai: { enabled: false, conditions: [] },
    proxy: { enabled: false, conditions: [] },
})

const clonePolicy = (policy?: ModePolicy): ModePolicy => ({
    configured: true,
    ai: {
        enabled: policy?.ai.enabled ?? false,
        conditions: [],
        conditionTree: policy?.ai.conditionTree ?? conditionsToTree(policy?.ai.conditions ?? []),
    },
    proxy: {
        enabled: policy?.proxy.enabled ?? false,
        disableRecording: policy?.proxy.disableRecording ?? false,
        conditions: [],
        conditionTree: policy?.proxy.conditionTree ?? conditionsToTree(policy?.proxy.conditions ?? []),
    },
})

export default function SpecDetail() {
    const { specId } = useParams<{ specId: string }>()
    const queryClient = useQueryClient()
    const [backendURIInput, setBackendURIInput] = useState<string | null>(null)
    const [draftPolicy, setDraftPolicy] = useState<ModePolicy | null>(null)
    const [policyError, setPolicyError] = useState('')
    const [newSignatureHeader, setNewSignatureHeader] = useState('')

    const { data: spec, isLoading: specLoading } = useQuery<Spec>({
        queryKey: ['spec', specId],
        queryFn: () => specsApi.get(specId!),
        enabled: !!specId,
    })

    const { data: operations, isLoading: opsLoading } = useQuery<OperationSummary[]>({
        queryKey: ['operations', specId],
        queryFn: () => operationsApi.listBySpec(specId!),
        enabled: !!specId,
    })

    const { data: tags } = useQuery({
        queryKey: ['tags'],
        queryFn: tagsApi.list,
    })

    const { data: aiStatus = { configured: false, provider: 'openai' }, isLoading: aiConfigLoading } = useQuery<AIStatus>({
        queryKey: ['ai-status'],
        queryFn: () => aiApi.getStatus(),
        staleTime: 60_000,
    })
    const aiConfigured = aiStatus.configured
    const aiProviderLabel = aiStatus.provider === 'claude' ? 'Claude' : aiStatus.provider === 'openai' ? 'OpenAI' : 'AI provider'

    const updateTagsMutation = useMutation({
        mutationFn: (enabledTags: string[]) => specsApi.updateTags(specId!, enabledTags),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['spec', specId] })
        },
    })

    const setBackendMutation = useMutation({
        mutationFn: (uri: string) => specsApi.setBackendURI(specId!, uri),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['spec', specId] })
            setBackendURIInput(null)
        },
    })

    const updateModePolicyMutation = useMutation({
        mutationFn: (modePolicy: ModePolicy) => specsApi.updateModePolicy(specId!, modePolicy),
        onSuccess: () => {
            setPolicyError('')
            setDraftPolicy(null)
            queryClient.invalidateQueries({ queryKey: ['spec', specId] })
        },
        onError: (error: Error) => {
            setPolicyError(error.message)
        },
    })

    const updateSignatureHeadersMutation = useMutation({
        mutationFn: (signatureHeaders: string[]) => specsApi.update(specId!, { signatureHeaders }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['spec', specId] })
            setNewSignatureHeader('')
        },
    })

    if (specLoading || opsLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (!spec) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Specification not found
                </div>
            </div>
        )
    }

    const enabledTags = new Set(spec.enabledTags || [])
    const currentPolicy = draftPolicy ?? clonePolicy(spec.modePolicy ?? createDefaultPolicy())

    const toggleTag = (tagName: string) => {
        if (!specId) return
        const next = new Set(enabledTags)
        if (next.has(tagName)) {
            next.delete(tagName)
        } else {
            next.add(tagName)
        }
        updateTagsMutation.mutate(Array.from(next))
    }

    const signatureHeaders = spec.signatureHeaders || []

    const addSignatureHeader = () => {
        const next = newSignatureHeader.trim()
        if (!next) return
        updateSignatureHeadersMutation.mutate([...signatureHeaders, next])
    }

    const removeSignatureHeader = (header: string) => {
        updateSignatureHeadersMutation.mutate(signatureHeaders.filter((item) => item !== header))
    }

    const groupedOps = (operations || []).reduce((acc, op) => {
        const tag = op.operationId.split('_')[0] || 'default'
        if (!acc[tag]) acc[tag] = []
        acc[tag].push(op)
        return acc
    }, {} as Record<string, OperationSummary[]>)

    const updatePolicy = (updater: (policy: ModePolicy) => ModePolicy) => {
        setDraftPolicy((prev) => updater(clonePolicy(prev ?? spec.modePolicy ?? createDefaultPolicy())))
    }

    return (
        <div className="p-8">
            <div className="mb-8">
                <Link
                    to="/specs"
                    className="inline-flex items-center text-sm text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200 mb-4"
                >
                    <ArrowLeft className="w-4 h-4 mr-1" />
                    Back to Specifications
                </Link>

                <div className="flex items-start">
                    <div className="p-3 bg-primary-100/80 dark:bg-primary-900/40 rounded-lg">
                        <FileCode2 className="w-8 h-8 text-primary-600" />
                    </div>
                    <div className="ml-4">
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">{spec.name}</h1>
                        <p className="text-gray-500 dark:text-slate-400 mt-1">{spec.description || 'No description'}</p>
                        <div className="flex items-center gap-4 mt-3 text-sm">
                            <span className={clsx(
                                'px-2 py-1 rounded-full text-xs font-medium',
                                spec.enabled ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300' : 'bg-gray-100 text-gray-500 dark:bg-slate-800 dark:text-slate-400'
                            )}>
                                {spec.enabled ? 'Enabled' : 'Disabled'}
                            </span>
                            <span className="text-gray-500 dark:text-slate-400">
                                Version: <span className="font-medium text-gray-700 dark:text-slate-200">{spec.version}</span>
                            </span>
                            <span className="text-gray-500 dark:text-slate-400">
                                Base Path: <code className="font-mono bg-gray-100 dark:bg-slate-800 px-1 rounded">{spec.basePath || '/'}</code>
                            </span>
                        </div>
                    </div>
                </div>
            </div>

            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800">
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">
                        Operations ({operations?.length || 0})
                    </h2>
                </div>

                {operations && operations.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {Object.entries(groupedOps).map(([tag, ops]) => (
                            <div key={tag}>
                                <div className="px-6 py-3 bg-gray-50 dark:bg-slate-800 text-sm font-medium text-gray-500 dark:text-slate-300 uppercase">
                                    {tag}
                                </div>
                                {ops.map((op) => (
                                    <Link
                                        key={op.id}
                                        to={`/operations/${op.id}`}
                                        className="flex items-center justify-between px-6 py-4 hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                                    >
                                        <div className="flex items-center">
                                            <span className={clsx(
                                                'px-2 py-1 rounded text-xs font-bold uppercase w-20 text-center',
                                                methodColors[op.method] || 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                            )}>
                                                {op.method}
                                            </span>
                                            <div className="ml-4">
                                                <p className="font-mono text-sm text-gray-900 dark:text-slate-100">{op.path}</p>
                                                {op.summary && (
                                                    <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">{op.summary}</p>
                                                )}
                                            </div>
                                        </div>
                                        <div className="flex items-center text-gray-400 dark:text-slate-500">
                                            {op.hasExampleResponse && (
                                                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 mr-2" title="Has example response from spec">
                                                    <Sparkles className="w-3 h-3 mr-1" />
                                                    Example
                                                </span>
                                            )}
                                            <span className="text-sm mr-2">
                                                {op.responseCount} response{op.responseCount !== 1 ? 's' : ''}
                                            </span>
                                            <ChevronRight className="w-5 h-5" />
                                        </div>
                                    </Link>
                                ))}
                            </div>
                        ))}
                    </div>
                ) : (
                    <div className="p-12 text-center text-gray-500 dark:text-slate-400">
                        No operations found in this specification
                    </div>
                )}
            </div>

            <ScriptBindingsPanel kind="spec" specId={spec.id} />

            <CollectionMappingsPanel kind="spec" specId={spec.id} />

            {/* Proxy Configuration */}
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center gap-3">
                    <div className="p-2 bg-violet-100 dark:bg-violet-900/30 rounded-lg">
                        <Radio className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                    </div>
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Proxy Configuration</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                            Forward requests to a real backend. Add conditions to proxy only matching requests; leave conditions empty to proxy everything when enabled.
                        </p>
                    </div>
                </div>

                <div className="p-6 space-y-5">
                    {policyError && (
                        <div className="rounded-lg border border-red-200 bg-red-50 dark:bg-red-950/30 dark:border-red-900/40 px-3 py-2 text-sm text-red-700 dark:text-red-300">
                            {policyError}
                        </div>
                    )}

                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                            Upstream URL
                        </label>
                        <div className="flex items-center gap-3">
                            <input
                                type="url"
                                placeholder="https://api.example.com"
                                value={backendURIInput !== null ? backendURIInput : (spec.backendUri || '')}
                                onChange={e => setBackendURIInput(e.target.value)}
                                className="flex-1 px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-violet-500 dark:focus:ring-violet-400"
                            />
                            <button
                                onClick={() => setBackendMutation.mutate(backendURIInput ?? spec.backendUri ?? '')}
                                disabled={backendURIInput === null || setBackendMutation.isPending}
                                className="inline-flex items-center gap-2 px-4 py-2 bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                            >
                                <Save className="w-4 h-4" />
                                Save URL
                            </button>
                        </div>
                    </div>

                    <div className="flex items-center justify-between py-3 border-t border-b border-gray-100 dark:border-slate-800">
                        <div>
                            <span className="text-sm font-medium text-gray-800 dark:text-slate-200">Proxy forwarding</span>
                            {!spec.backendUri && (
                                <p className="text-xs text-violet-600 dark:text-violet-400 mt-0.5">
                                    Set an upstream URL above before enabling.
                                </p>
                            )}
                        </div>
                        <button
                            type="button"
                            disabled={!spec.backendUri}
                            onClick={() => updatePolicy((policy) => ({
                                ...policy,
                                proxy: { ...policy.proxy, enabled: !policy.proxy.enabled },
                            }))}
                            className={clsx(
                                'px-3 py-1.5 rounded-lg text-sm border',
                                currentPolicy.proxy.enabled
                                    ? 'bg-violet-100 text-violet-700 border-violet-300 dark:bg-violet-900/30 dark:text-violet-300 dark:border-violet-700'
                                    : 'bg-gray-100 text-gray-600 border-gray-300 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700',
                                !spec.backendUri && 'opacity-50 cursor-not-allowed'
                            )}
                        >
                            {currentPolicy.proxy.enabled ? 'Enabled' : 'Disabled'}
                        </button>
                    </div>

                    <label className="flex items-start gap-3 cursor-pointer select-none">
                        <input
                            type="checkbox"
                            checked={!currentPolicy.proxy.disableRecording}
                            onChange={() => updatePolicy((policy) => ({
                                ...policy,
                                proxy: {
                                    ...policy.proxy,
                                    disableRecording: !policy.proxy.disableRecording,
                                },
                            }))}
                            className="mt-0.5 h-4 w-4 rounded border-gray-300 dark:border-slate-600 text-violet-600 focus:ring-violet-500"
                        />
                        <div>
                            <span className="text-sm font-medium text-gray-800 dark:text-slate-200">
                                Record proxied responses
                            </span>
                            <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">
                                Automatically save backend responses for replay. Uncheck for pure pass-through.
                            </p>
                        </div>
                    </label>

                    <ConditionEditor
                        label="Proxy conditions"
                        value={currentPolicy.proxy.conditionTree}
                        onChange={(conditionTree: ConditionNode | undefined) => updatePolicy((policy) => ({
                            ...policy,
                            proxy: { ...policy.proxy, conditions: [], conditionTree },
                        }))}
                        sources={BASIC_SOURCES}
                        emptyHint="No conditions — proxy applies to all requests when enabled."
                        compact
                    />

                    <div className="flex items-center justify-end gap-3 pt-1">
                        <button
                            type="button"
                            onClick={() => { setDraftPolicy(null); setPolicyError('') }}
                            className="px-4 py-2 rounded-lg border border-gray-300 dark:border-slate-700 text-sm text-gray-700 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800"
                        >
                            Reset
                        </button>
                        <button
                            type="button"
                            onClick={() => updateModePolicyMutation.mutate(currentPolicy)}
                            disabled={updateModePolicyMutation.isPending}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                        >
                            <Save className="w-4 h-4" />
                            Save Proxy Config
                        </button>
                    </div>
                </div>
            </div>

            {/* AI Fallback */}
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center gap-3">
                    <div className="p-2 bg-fuchsia-100 dark:bg-fuchsia-900/30 rounded-lg">
                        <Bot className="w-5 h-5 text-fuchsia-600 dark:text-fuchsia-400" />
                    </div>
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">AI Fallback</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                            Generate responses on-the-fly when no saved response matches. Add conditions to limit which requests are handled by AI.
                        </p>
                    </div>
                </div>

                <div className="p-6 space-y-5">
                    <div className="flex items-center justify-between py-3 border-b border-gray-100 dark:border-slate-800">
                        <div>
                            <span className="text-sm font-medium text-gray-800 dark:text-slate-200">AI generation</span>
                            {!aiConfigured && (
                                <p className="text-xs text-fuchsia-600 dark:text-fuchsia-400 mt-0.5">
                                    Configure {aiProviderLabel} to enable.
                                </p>
                            )}
                        </div>
                        <button
                            type="button"
                            disabled={aiConfigLoading || !aiConfigured}
                            onClick={() => updatePolicy((policy) => ({
                                ...policy,
                                ai: { ...policy.ai, enabled: !policy.ai.enabled },
                            }))}
                            className={clsx(
                                'px-3 py-1.5 rounded-lg text-sm border',
                                currentPolicy.ai.enabled
                                    ? 'bg-fuchsia-100 text-fuchsia-700 border-fuchsia-300 dark:bg-fuchsia-900/30 dark:text-fuchsia-300 dark:border-fuchsia-700'
                                    : 'bg-gray-100 text-gray-600 border-gray-300 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700',
                                (!aiConfigured || aiConfigLoading) && 'opacity-50 cursor-not-allowed'
                            )}
                        >
                            {currentPolicy.ai.enabled ? 'Enabled' : 'Disabled'}
                        </button>
                    </div>

                    <ConditionEditor
                        label="AI conditions"
                        value={currentPolicy.ai.conditionTree}
                        onChange={(conditionTree: ConditionNode | undefined) => updatePolicy((policy) => ({
                            ...policy,
                            ai: { ...policy.ai, conditions: [], conditionTree },
                        }))}
                        sources={BASIC_SOURCES}
                        emptyHint="No conditions — AI handles all unmatched requests when enabled."
                        compact
                    />

                    <div className="flex items-center justify-end gap-3 pt-1">
                        <button
                            type="button"
                            onClick={() => { setDraftPolicy(null); setPolicyError('') }}
                            className="px-4 py-2 rounded-lg border border-gray-300 dark:border-slate-700 text-sm text-gray-700 dark:text-slate-200 hover:bg-gray-50 dark:hover:bg-slate-800"
                        >
                            Reset
                        </button>
                        <button
                            type="button"
                            onClick={() => updateModePolicyMutation.mutate(currentPolicy)}
                            disabled={updateModePolicyMutation.isPending}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                        >
                            <Save className="w-4 h-4" />
                            Save AI Config
                        </button>
                    </div>
                </div>
            </div>

            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center gap-3">
                    <div className="p-2 bg-violet-100 dark:bg-violet-900/30 rounded-lg">
                        <Fingerprint className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                    </div>
                    <div>
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Default Signature Headers</h2>
                        <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                            These headers are included by default in request signatures across all operations in this spec unless an operation explicitly overrides its header set.
                        </p>
                    </div>
                </div>
                <div className="p-6">
                    <div className="flex flex-wrap gap-2 mb-4">
                        {signatureHeaders.length > 0 ? signatureHeaders.map((header) => (
                            <span key={header} className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300 font-mono">
                                {header}
                                <button
                                    type="button"
                                    onClick={() => removeSignatureHeader(header)}
                                    className="hover:text-red-500"
                                    disabled={updateSignatureHeadersMutation.isPending}
                                >
                                    <Trash2 className="w-3 h-3" />
                                </button>
                            </span>
                        )) : (
                            <p className="text-sm text-gray-400 dark:text-slate-500 italic">No spec-level signature headers configured</p>
                        )}
                    </div>
                    <div className="flex gap-2">
                        <input
                            type="text"
                            value={newSignatureHeader}
                            onChange={(e) => setNewSignatureHeader(e.target.value)}
                            placeholder="Add header name…"
                            onKeyDown={(e) => {
                                if (e.key === 'Enter') {
                                    addSignatureHeader()
                                }
                            }}
                            className="flex-1 px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-violet-500"
                        />
                        <button
                            type="button"
                            onClick={addSignatureHeader}
                            disabled={updateSignatureHeadersMutation.isPending}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50"
                        >
                            <Plus className="w-4 h-4" />
                            Add Header
                        </button>
                    </div>
                </div>
            </div>

            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800">
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Enabled Tags</h2>
                    <p className="text-sm text-gray-500 dark:text-slate-400 mt-1">
                        Responses tagged with enabled tags are considered for this spec. The default tag is always included.
                    </p>
                </div>
                <div className="p-6 flex flex-wrap gap-2">
                    {(tags || []).map((tag: { name: string }) => (
                        <button
                            key={tag.name}
                            type="button"
                            onClick={() => toggleTag(tag.name)}
                            disabled={tag.name === 'default'}
                            className={clsx(
                                'px-3 py-1.5 rounded-full text-sm font-medium border transition-colors',
                                tag.name === 'default'
                                    ? 'bg-gray-100 text-gray-500 border-gray-200 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700'
                                    : enabledTags.has(tag.name)
                                        ? 'bg-primary-50 text-primary-700 border-primary-200 dark:bg-primary-900/30 dark:text-primary-200 dark:border-primary-800'
                                        : 'bg-white text-gray-600 border-gray-200 hover:bg-gray-50 dark:bg-slate-900 dark:text-slate-300 dark:border-slate-700 dark:hover:bg-slate-800'
                            )}
                            title={tag.name === 'default' ? 'Default tag is always enabled' : undefined}
                        >
                            {tag.name}
                        </button>
                    ))}
                </div>
            </div>
        </div>
    )
}
