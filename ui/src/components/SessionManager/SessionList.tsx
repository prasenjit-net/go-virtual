import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
    Users, RefreshCw, Trash2, ChevronRight,
    Clock, AlertTriangle, Search, X,
} from 'lucide-react';
import { sessionsApi } from '../../services/api';

export default function SessionList() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [search, setSearch] = useState('');
    const [showInvalidateAll, setShowInvalidateAll] = useState(false);

    const { data, isLoading, refetch, isFetching } = useQuery({
        queryKey: ['sessions'],
        queryFn: sessionsApi.list,
        refetchInterval: 30_000,
    });

    const sessions = data?.sessions ?? [];
    const count = data?.count ?? 0;

    const invalidateMutation = useMutation({
        mutationFn: (id: string) => sessionsApi.invalidate(id),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['sessions'] }),
    });

    const invalidateAllMutation = useMutation({
        mutationFn: sessionsApi.invalidateAll,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['sessions'] });
            setShowInvalidateAll(false);
        },
    });

    const formatAge = (iso: string) => {
        const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
        if (secs < 60) return `${secs}s ago`;
        if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
        return `${Math.floor(secs / 3600)}h ago`;
    };

    const filtered = search.trim()
        ? sessions.filter((s) => s.id.toLowerCase().includes(search.trim().toLowerCase()))
        : sessions;

    return (
        <div className="p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-6 flex-wrap gap-3">
                <div className="flex items-start gap-3">
                    <div className="rounded-lg bg-teal-100 p-3 dark:bg-teal-900/30">
                        <Users className="h-6 w-6 text-teal-600 dark:text-teal-400" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
                            Sessions
                            {count > 0 && (
                                <span className="ml-2 text-xs bg-teal-100 text-teal-700 dark:bg-teal-700/40 dark:text-teal-300 px-2 py-0.5 rounded-full">
                                    {count}
                                </span>
                            )}
                        </h1>
                        <p className="mt-1 text-sm text-gray-500 dark:text-slate-400">Active request sessions</p>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <button
                        onClick={() => refetch()}
                        disabled={isFetching}
                        className="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white transition-colors"
                        title="Refresh"
                    >
                        <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
                    </button>
                    {sessions.length > 0 && (
                        <button
                            onClick={() => setShowInvalidateAll(true)}
                            className="flex items-center gap-2 px-3 py-2 bg-red-700/60 hover:bg-red-700 text-red-200 rounded-lg text-sm transition-colors"
                        >
                            <Trash2 className="w-4 h-4" />
                            Invalidate All
                        </button>
                    )}
                </div>
            </div>

            {/* Search bar */}
            <div className="relative mb-4">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 dark:text-slate-500 pointer-events-none" />
                <input
                    type="text"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search by session ID…"
                    className="w-full pl-9 pr-9 py-2.5 rounded-lg border border-gray-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-sm text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-teal-500 focus:border-transparent"
                />
                {search && (
                    <button
                        onClick={() => setSearch('')}
                        className="absolute right-2.5 top-1/2 -translate-y-1/2 p-0.5 text-gray-400 hover:text-gray-700 dark:hover:text-white"
                    >
                        <X className="w-3.5 h-3.5" />
                    </button>
                )}
            </div>

            {/* Table */}
            {isLoading ? (
                <div className="text-center py-12 text-gray-500 dark:text-slate-400">Loading…</div>
            ) : filtered.length === 0 ? (
                <div className="text-center py-12 text-gray-500 dark:text-slate-400">
                    <Users className="w-10 h-10 mx-auto mb-3 opacity-30" />
                    {search ? `No sessions match "${search}"` : 'No active sessions'}
                </div>
            ) : (
                <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm">
                            <thead>
                                <tr className="bg-gray-50 dark:bg-slate-700/50 text-gray-500 dark:text-slate-400 text-xs uppercase tracking-wide">
                                    <th className="px-4 py-3 text-left">Session ID</th>
                                    <th className="px-4 py-3 text-left">Created</th>
                                    <th className="px-4 py-3 text-left hidden md:table-cell">Last Active</th>
                                    <th className="px-4 py-3 text-left">Entries</th>
                                    <th className="px-4 py-3 text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                {filtered.map((s) => (
                                    <tr
                                        key={s.id}
                                        onClick={() => navigate(`/sessions/${encodeURIComponent(s.id)}`)}
                                        className="border-t border-gray-100 dark:border-slate-700 hover:bg-gray-50 dark:hover:bg-slate-700/30 transition-colors cursor-pointer"
                                    >
                                        <td className="px-4 py-3">
                                            <div className="flex items-center gap-2">
                                                <span className="font-mono text-teal-600 dark:text-teal-300 text-xs">
                                                    {s.id.slice(0, 8)}
                                                    <span className="text-gray-400 dark:text-slate-500">…</span>
                                                </span>
                                                <ChevronRight className="w-3 h-3 text-gray-400 dark:text-slate-500" />
                                            </div>
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 dark:text-slate-400 text-xs">
                                            {formatAge(s.createdAt)}
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 dark:text-slate-400 text-xs hidden md:table-cell">
                                            <div className="flex items-center gap-1">
                                                <Clock className="w-3 h-3" />
                                                {formatAge(s.lastActive)}
                                            </div>
                                        </td>
                                        <td className="px-4 py-3 text-gray-700 dark:text-slate-300">
                                            {s.entryCount}
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <button
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    invalidateMutation.mutate(s.id);
                                                }}
                                                disabled={invalidateMutation.isPending}
                                                className="p-1.5 hover:bg-red-50 dark:hover:bg-red-900/30 rounded text-gray-500 dark:text-slate-400 hover:text-red-400 transition-colors"
                                                title="Invalidate"
                                            >
                                                <Trash2 className="w-3.5 h-3.5" />
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                    {search && filtered.length < sessions.length && (
                        <div className="px-4 py-2 text-xs text-gray-400 dark:text-slate-500 border-t border-gray-100 dark:border-slate-700">
                            Showing {filtered.length} of {sessions.length} sessions
                        </div>
                    )}
                </div>
            )}

            {/* Invalidate All confirmation */}
            {showInvalidateAll && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-800 rounded-xl border border-gray-200 dark:border-slate-700 w-full max-w-sm shadow-2xl p-6">
                        <div className="flex items-center gap-3 mb-3">
                            <AlertTriangle className="w-5 h-5 text-amber-400" />
                            <h2 className="text-gray-900 dark:text-white font-semibold">Invalidate All Sessions?</h2>
                        </div>
                        <p className="text-gray-500 dark:text-slate-400 text-sm mb-4">
                            This will remove all {count} active sessions. Clients will receive new
                            session IDs on their next request.
                        </p>
                        <div className="flex justify-end gap-2">
                            <button
                                onClick={() => setShowInvalidateAll(false)}
                                className="px-4 py-2 text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white text-sm"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => invalidateAllMutation.mutate()}
                                disabled={invalidateAllMutation.isPending}
                                className="px-4 py-2 bg-red-700 hover:bg-red-600 text-white rounded-lg text-sm"
                            >
                                {invalidateAllMutation.isPending ? 'Invalidating…' : 'Invalidate All'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
