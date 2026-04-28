import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
    Users,
    RefreshCw,
    Trash2,
    ChevronRight,
    X,
    Clock,
    AlertTriangle,
} from 'lucide-react';
import clsx from 'clsx';
import { sessionsApi } from '../../services/api';
import type { SessionInfo } from '../../types';

export default function SessionManager() {
    const queryClient = useQueryClient();

    const [selectedSession, setSelectedSession] = useState<SessionInfo | null>(null);
    const [showInvalidateAll, setShowInvalidateAll] = useState(false);

    const { data, isLoading, refetch, isFetching } = useQuery({
        queryKey: ['sessions'],
        queryFn: sessionsApi.list,
        refetchInterval: 30_000, // auto-refresh every 30s
    });

    const sessions = data?.sessions ?? [];
    const count = data?.count ?? 0;

    const invalidateMutation = useMutation({
        mutationFn: (id: string) => sessionsApi.invalidate(id),
        onSuccess: (_, id) => {
            queryClient.invalidateQueries({ queryKey: ['sessions'] });
            if (selectedSession?.id === id) {
                setSelectedSession(null);
            }
        },
    });

    const invalidateAllMutation = useMutation({
        mutationFn: sessionsApi.invalidateAll,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['sessions'] });
            setSelectedSession(null);
            setShowInvalidateAll(false);
        },
    });

    const { data: sessionDetail, isLoading: detailLoading } = useQuery({
        queryKey: ['sessions', selectedSession?.id],
        queryFn: () => sessionsApi.get(selectedSession!.id),
        enabled: !!selectedSession,
    });

    const formatTime = (iso: string) => new Date(iso).toLocaleString();
    const formatAge = (iso: string) => {
        const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
        if (secs < 60) return `${secs}s ago`;
        if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
        return `${Math.floor(secs / 3600)}h ago`;
    };

    const truncateID = (id: string) => id.slice(0, 8) + '…';

    return (
        <div className="p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
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

                <div className="flex flex-wrap items-center gap-2">
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

            <div className={clsx("flex gap-4", selectedSession ? "flex-col lg:flex-row" : "")}>
                {/* Session list */}
                <div className={clsx(
                    "min-w-0",
                    selectedSession ? "hidden lg:block lg:flex-1" : "flex-1"
                )}>
                    {isLoading ? (
                        <div className="text-center py-12 text-gray-500 dark:text-slate-400">Loading…</div>
                    ) : sessions.length === 0 ? (
                        <div className="text-center py-12 text-gray-500 dark:text-slate-400">
                            <Users className="w-10 h-10 mx-auto mb-3 opacity-30" />
                            No active sessions
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
                                    {sessions.map((s) => (
                                        <tr
                                            key={s.id}
                                            onClick={() =>
                                                setSelectedSession(
                                                    s.id === selectedSession?.id ? null : s
                                                )
                                            }
                                            className={`border-t border-gray-100 dark:border-slate-700 hover:bg-gray-50 dark:hover:bg-slate-700/30 transition-colors cursor-pointer ${selectedSession?.id === s.id
                                                    ? 'bg-teal-50 dark:bg-teal-900/20 border-l-2 border-l-teal-500'
                                                    : ''
                                                }`}
                                        >
                                            <td className="px-4 py-3">
                                                <div className="flex items-center gap-2">
                                                    <span className="font-mono text-teal-600 dark:text-teal-300 text-xs">
                                                        {truncateID(s.id)}
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
                        </div>
                    )}
                </div>

                {/* Detail panel */}
                {selectedSession && (
                    <div className="lg:w-80 lg:flex-shrink-0 flex-1 bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 h-fit">
                        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-slate-700">
                            <div className="flex items-center gap-2">
                                <button
                                    className="flex lg:hidden items-center gap-1 text-sm text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white"
                                    onClick={() => setSelectedSession(null)}
                                >
                                    ← Back
                                </button>
                                <h2 className="text-sm font-semibold text-gray-900 dark:text-white hidden lg:block">Session Detail</h2>
                            </div>
                            <button
                                onClick={() => setSelectedSession(null)}
                                className="text-gray-400 dark:text-slate-400 hover:text-gray-700 dark:hover:text-white"
                            >
                                <X className="w-4 h-4" />
                            </button>
                        </div>

                        <div className="p-4 space-y-3 text-sm">
                            <div>
                                <div className="text-gray-500 dark:text-slate-400 text-xs mb-1">Session ID</div>
                                <div className="font-mono text-teal-600 dark:text-teal-300 text-xs break-all">
                                    {selectedSession.id}
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-2 text-xs">
                                <div>
                                    <div className="text-gray-500 dark:text-slate-400 mb-0.5">Created</div>
                                    <div className="text-gray-700 dark:text-slate-300">
                                        {formatTime(selectedSession.createdAt)}
                                    </div>
                                </div>
                                <div>
                                    <div className="text-gray-500 dark:text-slate-400 mb-0.5">Last Active</div>
                                    <div className="text-gray-700 dark:text-slate-300">
                                        {formatTime(selectedSession.lastActive)}
                                    </div>
                                </div>
                            </div>

                            <div>
                                <div className="text-gray-500 dark:text-slate-400 text-xs mb-2">
                                    Store Snapshot ({selectedSession.entryCount} entries)
                                </div>
                                {detailLoading ? (
                                    <div className="text-gray-500 dark:text-slate-400 text-xs">Loading…</div>
                                ) : sessionDetail?.storeSnapshot &&
                                    Object.keys(sessionDetail.storeSnapshot).length > 0 ? (
                                    <div className="bg-gray-50 dark:bg-slate-900 rounded-lg p-2 overflow-auto max-h-48">
                                        <table className="w-full text-xs">
                                            <thead>
                                                <tr className="text-gray-400 dark:text-slate-500">
                                                    <th className="text-left pb-1">Key</th>
                                                    <th className="text-left pb-1">Value</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {Object.entries(sessionDetail.storeSnapshot).map(
                                                    ([k, v]) => (
                                                        <tr
                                                            key={k}
                                                            className="border-t border-gray-100 dark:border-slate-800"
                                                        >
                                                            <td className="py-1 pr-2 font-mono text-indigo-600 dark:text-indigo-300">
                                                                {k}
                                                            </td>
                                                            <td className="py-1 font-mono text-gray-700 dark:text-slate-300 truncate max-w-[120px]">
                                                                {JSON.stringify(v)}
                                                            </td>
                                                        </tr>
                                                    )
                                                )}
                                            </tbody>
                                        </table>
                                    </div>
                                ) : (
                                    <div className="text-gray-400 dark:text-slate-500 text-xs">No store entries</div>
                                )}
                            </div>

                            <button
                                onClick={() => invalidateMutation.mutate(selectedSession.id)}
                                disabled={invalidateMutation.isPending}
                                className="w-full flex items-center justify-center gap-2 px-3 py-2 bg-red-700/60 hover:bg-red-700 text-red-200 rounded-lg text-xs transition-colors"
                            >
                                <Trash2 className="w-3.5 h-3.5" />
                                Invalidate This Session
                            </button>
                        </div>
                    </div>
                )}
            </div>

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
