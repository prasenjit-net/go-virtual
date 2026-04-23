import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Activity,
    Bot,
    Trash2,
    Search,
    ChevronRight,
    Clock,
    ArrowDownUp,
    RefreshCw,
    Radio,
    Fingerprint,
    Globe,
    Copy,
    Check,
    Code2,
    Terminal,
    Users
} from 'lucide-react'
import clsx from 'clsx'
import { tracesApi, specsApi } from '../services/api'
import type { Trace, Spec, ScriptTrace, SessionTrace } from '../types'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
}

const responseTierLabels: Record<string, string> = {
    configured: 'Configured',
    recorded: 'Recorded',
    fallback: 'Fallback',
}

const skipReasonLabels: Record<string, string> = {
    disabled: 'disabled',
    'not-configured': 'not configured',
    'no-backend': 'no upstream',
    'conditions-not-matched': 'conditions not matched',
}

export default function TraceViewer() {
    const [isLive, setIsLive] = useState(false)
    const [liveTraces, setLiveTraces] = useState<Trace[]>([])
    const [selectedTrace, setSelectedTrace] = useState<Trace | null>(null)
    const [specFilter, setSpecFilter] = useState('')
    const [searchQuery, setSearchQuery] = useState('')
    const wsRef = useRef<WebSocket | null>(null)
    const queryClient = useQueryClient()

    const { data: specs } = useQuery<Spec[]>({
        queryKey: ['specs'],
        queryFn: specsApi.list,
    })

    const { data: storedTraces, refetch: refetchStoredTraces } = useQuery<Trace[]>({
        queryKey: ['traces', specFilter],
        queryFn: () => tracesApi.list({ specId: specFilter || undefined }),
        enabled: !isLive,
    })

    // When switching from live to paused, refetch stored traces
    const handleToggleLive = () => {
        if (isLive) {
            // Switching to paused - refetch stored traces to include just-captured ones
            setIsLive(false)
            // Small delay to ensure server has stored the traces
            setTimeout(() => {
                refetchStoredTraces()
            }, 100)
        } else {
            // Switching to live - clear live traces and start fresh
            setLiveTraces([])
            setSelectedTrace(null)
            setIsLive(true)
        }
    }

    const clearMutation = useMutation({
        mutationFn: () => tracesApi.clear(specFilter || undefined),
        onSuccess: () => {
            setLiveTraces([])
            setSelectedTrace(null)
            queryClient.invalidateQueries({ queryKey: ['traces'] })
        },
    })

    // WebSocket connection for live traces
    useEffect(() => {
        if (!isLive) {
            if (wsRef.current) {
                wsRef.current.close()
                wsRef.current = null
            }
            return
        }

        const ws = tracesApi.createStream()
        wsRef.current = ws

        ws.onmessage = (event) => {
            const trace = JSON.parse(event.data) as Trace
            setLiveTraces((prev) => [trace, ...prev].slice(0, 100))
        }

        ws.onerror = () => {
            console.error('WebSocket error')
        }

        ws.onclose = () => {
            // Reconnect after a delay
            if (isLive) {
                setTimeout(() => {
                    if (isLive && wsRef.current === ws) {
                        wsRef.current = tracesApi.createStream()
                    }
                }, 3000)
            }
        }

        return () => {
            ws.close()
        }
    }, [isLive])

    const traces = isLive ? liveTraces : (storedTraces || [])
    const filteredTraces = traces.filter((trace) => {
        if (specFilter && trace.specId !== specFilter) return false
        if (searchQuery) {
            const query = searchQuery.toLowerCase()
            return (
                trace.request.path.toLowerCase().includes(query) ||
                trace.request.method.toLowerCase().includes(query) ||
                trace.specName.toLowerCase().includes(query)
            )
        }
        return true
    })

    // Clear detail panel when the selected trace is no longer in the visible list
    // (e.g. spec filter changed, search narrowed results, or traces were cleared)
    useEffect(() => {
        if (selectedTrace && !filteredTraces.some((t) => t.id === selectedTrace.id)) {
            setSelectedTrace(null)
        }
    }, [filteredTraces, selectedTrace])

    const formatDuration = (ns: number) => {
        const ms = ns / 1e6
        if (ms < 1) return `${(ns / 1e3).toFixed(0)}µs`
        if (ms < 1000) return `${ms.toFixed(0)}ms`
        return `${(ms / 1000).toFixed(2)}s`
    }

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="p-6 border-b border-gray-200 dark:border-slate-800 bg-white dark:bg-slate-900">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Request Traces</h1>
                        <p className="text-gray-500 dark:text-slate-400 mt-1">
                            Monitor live requests and responses
                        </p>
                    </div>
                    <div className="flex items-center gap-3">
                        {!isLive && (
                            <button
                                onClick={() => refetchStoredTraces()}
                                className="flex items-center px-4 py-2 bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-slate-200 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-700 transition-colors"
                                title="Refresh traces"
                            >
                                <RefreshCw className="w-5 h-5 mr-2" />
                                Refresh
                            </button>
                        )}
                        <button
                            onClick={handleToggleLive}
                            className={clsx(
                                'flex items-center px-4 py-2 rounded-lg font-medium transition-colors',
                                isLive
                                    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 hover:bg-green-200'
                                    : 'bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-slate-200 hover:bg-gray-200 dark:hover:bg-slate-700'
                            )}
                            title={isLive ? 'Switch to history mode' : 'Switch to live streaming'}
                        >
                            {isLive ? (
                                <>
                                    <Activity className="w-5 h-5 mr-2 animate-pulse" />
                                    Live
                                </>
                            ) : (
                                <>
                                    <Activity className="w-5 h-5 mr-2" />
                                    Go Live
                                </>
                            )}
                        </button>
                        <button
                            onClick={() => clearMutation.mutate()}
                            className="flex items-center px-4 py-2 bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 rounded-lg hover:bg-red-200 transition-colors"
                        >
                            <Trash2 className="w-5 h-5 mr-2" />
                            Clear
                        </button>
                    </div>
                </div>

                {/* Filters */}
                <div className="flex items-center gap-4">
                    <div className="relative flex-1">
                        <Search className="w-5 h-5 text-gray-400 dark:text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by path, method, or spec name..."
                            className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                        />
                    </div>
                    <select
                        value={specFilter}
                        onChange={(e) => setSpecFilter(e.target.value)}
                        className="px-4 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                    >
                        <option value="">All Specs</option>
                        {specs?.map((spec) => (
                            <option key={spec.id} value={spec.id}>
                                {spec.name}
                            </option>
                        ))}
                    </select>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 flex overflow-hidden">
                {/* Trace List */}
                <div className="w-1/2 border-r border-gray-200 dark:border-slate-800 overflow-y-auto">
                    {filteredTraces.length > 0 ? (
                        <div className="divide-y divide-gray-100 dark:divide-slate-800">
                            {filteredTraces.map((trace) => (
                                <div
                                    key={trace.id}
                                    onClick={() => setSelectedTrace(trace)}
                                    className={clsx(
                                        'p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors',
                                        selectedTrace?.id === trace.id && 'bg-primary-50 dark:bg-primary-900/30'
                                    )}
                                >
                                    <div className="flex items-center justify-between mb-2">
                                        <div className="flex items-center gap-2 min-w-0">
                                            <span className={clsx(
                                                'px-2 py-0.5 rounded text-xs font-bold uppercase shrink-0',
                                                methodColors[trace.request.method] || 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                            )}>
                                                {trace.request.method}
                                            </span>
                                            {trace.responseSource === 'proxy' && (
                                                <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300 shrink-0">
                                                    <Radio className="w-2.5 h-2.5" />
                                                    Proxy
                                                </span>
                                            )}
                                            {trace.responseSource === 'ai' && (
                                                <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/40 dark:text-fuchsia-300 shrink-0">
                                                    <Bot className="w-2.5 h-2.5" />
                                                    AI
                                                </span>
                                            )}
                                            {trace.responseTier && (
                                                <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 shrink-0">
                                                    {responseTierLabels[trace.responseTier] || trace.responseTier}
                                                </span>
                                            )}
                                        <span className="font-mono text-sm text-gray-900 dark:text-slate-100 truncate">
                                            {trace.request.path}
                                        </span>
                                        </div>
                                        <span className={clsx(
                                            'px-2 py-0.5 rounded text-xs font-medium shrink-0 ml-2',
                                            trace.response.statusCode >= 200 && trace.response.statusCode < 300
                                                ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                                : trace.response.statusCode >= 400
                                                    ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                                    : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                        )}>
                                            {trace.response.statusCode}
                                        </span>
                                    </div>
                                    <div className="flex items-center text-xs text-gray-500 dark:text-slate-400">
                                        <Clock className="w-3 h-3 mr-1" />
                                        {new Date(trace.timestamp).toLocaleTimeString()}
                                        <span className="mx-2">•</span>
                                        {formatDuration(trace.duration)}
                                        <span className="mx-2">•</span>
                                        {trace.specName}
                                        {trace.signature && (
                                            <>
                                                <span className="mx-2">•</span>
                                                <Fingerprint className="w-3 h-3 mr-1" />
                                                <span className="font-mono">{trace.signature.substring(0, 8)}</span>
                                            </>
                                        )}
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="p-12 text-center text-gray-500 dark:text-slate-400">
                            <Activity className="w-12 h-12 mx-auto mb-4 text-gray-300 dark:text-slate-600" />
                            <p>No traces yet</p>
                            <p className="text-sm mt-1 dark:text-slate-400">
                                Enable tracing on a spec to see requests
                            </p>
                        </div>
                    )}
                </div>

                {/* Trace Detail */}
                <div className="w-1/2 overflow-y-auto bg-gray-50 dark:bg-slate-950">
                    {selectedTrace ? (
                        <TraceDetail trace={selectedTrace} formatDuration={formatDuration} />
                    ) : (
                        <div className="h-full flex items-center justify-center text-gray-500 dark:text-slate-400">
                            <div className="text-center">
                                <ChevronRight className="w-12 h-12 mx-auto mb-4 text-gray-300 dark:text-slate-600" />
                                <p>Select a trace to view details</p>
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}

function SessionSection({ session }: { session: SessionTrace }) {
    return (
        <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center gap-2">
                <Users className="w-4 h-4 text-indigo-500" />
                Session
            </h3>
            <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3 flex items-center gap-4 text-sm flex-wrap">
                <div>
                    <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase mr-1">ID</span>
                    <span className="font-mono text-xs text-gray-700 dark:text-slate-200">{session.id}</span>
                </div>
                {session.isNew && (
                    <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                        New
                    </span>
                )}
                {session.storeAccess && session.storeAccess.length > 0 && (
                    <span className="text-xs text-gray-400 dark:text-slate-500">
                        {session.storeAccess.length} store op{session.storeAccess.length !== 1 ? 's' : ''}
                    </span>
                )}
            </div>
        </div>
    )
}

function ScriptsSection({ scripts }: { scripts: ScriptTrace[] }) {
    return (
        <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center gap-2">
                <Code2 className="w-4 h-4 text-amber-500" />
                Scripts
                <span className="text-xs font-normal text-gray-400 dark:text-slate-500 normal-case tracking-normal">
                    {scripts.length} binding{scripts.length !== 1 ? 's' : ''} executed
                </span>
            </h3>
            <div className="space-y-3">
                {scripts.map((st, i) => (
                    <div
                        key={st.bindingId || i}
                        className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden"
                    >
                        {/* Script header */}
                        <div className="px-4 py-3 border-b border-gray-100 dark:border-slate-800 flex items-center justify-between gap-3 flex-wrap">
                            <div className="flex items-center gap-2 min-w-0">
                                <span className="font-medium text-sm text-gray-900 dark:text-slate-100 truncate">{st.scriptName}</span>
                                <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-500 dark:text-slate-400 px-1.5 py-0.5 rounded font-mono shrink-0">
                                    .script.{st.outputKey}
                                </span>
                            </div>
                            <div className="flex items-center gap-2 text-xs shrink-0">
                                {st.error ? (
                                    <span className="text-red-600 dark:text-red-400 font-medium">Error</span>
                                ) : (
                                    <span className="text-emerald-600 dark:text-emerald-400 font-medium">OK</span>
                                )}
                                <span className="text-gray-400 dark:text-slate-500">{st.durationMs.toFixed(2)}ms</span>
                            </div>
                        </div>

                        {/* Error */}
                        {st.error && (
                            <div className="px-4 py-2 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/40 text-xs font-mono text-red-700 dark:text-red-300">
                                {st.error}
                            </div>
                        )}

                        {/* Logs */}
                        {st.logs && st.logs.length > 0 && (
                            <div className="border-b border-gray-100 dark:border-slate-800">
                                <div className="px-4 pt-3 pb-1 flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-slate-400 uppercase">
                                    <Terminal className="w-3.5 h-3.5" />
                                    Logs
                                </div>
                                <div className="px-4 pb-3 space-y-0.5">
                                    {st.logs.map((line, j) => (
                                        <div key={j} className="flex items-start gap-2 font-mono text-xs">
                                            <span className="select-none text-gray-400 dark:text-slate-600 shrink-0 w-5 text-right">{j + 1}</span>
                                            <span className="text-emerald-700 dark:text-emerald-300 break-all">{line}</span>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}

                        {/* Output */}
                        {st.output !== null && st.output !== undefined && (
                            <div className="px-4 py-3">
                                <div className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-1">Output</div>
                                <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-x-auto">
                                    {JSON.stringify(st.output, null, 2)}
                                </pre>
                            </div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    )
}

function tryFormatJson(str: string): string {
    try {
        return JSON.stringify(JSON.parse(str), null, 2)
    } catch {
        return str
    }
}

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)
    const handleCopy = () => {
        navigator.clipboard.writeText(text).then(() => {
            setCopied(true)
            setTimeout(() => setCopied(false), 1500)
        })
    }
    return (
        <button
            onClick={handleCopy}
            className="ml-2 p-1 rounded hover:bg-gray-200 dark:hover:bg-slate-700 transition-colors text-gray-400 hover:text-gray-700 dark:hover:text-slate-200"
            title="Copy to clipboard"
        >
            {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
        </button>
    )
}

function HeadersBlock({ headers }: { headers: Record<string, string[]> }) {
    const entries = Object.entries(headers)
    if (entries.length === 0) return null
    return (
        <div className="p-4 border-b border-gray-100 dark:border-slate-800">
            <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Headers</h4>
            <div className="font-mono text-xs space-y-1">
                {entries.map(([key, values]) => (
                    <div key={key}>
                        <span className="text-purple-600 dark:text-purple-400">{key}:</span>{' '}
                        <span className="text-gray-600 dark:text-slate-300">{values.join(', ')}</span>
                    </div>
                ))}
            </div>
        </div>
    )
}

function BodyBlock({ body }: { body: string }) {
    if (!body) return null
    return (
        <div className="p-4">
            <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Body</h4>
            <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-x-auto whitespace-pre-wrap break-all">
                {tryFormatJson(body)}
            </pre>
        </div>
    )
}

function TraceDetail({
    trace,
    formatDuration,
}: {
    trace: Trace
    formatDuration: (ns: number) => string
}) {
    return (
        <div className="p-6 space-y-6">
            {/* Generated/Proxy Banner */}
            {(trace.responseSource === 'proxy' || trace.responseSource === 'ai') && (
                <div className="rounded-lg border border-violet-200 dark:border-violet-800 bg-violet-50 dark:bg-violet-900/20 p-4">
                    <div className="flex items-center gap-2 mb-3">
                        {trace.responseSource === 'proxy' ? (
                            <Radio className="w-4 h-4 text-violet-600 dark:text-violet-400 shrink-0" />
                        ) : (
                            <Bot className="w-4 h-4 text-fuchsia-600 dark:text-fuchsia-400 shrink-0" />
                        )}
                        <span className="text-sm font-semibold text-violet-800 dark:text-violet-200">
                            {trace.responseSource === 'proxy' ? 'Proxy Recording' : 'AI Generation'}
                        </span>
                        <span className="ml-auto text-xs text-violet-500 dark:text-violet-400">
                            {formatDuration(trace.duration)}
                        </span>
                    </div>
                    <div className="space-y-2 text-sm">
                        {trace.backendUri && (
                            <div className="flex items-start gap-2">
                                <Globe className="w-3.5 h-3.5 text-violet-500 dark:text-violet-400 mt-0.5 shrink-0" />
                                <div className="min-w-0">
                                    <span className="text-xs text-violet-600 dark:text-violet-400 uppercase font-medium">Backend</span>
                                    <div className="flex items-center gap-1">
                                        <span className="font-mono text-xs text-violet-900 dark:text-violet-100 break-all">
                                            {trace.backendUri}
                                        </span>
                                        <CopyButton text={trace.backendUri} />
                                    </div>
                                </div>
                            </div>
                        )}
                        {/* Signature */}
                        {trace.signature && (
                            <div className="flex items-start gap-2">
                                <Fingerprint className="w-3.5 h-3.5 text-violet-500 dark:text-violet-400 mt-0.5 shrink-0" />
                                <div className="min-w-0">
                                    <span className="text-xs text-violet-600 dark:text-violet-400 uppercase font-medium">Request Signature</span>
                                    <div className="flex items-center gap-1">
                                        <span className="font-mono text-xs text-violet-900 dark:text-violet-100 break-all">
                                            {trace.signature}
                                        </span>
                                        <CopyButton text={trace.signature} />
                                    </div>
                                    <p className="text-xs text-violet-500 dark:text-violet-400 mt-0.5">
                                        This hash uniquely identifies the request and is used to match replayed responses.
                                    </p>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {(trace.aiSkippedReason || trace.proxySkippedReason || trace.mode || trace.responseTier) && (
                <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3">
                        Fallback decision
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                        {trace.mode && (
                            <div>
                                <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Selected mode</span>
                                <div className="text-gray-700 dark:text-slate-200 capitalize">{trace.mode}</div>
                            </div>
                        )}
                        {trace.responseTier && (
                            <div>
                                <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Response tier</span>
                                <div className="text-gray-700 dark:text-slate-200">{responseTierLabels[trace.responseTier] || trace.responseTier}</div>
                            </div>
                        )}
                        {trace.aiSkippedReason && (
                            <div>
                                <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">AI skip reason</span>
                                <div className="text-gray-700 dark:text-slate-200">{skipReasonLabels[trace.aiSkippedReason] || trace.aiSkippedReason}</div>
                            </div>
                        )}
                        {trace.proxySkippedReason && (
                            <div>
                                <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Proxy skip reason</span>
                                <div className="text-gray-700 dark:text-slate-200">{skipReasonLabels[trace.proxySkippedReason] || trace.proxySkippedReason}</div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* Request */}
            <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center">
                    <ArrowDownUp className="w-4 h-4 mr-2 text-blue-600" />
                    {trace.proxyMode ? 'Client Request → Backend' : 'Request'}
                </h3>
                <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden">
                    <div className="p-4 border-b border-gray-100 dark:border-slate-800 flex items-center gap-2 flex-wrap">
                        <span className={clsx(
                            'px-2 py-1 rounded text-sm font-bold uppercase',
                            methodColors[trace.request.method] || 'bg-gray-100 text-gray-700'
                        )}>
                            {trace.request.method}
                        </span>
                        <span className="font-mono text-sm text-gray-900 dark:text-slate-100 break-all">{trace.request.url}</span>
                    </div>
                    <HeadersBlock headers={trace.request.headers} />
                    <BodyBlock body={trace.request.body} />
                    {/* Query params */}
                    {Object.keys(trace.request.query ?? {}).length > 0 && (
                        <div className="p-4 border-t border-gray-100 dark:border-slate-800">
                            <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Query Parameters</h4>
                            <div className="font-mono text-xs space-y-1">
                                {Object.entries(trace.request.query).map(([key, values]) => (
                                    <div key={key}>
                                        <span className="text-blue-600 dark:text-blue-400">{key}:</span>{' '}
                                        <span className="text-gray-600 dark:text-slate-300">{values.join(', ')}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Response */}
            <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center">
                    <ArrowDownUp className="w-4 h-4 mr-2 text-green-600 rotate-180" />
                    {trace.proxyMode ? 'Backend Response → Client' : 'Response'}
                </h3>
                <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden">
                    <div className="p-4 border-b border-gray-100 dark:border-slate-800 flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <span className={clsx(
                                'px-2 py-1 rounded text-sm font-bold',
                                trace.response.statusCode >= 200 && trace.response.statusCode < 300
                                    ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                    : trace.response.statusCode >= 400
                                        ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                        : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                            )}>
                                {trace.response.statusCode}
                            </span>
                            {(trace.responseSource === 'proxy' || trace.responseSource === 'ai') && (
                                <span className="text-xs text-violet-600 dark:text-violet-400 font-medium">
                                    {trace.responseSource === 'proxy'
                                        ? 'Recorded & saved for replay'
                                        : 'AI-generated & saved for replay'}
                                </span>
                            )}
                        </div>
                        <span className="text-sm text-gray-500 dark:text-slate-400">
                            {formatDuration(trace.duration)}
                        </span>
                    </div>
                    <HeadersBlock headers={trace.response.headers} />
                    <BodyBlock body={trace.response.body} />
                </div>
            </div>

            {/* Matched Config (virtual mode) */}
            {trace.matchedConfig && trace.responseSource === 'config' && (
                <div className="text-sm text-gray-500 dark:text-slate-400 bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3">
                    Matched config: <span className="font-medium text-gray-700 dark:text-slate-200">{trace.matchedConfig}</span>
                    {trace.matchedConfigOrigin && (
                        <span className="ml-2 text-xs uppercase tracking-wide text-gray-400 dark:text-slate-500">
                            ({trace.matchedConfigOrigin})
                        </span>
                    )}
                </div>
            )}

            {/* Session info */}
            {trace.session && (
                <SessionSection session={trace.session} />
            )}

            {/* Script traces */}
            {trace.scripts && trace.scripts.length > 0 && (
                <ScriptsSection scripts={trace.scripts} />
            )}
        </div>
    )
}
