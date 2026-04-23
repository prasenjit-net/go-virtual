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
    Globe
} from 'lucide-react'
import clsx from 'clsx'
import { specsApi, operationsApi, tagsApi, aiApi } from '../../services/api'
import type { Spec, OperationSummary, SpecMode } from '../../types'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
    HEAD: 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300',
    OPTIONS: 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300',
}

export default function SpecDetail() {
    const { specId } = useParams<{ specId: string }>()
    const queryClient = useQueryClient()
    const [backendURIInput, setBackendURIInput] = useState<string | null>(null)

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

    const { data: aiConfigured = false, isLoading: aiConfigLoading } = useQuery<boolean>({
        queryKey: ['ai-configured'],
        queryFn: () => aiApi.isConfigured(),
        staleTime: 60_000,
    })

    const setModeMutation = useMutation({
        mutationFn: (mode: SpecMode) => specsApi.setMode(specId!, mode),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['spec', specId] })
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

    // Group operations by tag
    const groupedOps = (operations || []).reduce((acc, op) => {
        const tag = op.operationId.split('_')[0] || 'default'
        if (!acc[tag]) acc[tag] = []
        acc[tag].push(op)
        return acc
    }, {} as Record<string, OperationSummary[]>)

    return (
        <div className="p-8">
            {/* Header */}
            <div className="mb-8">
                <Link
                    to="/specs"
                    className="inline-flex items-center text-sm text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-200 mb-4"
                >
                    <ArrowLeft className="w-4 h-4 mr-1" />
                    Back to Specifications
                </Link>

                <div className="flex items-start justify-between">
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
            </div>

            {/* Operations */}
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

            {/* Execution Mode & Backend Configuration */}
            <div className="mt-6 bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 bg-violet-100 dark:bg-violet-900/30 rounded-lg">
                            <Globe className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                        </div>
                        <div>
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">Execution Mode</h2>
                            <p className="text-sm text-gray-500 dark:text-slate-400 mt-0.5">
                                Existing saved responses are always tried first; mode controls what happens when none match.
                            </p>
                        </div>
                    </div>
                </div>

                <div className="p-6 space-y-5">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <button
                            onClick={() => setModeMutation.mutate('standard')}
                            disabled={setModeMutation.isPending}
                            className={clsx(
                                'rounded-xl border px-4 py-3 text-left transition-colors',
                                spec.mode === 'standard'
                                    ? 'border-primary-300 bg-primary-50 text-primary-800 dark:border-primary-700 dark:bg-primary-900/30 dark:text-primary-200'
                                    : 'border-gray-200 hover:bg-gray-50 dark:border-slate-700 dark:hover:bg-slate-800 text-gray-700 dark:text-slate-300'
                            )}
                        >
                            <div className="font-semibold">Standard</div>
                            <p className="text-xs mt-1 opacity-80">
                                Fall back to the spec example/default response when no saved response matches.
                            </p>
                        </button>
                        <button
                            onClick={() => setModeMutation.mutate('ai')}
                            disabled={setModeMutation.isPending || aiConfigLoading || !aiConfigured}
                            className={clsx(
                                'rounded-xl border px-4 py-3 text-left transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
                                spec.mode === 'ai'
                                    ? 'border-fuchsia-300 bg-fuchsia-50 text-fuchsia-800 dark:border-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-200'
                                    : 'border-gray-200 hover:bg-gray-50 dark:border-slate-700 dark:hover:bg-slate-800 text-gray-700 dark:text-slate-300'
                            )}
                        >
                            <div className="font-semibold flex items-center gap-2">
                                <Bot className="w-4 h-4" />
                                AI
                            </div>
                            <p className="text-xs mt-1 opacity-80">
                                Generate a structured response with AI on misses, then save it for replay.
                            </p>
                        </button>
                        <button
                            onClick={() => setModeMutation.mutate('proxy')}
                            disabled={setModeMutation.isPending || !spec.backendUri}
                            className={clsx(
                                'rounded-xl border px-4 py-3 text-left transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
                                spec.mode === 'proxy'
                                    ? 'border-violet-300 bg-violet-50 text-violet-800 dark:border-violet-700 dark:bg-violet-900/30 dark:text-violet-200'
                                    : 'border-gray-200 hover:bg-gray-50 dark:border-slate-700 dark:hover:bg-slate-800 text-gray-700 dark:text-slate-300'
                            )}
                        >
                            <div className="font-semibold flex items-center gap-2">
                                <Radio className="w-4 h-4" />
                                Proxy
                            </div>
                            <p className="text-xs mt-1 opacity-80">
                                Forward misses to the upstream backend and save the returned response for replay.
                            </p>
                        </button>
                    </div>

                    <div className="rounded-xl border border-gray-200 dark:border-slate-800 px-4 py-3 bg-gray-50 dark:bg-slate-950 text-sm text-gray-600 dark:text-slate-300">
                        <span className="font-medium text-gray-900 dark:text-slate-100 mr-2">Active mode:</span>
                        <span className="capitalize">{spec.mode}</span>
                    </div>

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
                            Save
                        </button>
                    </div>
                    {!aiConfigured && (
                        <p className="mt-2 text-xs text-fuchsia-600 dark:text-fuchsia-400">
                            AI mode is unavailable until an OpenAI API key is configured.
                        </p>
                    )}
                    {spec.backendUri && spec.mode !== 'proxy' && (
                        <p className="mt-2 text-xs text-gray-500 dark:text-slate-400">
                            Backend configured. Switch to <strong>Proxy</strong> mode to use upstream fallback after saved responses miss.
                        </p>
                    )}
                    {spec.mode === 'proxy' && (
                        <p className="mt-2 text-xs text-violet-600 dark:text-violet-400">
                            <Radio className="w-3 h-3 inline mr-1 animate-pulse" />
                            Proxy mode active — saved responses are checked first, then unmatched requests are forwarded to <strong>{spec.backendUri}</strong> and recorded.
                        </p>
                    )}
                    {spec.mode === 'ai' && (
                        <p className="mt-2 text-xs text-fuchsia-600 dark:text-fuchsia-400">
                            <Bot className="w-3 h-3 inline mr-1" />
                            AI mode active — saved responses are checked first, then unmatched requests are generated with AI and saved for replay.
                        </p>
                    )}
                    {spec.mode === 'standard' && (
                        <p className="mt-2 text-xs text-gray-500 dark:text-slate-400">
                            Standard mode active — saved responses are checked first, then the spec example/default response is used when enabled.
                        </p>
                    )}
                </div>
            </div>

            {/* Enabled Tags */}
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
