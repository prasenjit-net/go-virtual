import { useEffect, useState } from 'react'
import {
    Activity,
    FileCode2,
    GitBranch,
    Clock,
    AlertTriangle,
    TrendingUp,
    Zap,
    ExternalLink,
    CheckCircle2,
    XCircle,
    BarChart2,
} from 'lucide-react'
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    ResponsiveContainer,
    BarChart,
    Bar,
    Cell,
    Legend,
} from 'recharts'
import { statsApi } from '../services/api'
import type { GlobalStats, OperationStat } from '../types'

// ─── helper: colour a METHOD badge ────────────────────────────────────────────
function MethodBadge({ method }: { method: string }) {
    const colours: Record<string, string> = {
        GET: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
        POST: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
        PUT: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
        PATCH: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
        DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    }
    return (
        <span className={`px-2 py-0.5 rounded text-xs font-mono font-semibold ${colours[method] ?? 'bg-gray-100 text-gray-600'}`}>
            {method}
        </span>
    )
}

// ─── helper: error-rate colour ────────────────────────────────────────────────
function errorRateColour(rate: number) {
    if (rate < 1) return 'text-green-600 dark:text-green-400'
    if (rate < 5) return 'text-yellow-600 dark:text-yellow-400'
    return 'text-red-600 dark:text-red-400'
}

// ─── helper: response-time chart data ────────────────────────────────────────
function buildLatencyData(ops: OperationStat[]) {
    return ops.slice(0, 8).map((op) => ({
        label: `${op.method} ${op.path}`,
        avg: parseFloat(op.avgResponseTimeMs.toFixed(2)),
        min: parseFloat(op.minResponseTimeMs.toFixed(2)),
        max: parseFloat(op.maxResponseTimeMs.toFixed(2)),
    }))
}

// ─────────────────────────────────────────────────────────────────────────────

export default function Dashboard() {
    const [stats, setStats] = useState<GlobalStats | null>(null)
    const [error, setError] = useState<string | null>(null)

    useEffect(() => {
        const stream = statsApi.createStream()
        let hasReceivedStats = false

        const handleStats = (event: Event) => {
            const message = event as MessageEvent<string>
            try {
                const next = JSON.parse(message.data) as GlobalStats
                hasReceivedStats = true
                setStats(next)
                setError(null)
            } catch {
                setError('Failed to parse live statistics stream')
            }
        }

        const handleError = () => {
            if (!hasReceivedStats) {
                setError('Failed to connect to live statistics stream')
            }
        }

        stream.addEventListener('stats', handleStats)
        stream.onerror = handleError

        return () => {
            stream.removeEventListener('stats', handleStats)
            stream.close()
        }
    }, [])

    const isLoading = !stats && !error

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-6">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48" />
                    <div className="grid grid-cols-2 lg:grid-cols-5 gap-6">
                        {[1, 2, 3, 4, 5].map((i) => (
                            <div key={i} className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl" />
                        ))}
                    </div>
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <div className="h-72 bg-gray-200 dark:bg-slate-800 rounded-xl" />
                        <div className="h-72 bg-gray-200 dark:bg-slate-800 rounded-xl" />
                    </div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Failed to load statistics: {error}
                </div>
            </div>
        )
    }

    const errorRate = stats?.totalRequests
        ? (stats.totalErrors / stats.totalRequests) * 100
        : 0

    const statCards = [
        {
            label: 'Total Requests',
            value: stats?.totalRequests.toLocaleString() ?? '0',
            icon: Activity,
            color: 'text-blue-600',
            bgColor: 'bg-blue-50 dark:bg-blue-900/20',
            iconColor: 'text-blue-500',
        },
        {
            label: 'Active Specs',
            value: String(stats?.activeSpecs ?? 0),
            icon: FileCode2,
            color: 'text-emerald-600',
            bgColor: 'bg-emerald-50 dark:bg-emerald-900/20',
            iconColor: 'text-emerald-500',
        },
        {
            label: 'Total Operations',
            value: String(stats?.totalOperations ?? 0),
            icon: GitBranch,
            color: 'text-violet-600',
            bgColor: 'bg-violet-50 dark:bg-violet-900/20',
            iconColor: 'text-violet-500',
        },
        {
            label: 'Avg Response Time',
            value: `${stats?.avgResponseTimeMs.toFixed(1) ?? 0} ms`,
            icon: Clock,
            color: 'text-amber-600',
            bgColor: 'bg-amber-50 dark:bg-amber-900/20',
            iconColor: 'text-amber-500',
        },
        {
            label: 'Requests / sec',
            value: stats?.requestsPerSecond.toFixed(2) ?? '0.00',
            icon: Zap,
            color: 'text-sky-600',
            bgColor: 'bg-sky-50 dark:bg-sky-900/20',
            iconColor: 'text-sky-500',
        },
    ]

    const latencyData = buildLatencyData(stats?.topOperations ?? [])

    return (
        <div className="p-8 space-y-8">
            {/* ── Header ──────────────────────────────────────────────────── */}
            <div className="flex items-start justify-between">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Dashboard</h1>
                    <p className="text-gray-500 dark:text-slate-400 mt-1">
                        Monitor your API proxy performance and statistics
                    </p>
                </div>
                <a
                    href="/_prometheus"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium
                               bg-orange-50 text-orange-700 border border-orange-200
                               hover:bg-orange-100 transition-colors
                               dark:bg-orange-950/30 dark:text-orange-300 dark:border-orange-800/40 dark:hover:bg-orange-950/60"
                >
                    <BarChart2 className="w-4 h-4" />
                    Prometheus Metrics
                    <ExternalLink className="w-3 h-3 opacity-60" />
                </a>
            </div>

            {/* ── KPI Cards ───────────────────────────────────────────────── */}
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
                {statCards.map((card) => (
                    <div
                        key={card.label}
                        className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-5"
                    >
                        <div className={`inline-flex p-2 rounded-lg ${card.bgColor} mb-3`}>
                            <card.icon className={`w-5 h-5 ${card.iconColor}`} />
                        </div>
                        <p className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase tracking-wide">
                            {card.label}
                        </p>
                        <p className="text-2xl font-bold text-gray-900 dark:text-slate-100 mt-0.5">
                            {card.value}
                        </p>
                    </div>
                ))}
            </div>

            {/* ── Charts row 1 ────────────────────────────────────────────── */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Requests & errors over time */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">
                        Requests · Last 24 Hours
                    </h3>
                    <div className="h-56">
                        <ResponsiveContainer width="100%" height="100%">
                            <AreaChart data={stats?.requestsByHour ?? []} margin={{ top: 4, right: 4, left: -10, bottom: 0 }}>
                                <CartesianGrid strokeDasharray="3 3" stroke="currentColor" className="text-gray-100 dark:text-slate-800" />
                                <XAxis dataKey="hour" tick={{ fontSize: 11 }} />
                                <YAxis tick={{ fontSize: 11 }} />
                                <Tooltip />
                                <Legend iconType="circle" iconSize={8} />
                                <Area type="monotone" dataKey="requests" stroke="#3b82f6" fill="#bfdbfe" name="Requests" />
                                <Area type="monotone" dataKey="errors" stroke="#ef4444" fill="#fecaca" name="Errors" />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>
                </div>

                {/* Top operations by request count */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">
                        Top Operations · Request Volume
                    </h3>
                    <div className="h-56">
                        <ResponsiveContainer width="100%" height="100%">
                            <BarChart
                                data={(stats?.topOperations ?? []).slice(0, 6)}
                                layout="vertical"
                                margin={{ top: 4, right: 8, left: 0, bottom: 0 }}
                            >
                                <CartesianGrid strokeDasharray="3 3" stroke="currentColor" className="text-gray-100 dark:text-slate-800" />
                                <XAxis type="number" tick={{ fontSize: 11 }} />
                                <YAxis type="category" dataKey="path" tick={{ fontSize: 10 }} width={110} />
                                <Tooltip />
                                <Bar dataKey="totalRequests" name="Requests" radius={[0, 4, 4, 0]}>
                                    {(stats?.topOperations ?? []).slice(0, 6).map((_, i) => (
                                        <Cell key={i} fill={`hsl(${210 + i * 20},70%,55%)`} />
                                    ))}
                                </Bar>
                            </BarChart>
                        </ResponsiveContainer>
                    </div>
                </div>
            </div>

            {/* ── Charts row 2: latency ────────────────────────────────────── */}
            {latencyData.length > 0 && (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">
                        Response-Time Breakdown (ms) · Top Operations
                    </h3>
                    <div className="h-56">
                        <ResponsiveContainer width="100%" height="100%">
                            <BarChart data={latencyData} margin={{ top: 4, right: 8, left: -10, bottom: 40 }}>
                                <CartesianGrid strokeDasharray="3 3" stroke="currentColor" className="text-gray-100 dark:text-slate-800" />
                                <XAxis dataKey="label" tick={{ fontSize: 10 }} angle={-30} textAnchor="end" interval={0} />
                                <YAxis tick={{ fontSize: 11 }} unit=" ms" />
                                <Tooltip />
                                <Legend iconType="square" iconSize={10} />
                                <Bar dataKey="min" fill="#86efac" name="Min" />
                                <Bar dataKey="avg" fill="#3b82f6" name="Avg" />
                                <Bar dataKey="max" fill="#f87171" name="Max" />
                            </BarChart>
                        </ResponsiveContainer>
                    </div>
                </div>
            )}

            {/* ── Bottom row ───────────────────────────────────────────────── */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Server status */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4 flex items-center gap-2">
                        <TrendingUp className="w-4 h-4 text-emerald-600" />
                        Server Status
                    </h3>
                    <dl className="space-y-3">
                        {[
                            { label: 'Uptime', value: stats?.uptime },
                            { label: 'Start Time', value: stats?.startTime ? new Date(stats.startTime).toLocaleString() : '—' },
                            { label: 'Requests / sec', value: stats?.requestsPerSecond.toFixed(3) },
                            { label: 'Total Errors', value: stats?.totalErrors.toLocaleString(), danger: (stats?.totalErrors ?? 0) > 0 },
                        ].map((row) => (
                            <div key={row.label} className="flex justify-between items-center py-2 border-b border-gray-100 dark:border-slate-800 last:border-0">
                                <dt className="text-sm text-gray-500 dark:text-slate-400">{row.label}</dt>
                                <dd className={`text-sm font-semibold ${row.danger ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-slate-100'}`}>
                                    {row.value}
                                </dd>
                            </div>
                        ))}
                        {/* Error rate bar */}
                        <div className="pt-2">
                            <div className="flex justify-between text-sm mb-1">
                                <span className="text-gray-500 dark:text-slate-400">Error Rate</span>
                                <span className={`font-semibold ${errorRateColour(errorRate)}`}>
                                    {errorRate.toFixed(2)} %
                                </span>
                            </div>
                            <div className="h-2 rounded-full bg-gray-100 dark:bg-slate-800 overflow-hidden">
                                <div
                                    className={`h-full rounded-full transition-all ${errorRate < 1 ? 'bg-green-500' : errorRate < 5 ? 'bg-yellow-500' : 'bg-red-500'}`}
                                    style={{ width: `${Math.min(errorRate, 100)}%` }}
                                />
                            </div>
                        </div>
                    </dl>
                </div>

                {/* Recent errors */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4 flex items-center gap-2">
                        <AlertTriangle className="w-4 h-4 text-yellow-500" />
                        Recent Errors
                        {(stats?.recentErrors?.length ?? 0) > 0 && (
                            <span className="ml-auto text-xs font-normal bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300 px-2 py-0.5 rounded-full">
                                {stats!.recentErrors.length}
                            </span>
                        )}
                    </h3>
                    {stats?.recentErrors && stats.recentErrors.length > 0 ? (
                        <div className="space-y-2 max-h-72 overflow-y-auto pr-1">
                            {stats.recentErrors.map((err, i) => (
                                <div
                                    key={i}
                                    className="p-3 bg-red-50 dark:bg-red-950/30 rounded-lg border border-red-100 dark:border-red-900/40"
                                >
                                    <div className="flex items-center gap-2 mb-1">
                                        <XCircle className="w-3.5 h-3.5 text-red-500 shrink-0" />
                                        <MethodBadge method={err.method} />
                                        <span className="text-xs font-mono text-red-700 dark:text-red-300 truncate flex-1">
                                            {err.path}
                                        </span>
                                        <span className="text-xs text-red-400 dark:text-red-500 shrink-0">
                                            {err.statusCode}
                                        </span>
                                        <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0">
                                            {new Date(err.timestamp).toLocaleTimeString()}
                                        </span>
                                    </div>
                                    {err.error && (
                                        <p className="text-xs text-red-600 dark:text-red-400 ml-5 truncate">{err.error}</p>
                                    )}
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="flex flex-col items-center justify-center py-10 text-gray-400 dark:text-slate-500 gap-2">
                            <CheckCircle2 className="w-8 h-8 text-green-400" />
                            <span className="text-sm">No errors recorded</span>
                        </div>
                    )}
                </div>
            </div>

            {/* ── Operation stats table ────────────────────────────────────── */}
            {(stats?.topOperations ?? []).length > 0 && (
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">
                        Operation Statistics
                    </h3>
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="border-b border-gray-100 dark:border-slate-800 text-left">
                                    {['Method', 'Path', 'Requests', 'Errors', 'Avg ms', 'Min ms', 'Max ms', 'Last Hit'].map((h) => (
                                        <th key={h} className="pb-2 pr-4 text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wide whitespace-nowrap">
                                            {h}
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody>
                                {(stats?.topOperations ?? []).map((op) => {
                                    const opErr = op.totalRequests > 0 ? (op.totalErrors / op.totalRequests) * 100 : 0
                                    return (
                                        <tr key={op.operationId} className="border-b border-gray-50 dark:border-slate-800/60 hover:bg-gray-50 dark:hover:bg-slate-800/40">
                                            <td className="py-2 pr-4"><MethodBadge method={op.method} /></td>
                                            <td className="py-2 pr-4 font-mono text-xs text-gray-700 dark:text-slate-300">{op.path}</td>
                                            <td className="py-2 pr-4 font-medium text-gray-900 dark:text-slate-100">{op.totalRequests.toLocaleString()}</td>
                                            <td className={`py-2 pr-4 font-medium ${opErr > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-400 dark:text-slate-500'}`}>
                                                {op.totalErrors.toLocaleString()}
                                            </td>
                                            <td className="py-2 pr-4 text-gray-700 dark:text-slate-300">{op.avgResponseTimeMs.toFixed(1)}</td>
                                            <td className="py-2 pr-4 text-green-600 dark:text-green-400">{op.minResponseTimeMs.toFixed(1)}</td>
                                            <td className="py-2 pr-4 text-red-600 dark:text-red-400">{op.maxResponseTimeMs.toFixed(1)}</td>
                                            <td className="py-2 text-xs text-gray-400 dark:text-slate-500 whitespace-nowrap">
                                                {op.lastRequestTime ? new Date(op.lastRequestTime).toLocaleTimeString() : '—'}
                                            </td>
                                        </tr>
                                    )
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}
        </div>
    )
}
