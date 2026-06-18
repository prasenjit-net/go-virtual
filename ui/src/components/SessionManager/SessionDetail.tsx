import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Clock, Trash2, RefreshCw, Database, KeyRound } from 'lucide-react';
import { sessionsApi } from '../../services/api';
import type { CollectionEvent } from '../../types';
import CollectionEventLog from './CollectionEventLog';

const CEVT_PREFIX = '__cevt__';

function formatTime(iso: string) {
    return new Date(iso).toLocaleString();
}
function formatAge(iso: string) {
    const secs = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
    if (secs < 60) return `${secs}s ago`;
    if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
    return `${Math.floor(secs / 3600)}h ago`;
}

function parseEvents(raw: unknown): CollectionEvent[] {
    if (!Array.isArray(raw)) return [];
    return raw as CollectionEvent[];
}

export default function SessionDetail() {
    const { sessionId } = useParams<{ sessionId: string }>();
    const navigate = useNavigate();
    const queryClient = useQueryClient();

    const { data: session, isLoading, refetch, isFetching } = useQuery({
        queryKey: ['sessions', sessionId],
        queryFn: () => sessionsApi.get(sessionId!),
        enabled: !!sessionId,
    });

    const invalidateMutation = useMutation({
        mutationFn: () => sessionsApi.invalidate(sessionId!),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['sessions'] });
            navigate('/sessions');
        },
    });

    if (isLoading) {
        return (
            <div className="p-8 text-center text-gray-500 dark:text-slate-400">Loading session…</div>
        );
    }

    if (!session) {
        return (
            <div className="p-8 text-center">
                <p className="text-gray-500 dark:text-slate-400 mb-4">Session not found or already expired.</p>
                <button
                    onClick={() => navigate('/sessions')}
                    className="text-teal-600 dark:text-teal-400 text-sm hover:underline"
                >
                    ← Back to sessions
                </button>
            </div>
        );
    }

    const snapshot = session.storeSnapshot ?? {};

    // Split snapshot into collection events and regular KV entries
    const collectionEntries: { name: string; events: CollectionEvent[] }[] = [];
    const storeEntries: { key: string; value: unknown }[] = [];

    for (const [k, v] of Object.entries(snapshot)) {
        if (k.startsWith(CEVT_PREFIX)) {
            collectionEntries.push({
                name: k.slice(CEVT_PREFIX.length),
                events: parseEvents(v),
            });
        } else {
            storeEntries.push({ key: k, value: v });
        }
    }

    return (
        <div className="p-6 max-w-4xl mx-auto">
            {/* Back nav */}
            <button
                onClick={() => navigate('/sessions')}
                className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white mb-6 transition-colors"
            >
                <ArrowLeft className="w-4 h-4" />
                Back to sessions
            </button>

            {/* Header */}
            <div className="flex items-start justify-between mb-6 gap-4 flex-wrap">
                <div>
                    <h1 className="text-xl font-bold text-gray-900 dark:text-white mb-1">
                        Session Detail
                    </h1>
                    <p className="font-mono text-xs text-teal-600 dark:text-teal-300 break-all">
                        {session.id}
                    </p>
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
                    <button
                        onClick={() => invalidateMutation.mutate()}
                        disabled={invalidateMutation.isPending}
                        className="flex items-center gap-2 px-3 py-2 bg-red-700/60 hover:bg-red-700 text-red-200 rounded-lg text-sm transition-colors"
                    >
                        <Trash2 className="w-4 h-4" />
                        {invalidateMutation.isPending ? 'Invalidating…' : 'Invalidate'}
                    </button>
                </div>
            </div>

            {/* Metadata cards */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
                {[
                    { label: 'Created', value: formatTime(session.createdAt) },
                    { label: 'Last Active', value: formatAge(session.lastActive) },
                    { label: 'Store Entries', value: storeEntries.length },
                    { label: 'Collection Events', value: collectionEntries.reduce((s, c) => s + c.events.length, 0) },
                ].map(({ label, value }) => (
                    <div key={label} className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 px-4 py-3">
                        <div className="text-xs text-gray-500 dark:text-slate-400 mb-0.5">{label}</div>
                        <div className="text-sm font-semibold text-gray-900 dark:text-white">{String(value)}</div>
                    </div>
                ))}
            </div>

            <div className="grid md:grid-cols-2 gap-6">
                {/* Regular store entries */}
                <section>
                    <div className="flex items-center gap-2 mb-3">
                        <KeyRound className="w-4 h-4 text-indigo-500" />
                        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                            Store Entries
                            {storeEntries.length > 0 && (
                                <span className="ml-1.5 text-xs font-normal bg-indigo-100 dark:bg-indigo-900/30 text-indigo-600 dark:text-indigo-300 px-1.5 py-0.5 rounded-full">
                                    {storeEntries.length}
                                </span>
                            )}
                        </h2>
                    </div>
                    {storeEntries.length === 0 ? (
                        <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 px-4 py-6 text-center text-xs text-gray-400 dark:text-slate-500">
                            No store entries
                        </div>
                    ) : (
                        <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
                            <table className="w-full text-xs">
                                <thead>
                                    <tr className="bg-gray-50 dark:bg-slate-700/50 text-gray-400 dark:text-slate-500 text-[10px] uppercase tracking-wide">
                                        <th className="px-3 py-2 text-left">Key</th>
                                        <th className="px-3 py-2 text-left">Value</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {storeEntries.map(({ key, value }) => (
                                        <tr key={key} className="border-t border-gray-100 dark:border-slate-700/60">
                                            <td className="px-3 py-2 font-mono text-indigo-600 dark:text-indigo-300 break-all">
                                                {key}
                                            </td>
                                            <td className="px-3 py-2 font-mono text-gray-700 dark:text-slate-300 break-all max-w-[160px]">
                                                {typeof value === 'string'
                                                    ? value
                                                    : JSON.stringify(value).slice(0, 80)}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </section>

                {/* Collection event logs */}
                <section>
                    <div className="flex items-center gap-2 mb-3">
                        <Database className="w-4 h-4 text-teal-500" />
                        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                            Collection Events
                            {collectionEntries.length > 0 && (
                                <span className="ml-1.5 text-xs font-normal bg-teal-100 dark:bg-teal-900/30 text-teal-600 dark:text-teal-300 px-1.5 py-0.5 rounded-full">
                                    {collectionEntries.length} collection{collectionEntries.length !== 1 ? 's' : ''}
                                </span>
                            )}
                        </h2>
                    </div>
                    {collectionEntries.length === 0 ? (
                        <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 px-4 py-6 text-center text-xs text-gray-400 dark:text-slate-500">
                            <Clock className="w-6 h-6 mx-auto mb-2 opacity-30" />
                            No collection mutations in this session
                        </div>
                    ) : (
                        <div className="space-y-3">
                            {collectionEntries.map(({ name, events }) => (
                                <CollectionEventLog key={name} collectionName={name} events={events} />
                            ))}
                        </div>
                    )}
                </section>
            </div>
        </div>
    );
}
