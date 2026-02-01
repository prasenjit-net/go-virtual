import { useQuery } from '@tanstack/react-query'
import {
    Activity,
    FileCode2,
    GitBranch,
    Clock,
    AlertTriangle,
    TrendingUp
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
    Bar
} from 'recharts'
import { statsApi } from '../services/api'
import type { GlobalStats } from '../types'

export default function Dashboard() {
    const { data: stats, isLoading, error } = useQuery<GlobalStats>({
        queryKey: ['globalStats'],
        queryFn: statsApi.getGlobal,
        refetchInterval: 5000,
    })

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-6">
                    <div className="h-8 bg-gray-200 rounded w-48"></div>
                    <div className="grid grid-cols-4 gap-6">
                        {[1, 2, 3, 4].map((i) => (
                            <div key={i} className="h-32 bg-gray-200 rounded-xl"></div>
                        ))}
                    </div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
                    Failed to load statistics: {(error as Error).message}
                </div>
            </div>
        )
    }

    const statCards = [
        {
            label: 'Total Requests',
            value: stats?.totalRequests.toLocaleString() || '0',
            icon: Activity,
            color: 'text-blue-600',
            bgColor: 'bg-blue-100',
        },
        {
            label: 'Active Specs',
            value: stats?.activeSpecs || 0,
            icon: FileCode2,
            color: 'text-green-600',
            bgColor: 'bg-green-100',
        },
        {
            label: 'Total Operations',
            value: stats?.totalOperations || 0,
            icon: GitBranch,
            color: 'text-purple-600',
            bgColor: 'bg-purple-100',
        },
        {
            label: 'Avg Response Time',
            value: `${stats?.avgResponseTimeMs.toFixed(2) || 0} ms`,
            icon: Clock,
            color: 'text-orange-600',
            bgColor: 'bg-orange-100',
        },
    ]

    return (
        <div className="p-8">
            <div className="mb-8">
                <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
                <p className="text-gray-500 mt-1">
                    Monitor your API proxy performance and statistics
                </p>
            </div>

            {/* Stats Cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
                {statCards.map((stat) => (
                    <div
                        key={stat.label}
                        className="bg-white rounded-xl shadow-sm border border-gray-200 p-6"
                    >
                        <div className="flex items-center">
                            <div className={`p-3 rounded-lg ${stat.bgColor}`}>
                                <stat.icon className={`w-6 h-6 ${stat.color}`} />
                            </div>
                            <div className="ml-4">
                                <p className="text-sm font-medium text-gray-500">{stat.label}</p>
                                <p className="text-2xl font-bold text-gray-900">{stat.value}</p>
                            </div>
                        </div>
                    </div>
                ))}
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
                {/* Requests Chart */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4">
                        Requests (Last 24 Hours)
                    </h3>
                    <div className="h-64">
                        <ResponsiveContainer width="100%" height="100%">
                            <AreaChart data={stats?.requestsByHour || []}>
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis dataKey="hour" tick={{ fontSize: 12 }} />
                                <YAxis tick={{ fontSize: 12 }} />
                                <Tooltip />
                                <Area
                                    type="monotone"
                                    dataKey="requests"
                                    stroke="#3b82f6"
                                    fill="#93c5fd"
                                    name="Requests"
                                />
                                <Area
                                    type="monotone"
                                    dataKey="errors"
                                    stroke="#ef4444"
                                    fill="#fca5a5"
                                    name="Errors"
                                />
                            </AreaChart>
                        </ResponsiveContainer>
                    </div>
                </div>

                {/* Top Operations */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4">
                        Top Operations
                    </h3>
                    <div className="h-64">
                        <ResponsiveContainer width="100%" height="100%">
                            <BarChart
                                data={(stats?.topOperations || []).slice(0, 5)}
                                layout="vertical"
                            >
                                <CartesianGrid strokeDasharray="3 3" />
                                <XAxis type="number" tick={{ fontSize: 12 }} />
                                <YAxis
                                    type="category"
                                    dataKey="path"
                                    tick={{ fontSize: 10 }}
                                    width={120}
                                />
                                <Tooltip />
                                <Bar dataKey="totalRequests" fill="#3b82f6" name="Requests" />
                            </BarChart>
                        </ResponsiveContainer>
                    </div>
                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                {/* Server Info */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                        <TrendingUp className="w-5 h-5 mr-2 text-green-600" />
                        Server Status
                    </h3>
                    <div className="space-y-4">
                        <div className="flex justify-between items-center py-2 border-b border-gray-100">
                            <span className="text-gray-500">Uptime</span>
                            <span className="font-medium text-gray-900">{stats?.uptime}</span>
                        </div>
                        <div className="flex justify-between items-center py-2 border-b border-gray-100">
                            <span className="text-gray-500">Requests/sec</span>
                            <span className="font-medium text-gray-900">
                                {stats?.requestsPerSecond.toFixed(2)}
                            </span>
                        </div>
                        <div className="flex justify-between items-center py-2 border-b border-gray-100">
                            <span className="text-gray-500">Total Errors</span>
                            <span className="font-medium text-red-600">
                                {stats?.totalErrors.toLocaleString()}
                            </span>
                        </div>
                        <div className="flex justify-between items-center py-2">
                            <span className="text-gray-500">Error Rate</span>
                            <span className="font-medium text-gray-900">
                                {stats?.totalRequests
                                    ? ((stats.totalErrors / stats.totalRequests) * 100).toFixed(2)
                                    : 0}
                                %
                            </span>
                        </div>
                    </div>
                </div>

                {/* Recent Errors */}
                <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center">
                        <AlertTriangle className="w-5 h-5 mr-2 text-yellow-600" />
                        Recent Errors
                    </h3>
                    {stats?.recentErrors && stats.recentErrors.length > 0 ? (
                        <div className="space-y-3 max-h-64 overflow-y-auto">
                            {stats.recentErrors.slice(0, 5).map((error, i) => (
                                <div
                                    key={i}
                                    className="p-3 bg-red-50 rounded-lg border border-red-100"
                                >
                                    <div className="flex items-center justify-between mb-1">
                                        <span className="text-sm font-medium text-red-700">
                                            {error.method} {error.path}
                                        </span>
                                        <span className="text-xs text-red-500">
                                            {new Date(error.timestamp).toLocaleTimeString()}
                                        </span>
                                    </div>
                                    <p className="text-sm text-red-600">{error.error}</p>
                                </div>
                            ))}
                        </div>
                    ) : (
                        <div className="text-center py-8 text-gray-500">
                            No errors recorded
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
