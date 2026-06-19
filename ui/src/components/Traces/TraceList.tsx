import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import {
    Activity, Bot, Trash2, Search, Clock,
    RefreshCw, Radio, Fingerprint,
} from 'lucide-react'
import clsx from 'clsx'
import { tracesApi, specsApi } from '../../services/api'
import type { Trace, Spec } from '../../types'

const methodColors: Record<string, string> = {
    GET:    'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST:   'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT:    'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH:  'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
}

const responseTierLabels: Record<string, string> = {
    configured: 'Configured',
    recorded:   'Recorded',
    fallback:   'Fallback',
}

function formatDuration(ns: number): string {
    const ms = ns / 1e6
    if (ms < 1)    return `${(ns / 1e3).toFixed(0)}µs`
    if (ms < 1000) return `${ms.toFixed(0)}ms`
    return `${(ms / 1000).toFixed(2)}s`
}

export default function TraceList() {
    const navigate     = useNavigate()
    const queryClient  = useQueryClient()
    const [isLive, setIsLive]         = useState(false)
    const [liveTraces, setLiveTraces] = useState<Trace[]>([])
    const [specFilter, setSpecFilter] = useState('')
    const [searchQuery, setSearchQuery] = useState('')
    const wsRef = useRef<WebSocket | null>(null)

    const { data: specs } = useQuery<Spec[]>({
        queryKey: ['specs'],
        queryFn: specsApi.list,
    })

    const { data: storedTraces, refetch: refetchStoredTraces } = useQuery<Trace[]>({
        queryKey: ['traces', specFilter],
        queryFn: () => tracesApi.list({ specId: specFilter || undefined }),
        enabled: !isLive,
    })

    const handleToggleLive = () => {
        if (isLive) {
            setIsLive(false)
            setTimeout(() => refetchStoredTraces(), 100)
        } else {
            setLiveTraces([])
            setIsLive(true)
        }
    }

    const clearMutation = useMutation({
        mutationFn: () => tracesApi.clear(specFilter || undefined),
        onSuccess: () => {
            setLiveTraces([])
            queryClient.invalidateQueries({ queryKey: ['traces'] })
        },
    })

    useEffect(() => {
        if (!isLive) {
            wsRef.current?.close()
            wsRef.current = null
            return
        }
        const ws = tracesApi.createStream()
        wsRef.current = ws
        ws.onmessage = (event) => {
            const trace = JSON.parse(event.data) as Trace
            setLiveTraces((prev) => [trace, ...prev].slice(0, 100))
        }
        ws.onclose = () => {
            if (isLive) {
                setTimeout(() => {
                    if (isLive && wsRef.current === ws) {
                        wsRef.current = tracesApi.createStream()
                    }
                }, 3000)
            }
        }
        return () => ws.close()
    }, [isLive])

    const traces = isLive ? liveTraces : (storedTraces || [])
    const filteredTraces = traces.filter((t) => {
        if (specFilter && t.specId !== specFilter) return false
        if (searchQuery) {
            const q = searchQuery.toLowerCase()
            return (
                t.request.path.toLowerCase().includes(q) ||
                t.request.method.toLowerCase().includes(q) ||
                t.specName.toLowerCase().includes(q)
            )
        }
        return true
    })

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="p-6 border-b border-gray-200 dark:border-slate-800 bg-white dark:bg-slate-900">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Request Traces</h1>
                        <p className="text-gray-500 dark:text-slate-400 mt-1">Monitor live requests and responses</p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        {!isLive && (
                            <button
                                onClick={() => refetchStoredTraces()}
                                className="flex items-center px-4 py-2 bg-gray-100 text-gray-600 dark:bg-slate-800 dark:text-slate-200 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-700 transition-colors"
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
                        >
                            <Activity className={clsx('w-5 h-5 mr-2', isLive && 'animate-pulse')} />
                            {isLive ? 'Live' : 'Go Live'}
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
                <div className="flex flex-wrap items-center gap-2">
                    <div className="relative flex-1 min-w-0">
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
                            <option key={spec.id} value={spec.id}>{spec.name}</option>
                        ))}
                    </select>
                </div>
            </div>

            {/* List */}
            <div className="flex-1 overflow-y-auto">
                {filteredTraces.length > 0 ? (
                    <div className="divide-y divide-gray-100 dark:divide-slate-800">
                        {filteredTraces.map((trace) => (
                            <div
                                key={trace.id}
                                onClick={() => navigate(`/traces/${trace.id}`)}
                                className="p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
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
                                <div className="flex items-center text-xs text-gray-500 dark:text-slate-400 flex-wrap gap-x-0">
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
                                    {(trace.pipeline?.length ?? 0) > 0 && (
                                        <>
                                            <span className="mx-2">•</span>
                                            <span>{trace.pipeline!.length} pipeline step{trace.pipeline!.length !== 1 ? 's' : ''}</span>
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
                        <p className="text-sm mt-1">Enable tracing on a spec to see requests</p>
                    </div>
                )}
            </div>
        </div>
    )
}
