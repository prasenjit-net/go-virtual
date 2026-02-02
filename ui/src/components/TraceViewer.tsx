import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
    Activity,
    Trash2,
    Search,
    ChevronRight,
    Clock,
    ArrowDownUp,
    RefreshCw
} from 'lucide-react'
import clsx from 'clsx'
import { tracesApi, specsApi } from '../services/api'
import type { Trace, Spec } from '../types'

const methodColors: Record<string, string> = {
    GET: 'bg-green-100 text-green-700',
    POST: 'bg-blue-100 text-blue-700',
    PUT: 'bg-yellow-100 text-yellow-700',
    DELETE: 'bg-red-100 text-red-700',
    PATCH: 'bg-purple-100 text-purple-700',
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

    const formatDuration = (ns: number) => {
        const ms = ns / 1e6
        if (ms < 1) return `${(ns / 1e3).toFixed(0)}µs`
        if (ms < 1000) return `${ms.toFixed(0)}ms`
        return `${(ms / 1000).toFixed(2)}s`
    }

    return (
        <div className="h-full flex flex-col">
            {/* Header */}
            <div className="p-6 border-b border-gray-200 bg-white">
                <div className="flex items-center justify-between mb-4">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900">Request Traces</h1>
                        <p className="text-gray-500 mt-1">
                            Monitor live requests and responses
                        </p>
                    </div>
                    <div className="flex items-center gap-3">
                        {!isLive && (
                            <button
                                onClick={() => refetchStoredTraces()}
                                className="flex items-center px-4 py-2 bg-gray-100 text-gray-600 rounded-lg hover:bg-gray-200 transition-colors"
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
                                    ? 'bg-green-100 text-green-700 hover:bg-green-200'
                                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
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
                            className="flex items-center px-4 py-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 transition-colors"
                        >
                            <Trash2 className="w-5 h-5 mr-2" />
                            Clear
                        </button>
                    </div>
                </div>

                {/* Filters */}
                <div className="flex items-center gap-4">
                    <div className="relative flex-1">
                        <Search className="w-5 h-5 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
                        <input
                            type="text"
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            placeholder="Search by path, method, or spec name..."
                            className="w-full pl-10 pr-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                        />
                    </div>
                    <select
                        value={specFilter}
                        onChange={(e) => setSpecFilter(e.target.value)}
                        className="px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
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
                <div className="w-1/2 border-r border-gray-200 overflow-y-auto">
                    {filteredTraces.length > 0 ? (
                        <div className="divide-y divide-gray-100">
                            {filteredTraces.map((trace) => (
                                <div
                                    key={trace.id}
                                    onClick={() => setSelectedTrace(trace)}
                                    className={clsx(
                                        'p-4 cursor-pointer hover:bg-gray-50 transition-colors',
                                        selectedTrace?.id === trace.id && 'bg-primary-50'
                                    )}
                                >
                                    <div className="flex items-center justify-between mb-2">
                                        <div className="flex items-center">
                                            <span className={clsx(
                                                'px-2 py-0.5 rounded text-xs font-bold uppercase',
                                                methodColors[trace.request.method] || 'bg-gray-100 text-gray-700'
                                            )}>
                                                {trace.request.method}
                                            </span>
                                            <span className="ml-2 font-mono text-sm text-gray-900 truncate max-w-[200px]">
                                                {trace.request.path}
                                            </span>
                                        </div>
                                        <span className={clsx(
                                            'px-2 py-0.5 rounded text-xs font-medium',
                                            trace.response.statusCode >= 200 && trace.response.statusCode < 300
                                                ? 'bg-green-100 text-green-700'
                                                : trace.response.statusCode >= 400
                                                    ? 'bg-red-100 text-red-700'
                                                    : 'bg-gray-100 text-gray-700'
                                        )}>
                                            {trace.response.statusCode}
                                        </span>
                                    </div>
                                    <div className="flex items-center text-xs text-gray-500">
                                        <Clock className="w-3 h-3 mr-1" />
                                        {new Date(trace.timestamp).toLocaleTimeString()}
                                        <span className="mx-2">•</span>
                                        {formatDuration(trace.duration)}
                                        <span className="mx-2">•</span>
                                        {trace.specName}
                                    </div>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="p-12 text-center text-gray-500">
                            <Activity className="w-12 h-12 mx-auto mb-4 text-gray-300" />
                            <p>No traces yet</p>
                            <p className="text-sm mt-1">
                                Enable tracing on a spec to see requests
                            </p>
                        </div>
                    )}
                </div>

                {/* Trace Detail */}
                <div className="w-1/2 overflow-y-auto bg-gray-50">
                    {selectedTrace ? (
                        <div className="p-6">
                            {/* Request */}
                            <div className="mb-6">
                                <h3 className="text-sm font-semibold text-gray-900 uppercase tracking-wider mb-3 flex items-center">
                                    <ArrowDownUp className="w-4 h-4 mr-2 text-blue-600" />
                                    Request
                                </h3>
                                <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
                                    <div className="p-4 border-b border-gray-100">
                                        <span className={clsx(
                                            'px-2 py-1 rounded text-sm font-bold uppercase mr-2',
                                            methodColors[selectedTrace.request.method]
                                        )}>
                                            {selectedTrace.request.method}
                                        </span>
                                        <span className="font-mono text-sm">{selectedTrace.request.url}</span>
                                    </div>
                                    {Object.keys(selectedTrace.request.headers).length > 0 && (
                                        <div className="p-4 border-b border-gray-100">
                                            <h4 className="text-xs font-medium text-gray-500 uppercase mb-2">Headers</h4>
                                            <div className="font-mono text-xs space-y-1">
                                                {Object.entries(selectedTrace.request.headers).map(([key, values]) => (
                                                    <div key={key}>
                                                        <span className="text-purple-600">{key}:</span>{' '}
                                                        <span className="text-gray-600">{values.join(', ')}</span>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                    {selectedTrace.request.body && (
                                        <div className="p-4">
                                            <h4 className="text-xs font-medium text-gray-500 uppercase mb-2">Body</h4>
                                            <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-x-auto">
                                                {tryFormatJson(selectedTrace.request.body)}
                                            </pre>
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Response */}
                            <div>
                                <h3 className="text-sm font-semibold text-gray-900 uppercase tracking-wider mb-3 flex items-center">
                                    <ArrowDownUp className="w-4 h-4 mr-2 text-green-600 rotate-180" />
                                    Response
                                </h3>
                                <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
                                    <div className="p-4 border-b border-gray-100 flex items-center justify-between">
                                        <span className={clsx(
                                            'px-2 py-1 rounded text-sm font-bold',
                                            selectedTrace.response.statusCode >= 200 && selectedTrace.response.statusCode < 300
                                                ? 'bg-green-100 text-green-700'
                                                : selectedTrace.response.statusCode >= 400
                                                    ? 'bg-red-100 text-red-700'
                                                    : 'bg-gray-100 text-gray-700'
                                        )}>
                                            {selectedTrace.response.statusCode}
                                        </span>
                                        <span className="text-sm text-gray-500">
                                            {formatDuration(selectedTrace.duration)}
                                        </span>
                                    </div>
                                    {Object.keys(selectedTrace.response.headers).length > 0 && (
                                        <div className="p-4 border-b border-gray-100">
                                            <h4 className="text-xs font-medium text-gray-500 uppercase mb-2">Headers</h4>
                                            <div className="font-mono text-xs space-y-1">
                                                {Object.entries(selectedTrace.response.headers).map(([key, values]) => (
                                                    <div key={key}>
                                                        <span className="text-purple-600">{key}:</span>{' '}
                                                        <span className="text-gray-600">{values.join(', ')}</span>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                    {selectedTrace.response.body && (
                                        <div className="p-4">
                                            <h4 className="text-xs font-medium text-gray-500 uppercase mb-2">Body</h4>
                                            <pre className="bg-gray-900 text-gray-100 rounded p-3 text-xs overflow-x-auto">
                                                {tryFormatJson(selectedTrace.response.body)}
                                            </pre>
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Matched Config */}
                            {selectedTrace.matchedConfig && (
                                <div className="mt-4 text-sm text-gray-500">
                                    Matched config: <span className="font-medium">{selectedTrace.matchedConfig}</span>
                                </div>
                            )}
                        </div>
                    ) : (
                        <div className="h-full flex items-center justify-center text-gray-500">
                            <div className="text-center">
                                <ChevronRight className="w-12 h-12 mx-auto mb-4 text-gray-300" />
                                <p>Select a trace to view details</p>
                            </div>
                        </div>
                    )}
                </div>
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
