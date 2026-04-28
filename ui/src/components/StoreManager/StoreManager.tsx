import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Plus, Trash2, Edit2, Database, AlertTriangle, Save, X } from 'lucide-react';
import Editor from '@monaco-editor/react';
import { storeApi } from '../../services/api';
import type { StoreEntry } from '../../types';

export default function StoreManager() {
    const queryClient = useQueryClient();

    const [search, setSearch] = useState('');
    const [editEntry, setEditEntry] = useState<StoreEntry | null>(null);
    const [newKey, setNewKey] = useState('');
    const [editorValue, setEditorValue] = useState('null');
    const [editorError, setEditorError] = useState('');
    const [showAddModal, setShowAddModal] = useState(false);
    const [showClearConfirm, setShowClearConfirm] = useState(false);

    const { data: entries = [], isLoading } = useQuery({
        queryKey: ['store'],
        queryFn: storeApi.list,
    });

    const upsertMutation = useMutation({
        mutationFn: ({ key, value }: { key: string; value: any }) =>
            storeApi.upsert(key, value),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['store'] });
            closeModal();
        },
    });

    const deleteMutation = useMutation({
        mutationFn: (key: string) => storeApi.delete(key),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['store'] }),
    });

    const clearMutation = useMutation({
        mutationFn: storeApi.clear,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['store'] });
            setShowClearConfirm(false);
        },
    });

    const openAdd = () => {
        setEditEntry(null);
        setNewKey('');
        setEditorValue('null');
        setEditorError('');
        setShowAddModal(true);
    };

    const openEdit = (entry: StoreEntry) => {
        setEditEntry(entry);
        setNewKey(entry.key);
        setEditorValue(JSON.stringify(entry.value, null, 2));
        setEditorError('');
        setShowAddModal(true);
    };

    const closeModal = () => {
        setShowAddModal(false);
        setEditEntry(null);
        setNewKey('');
        setEditorValue('null');
        setEditorError('');
    };

    const handleSave = () => {
        const key = editEntry ? editEntry.key : newKey.trim();
        if (!key) {
            setEditorError('Key must not be empty');
            return;
        }
        let parsed: any;
        try {
            parsed = JSON.parse(editorValue);
        } catch {
            setEditorError('Value must be valid JSON');
            return;
        }
        upsertMutation.mutate({ key, value: parsed });
    };

    const filtered = entries.filter((e) =>
        e.key.toLowerCase().includes(search.toLowerCase())
    );

    const truncateValue = (v: any): string => {
        const str = JSON.stringify(v);
        return str.length > 80 ? str.slice(0, 77) + '...' : str;
    };

    return (
        <div className="p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
                <div className="flex items-start gap-3">
                    <div className="rounded-lg bg-indigo-100 p-3 dark:bg-indigo-900/30">
                        <Database className="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Global Store</h1>
                        <p className="mt-1 text-sm text-gray-500 dark:text-slate-400">
                            Persistent key-value data seeded into every session
                        </p>
                    </div>
                </div>
                <button
                    onClick={openAdd}
                    className="flex items-center gap-2 px-3 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm transition-colors"
                >
                    <Plus className="w-4 h-4" />
                    Add Entry
                </button>
            </div>

            {/* Search */}
            <div className="mb-4">
                <input
                    type="text"
                    placeholder="Search by key…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="w-full px-3 py-2 bg-white dark:bg-slate-700 border border-gray-300 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-slate-400 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
            </div>

            {/* Entry table */}
            {isLoading ? (
                <div className="text-center py-12 text-gray-500 dark:text-slate-400">Loading…</div>
            ) : filtered.length === 0 ? (
                <div className="text-center py-12 text-gray-500 dark:text-slate-400">
                    <Database className="w-10 h-10 mx-auto mb-3 opacity-30" />
                    {entries.length === 0
                        ? 'No store entries. Add global data accessible to all scripts.'
                        : 'No entries match the search.'}
                </div>
            ) : (
                <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-slate-700 overflow-hidden">
                    <table className="w-full text-sm">
                        <thead>
                            <tr className="bg-gray-50 dark:bg-slate-700/50 text-gray-500 dark:text-slate-400 text-xs uppercase tracking-wide">
                                <th className="px-4 py-3 text-left w-1/4">Key</th>
                                <th className="px-4 py-3 text-left">Value</th>
                                <th className="px-4 py-3 text-left w-36">Updated</th>
                                <th className="px-4 py-3 text-right w-20">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.map((entry) => (
                                <tr
                                    key={entry.key}
                                    className="border-t border-gray-100 dark:border-slate-700 hover:bg-gray-50 dark:hover:bg-slate-700/30 transition-colors"
                                >
                                    <td className="px-4 py-3 font-mono text-indigo-600 dark:text-indigo-300 font-medium">
                                        {entry.key}
                                    </td>
                                    <td className="px-4 py-3 font-mono text-gray-700 dark:text-slate-300 truncate max-w-xs">
                                        {truncateValue(entry.value)}
                                    </td>
                                    <td className="px-4 py-3 text-gray-500 dark:text-slate-400 text-xs">
                                        {new Date(entry.updatedAt).toLocaleString()}
                                    </td>
                                    <td className="px-4 py-3 text-right">
                                        <div className="flex items-center justify-end gap-1">
                                            <button
                                                onClick={() => openEdit(entry)}
                                                className="p-1.5 hover:bg-gray-100 dark:hover:bg-slate-600 rounded text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white transition-colors"
                                                title="Edit"
                                            >
                                                <Edit2 className="w-3.5 h-3.5" />
                                            </button>
                                            <button
                                                onClick={() => deleteMutation.mutate(entry.key)}
                                                className="p-1.5 hover:bg-red-50 dark:hover:bg-red-900/30 rounded text-gray-500 dark:text-slate-400 hover:text-red-400 transition-colors"
                                                title="Delete"
                                            >
                                                <Trash2 className="w-3.5 h-3.5" />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Danger Zone */}
            {entries.length > 0 && (
                <details className="mt-8 group">
                    <summary className="cursor-pointer text-sm text-gray-500 dark:text-slate-400 flex items-center gap-2 select-none">
                        <AlertTriangle className="w-4 h-4 text-amber-400" />
                        Danger Zone
                    </summary>
                    <div className="mt-3 p-4 border border-red-200 dark:border-red-800/40 rounded-lg bg-red-50 dark:bg-red-900/10">
                        <p className="text-sm text-gray-700 dark:text-slate-300 mb-3">
                            Clearing all entries will remove every key from the global store.
                            Active sessions will retain their current snapshot until they expire.
                        </p>
                        <button
                            onClick={() => setShowClearConfirm(true)}
                            className="px-3 py-2 bg-red-700 hover:bg-red-600 text-white rounded-lg text-sm transition-colors"
                        >
                            Clear All Entries
                        </button>
                    </div>
                </details>
            )}

            {/* Add / Edit Modal */}
            {showAddModal && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-800 rounded-xl border border-gray-200 dark:border-slate-700 w-full max-w-lg shadow-2xl">
                        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-slate-700">
                            <h2 className="text-gray-900 dark:text-white font-semibold">
                                {editEntry ? `Edit "${editEntry.key}"` : 'Add Store Entry'}
                            </h2>
                            <button onClick={closeModal} className="text-gray-400 dark:text-slate-400 hover:text-gray-700 dark:hover:text-white">
                                <X className="w-5 h-5" />
                            </button>
                        </div>

                        <div className="p-5 space-y-4">
                            {/* Key */}
                            <div>
                                <label className="block text-sm text-gray-500 dark:text-slate-400 mb-1">Key</label>
                                <input
                                    type="text"
                                    value={newKey}
                                    onChange={(e) => setNewKey(e.target.value)}
                                    disabled={!!editEntry}
                                    placeholder="e.g. id-counter"
                                    className="w-full px-3 py-2 bg-white dark:bg-slate-700 border border-gray-300 dark:border-slate-600 rounded-lg text-gray-900 dark:text-white text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed font-mono"
                                />
                            </div>

                            {/* Value — Monaco JSON editor */}
                            <div>
                                <label className="block text-sm text-gray-500 dark:text-slate-400 mb-1">
                                    Value{' '}
                                    <span className="text-xs text-gray-400 dark:text-slate-500">(any valid JSON)</span>
                                </label>
                                <div className="rounded-lg overflow-hidden border border-gray-300 dark:border-slate-600">
                                    <Editor
                                        height="180px"
                                        language="json"
                                        value={editorValue}
                                        onChange={(v) => {
                                            setEditorValue(v ?? '');
                                            setEditorError('');
                                        }}
                                        theme="vs-dark"
                                        options={{
                                            minimap: { enabled: false },
                                            lineNumbers: 'off',
                                            fontSize: 13,
                                            scrollBeyondLastLine: false,
                                            wordWrap: 'on',
                                        }}
                                    />
                                </div>
                                {editorError && (
                                    <p className="text-red-600 dark:text-red-400 text-xs mt-1">{editorError}</p>
                                )}
                            </div>
                        </div>

                        <div className="flex justify-end gap-2 px-5 py-4 border-t border-gray-200 dark:border-slate-700">
                            <button
                                onClick={closeModal}
                                className="px-4 py-2 text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white text-sm transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleSave}
                                disabled={upsertMutation.isPending}
                                className="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg text-sm transition-colors disabled:opacity-50"
                            >
                                <Save className="w-4 h-4" />
                                {upsertMutation.isPending ? 'Saving…' : 'Save'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Clear confirmation */}
            {showClearConfirm && (
                <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
                    <div className="bg-white dark:bg-slate-800 rounded-xl border border-gray-200 dark:border-slate-700 w-full max-w-sm shadow-2xl p-6">
                        <h2 className="text-gray-900 dark:text-white font-semibold mb-2">Clear All Entries?</h2>
                        <p className="text-gray-500 dark:text-slate-400 text-sm mb-4">
                            This will permanently remove all {entries.length} entries from the global
                            store. This cannot be undone.
                        </p>
                        <div className="flex justify-end gap-2">
                            <button
                                onClick={() => setShowClearConfirm(false)}
                                className="px-4 py-2 text-gray-600 dark:text-slate-300 hover:text-gray-900 dark:hover:text-white text-sm"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => clearMutation.mutate()}
                                disabled={clearMutation.isPending}
                                className="px-4 py-2 bg-red-700 hover:bg-red-600 text-white rounded-lg text-sm"
                            >
                                {clearMutation.isPending ? 'Clearing…' : 'Clear All'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
