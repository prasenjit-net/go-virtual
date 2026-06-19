import { Database, Plus, RefreshCw, Trash2, Layers } from 'lucide-react';
import type { CollectionEvent } from '../../types';

const OP_META: Record<string, { label: string; icon: React.ReactNode; color: string }> = {
    insert:  { label: 'INSERT',  icon: <Plus  className="w-3 h-3" />, color: 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/30 border-emerald-200 dark:border-emerald-700' },
    update:  { label: 'UPDATE',  icon: <RefreshCw className="w-3 h-3" />, color: 'text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-700' },
    upsert:  { label: 'UPSERT',  icon: <Layers className="w-3 h-3" />, color: 'text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-900/30 border-violet-200 dark:border-violet-700' },
    delete:  { label: 'DELETE',  icon: <Trash2 className="w-3 h-3" />, color: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/30 border-red-200 dark:border-red-700' },
    clear:   { label: 'CLEAR',   icon: <Trash2 className="w-3 h-3" />, color: 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/30 border-amber-200 dark:border-amber-700' },
};

function EventBadge({ op }: { op: string }) {
    const meta = OP_META[op] ?? { label: op.toUpperCase(), icon: null, color: 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-slate-700 border-gray-200 dark:border-slate-600' };
    return (
        <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded border text-[10px] font-semibold font-mono ${meta.color}`}>
            {meta.icon}
            {meta.label}
        </span>
    );
}

function JsonInline({ value }: { value: unknown }) {
    if (value === undefined || value === null) return null;
    const str = JSON.stringify(value);
    return (
        <span className="font-mono text-[10px] text-gray-600 dark:text-slate-300 bg-gray-100 dark:bg-slate-700/60 px-1.5 py-0.5 rounded break-all">
            {str.length > 120 ? str.slice(0, 120) + '…' : str}
        </span>
    );
}

interface Props {
    collectionName: string;
    events: CollectionEvent[];
}

export default function CollectionEventLog({ collectionName, events }: Props) {
    return (
        <div className="rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
            {/* Header */}
            <div className="flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-slate-700/50 border-b border-gray-200 dark:border-slate-700">
                <Database className="w-3.5 h-3.5 text-teal-500" />
                <span className="font-mono text-xs font-semibold text-teal-600 dark:text-teal-300">
                    {collectionName}
                </span>
                <span className="ml-auto text-[10px] text-gray-400 dark:text-slate-500">
                    {events.length} event{events.length !== 1 ? 's' : ''}
                </span>
            </div>

            {events.length === 0 ? (
                <div className="px-3 py-4 text-center text-[11px] text-gray-400 dark:text-slate-500">
                    No events
                </div>
            ) : (
                <ol className="relative">
                    {events.map((evt, i) => (
                        <li key={i} className="flex gap-3 px-3 py-2 border-b border-gray-100 dark:border-slate-700/60 last:border-0">
                            {/* Timeline dot + line */}
                            <div className="flex flex-col items-center">
                                <div className="mt-0.5 w-2 h-2 rounded-full bg-gray-300 dark:bg-slate-500 flex-shrink-0" />
                                {i < events.length - 1 && (
                                    <div className="w-px flex-1 bg-gray-200 dark:bg-slate-700 mt-1" />
                                )}
                            </div>

                            {/* Event content */}
                            <div className="flex-1 min-w-0 pb-1">
                                <div className="flex items-center gap-2 flex-wrap">
                                    <span className="text-[10px] text-gray-400 dark:text-slate-500 font-mono">
                                        #{i + 1}
                                    </span>
                                    <EventBadge op={evt.op} />
                                </div>

                                {evt.filter && Object.keys(evt.filter).length > 0 && (
                                    <div className="mt-1 flex items-start gap-1.5">
                                        <span className="text-[10px] text-gray-400 dark:text-slate-500 w-10 flex-shrink-0">
                                            filter
                                        </span>
                                        <JsonInline value={evt.filter} />
                                    </div>
                                )}

                                {evt.data && Object.keys(evt.data).length > 0 && (
                                    <div className="mt-1 flex items-start gap-1.5">
                                        <span className="text-[10px] text-gray-400 dark:text-slate-500 w-10 flex-shrink-0">
                                            data
                                        </span>
                                        <JsonInline value={evt.data} />
                                    </div>
                                )}
                            </div>
                        </li>
                    ))}
                </ol>
            )}
        </div>
    );
}
