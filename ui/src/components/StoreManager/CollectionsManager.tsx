import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, Plus, Trash2, Edit2, Database, Save, X, AlertTriangle } from 'lucide-react';
import Editor from '@monaco-editor/react';
import { collectionsApi } from '../../services/api';
import type { CollectionDocument, CollectionInfo } from '../../types';

export default function CollectionsManager() {
    const queryClient = useQueryClient();
    const [expanded, setExpanded] = useState<string | null>(null);
    const [editState, setEditState] = useState<{ name: string; index: number; value: string } | null>(null);
    const [addState, setAddState] = useState<{ name: string; value: string } | null>(null);
    const [clearConfirm, setClearConfirm] = useState<string | null>(null);
    const [editorError, setEditorError] = useState('');
    // new collection creation state
    const [newColName, setNewColName] = useState('');
    const [showNewCol, setShowNewCol] = useState(false);

    const { data: collections = [], isLoading } = useQuery({
        queryKey: ['collections'],
        queryFn: collectionsApi.list,
    });

    const { data: docs = [], isLoading: docsLoading } = useQuery({
        queryKey: ['collection-docs', expanded],
        queryFn: () => expanded ? collectionsApi.get(expanded) : Promise.resolve([]),
        enabled: !!expanded,
    });

    const insertMutation = useMutation({
        mutationFn: ({ name, doc }: { name: string; doc: CollectionDocument }) =>
            collectionsApi.insert(name, doc),
        onSuccess: (_, { name }) => {
            queryClient.invalidateQueries({ queryKey: ['collections'] });
            queryClient.invalidateQueries({ queryKey: ['collection-docs', name] });
            setAddState(null);
            setEditorError('');
        },
        onError: (err: Error) => setEditorError(err.message),
    });

    const updateMutation = useMutation({
        mutationFn: ({ name, index, changes }: { name: string; index: number; changes: CollectionDocument }) =>
            collectionsApi.update(name, index, changes),
        onSuccess: (_, { name }) => {
            queryClient.invalidateQueries({ queryKey: ['collection-docs', name] });
            setEditState(null);
            setEditorError('');
        },
        onError: (err: Error) => setEditorError(err.message),
    });

    const deleteMutation = useMutation({
        mutationFn: ({ name, index }: { name: string; index: number }) =>
            collectionsApi.deleteDoc(name, index),
        onSuccess: (_, { name }) => {
            queryClient.invalidateQueries({ queryKey: ['collections'] });
            queryClient.invalidateQueries({ queryKey: ['collection-docs', name] });
        },
    });

    const clearMutation = useMutation({
        mutationFn: (name: string) => collectionsApi.clear(name),
        onSuccess: (_, name) => {
            queryClient.invalidateQueries({ queryKey: ['collections'] });
            queryClient.invalidateQueries({ queryKey: ['collection-docs', name] });
            setClearConfirm(null);
        },
    });

    const toggleExpand = (name: string) => {
        setExpanded(prev => prev === name ? null : name);
        setEditState(null);
        setAddState(null);
        setEditorError('');
    };

    const startAdd = (name: string) => {
        setAddState({ name, value: '{}' });
        setEditState(null);
        setEditorError('');
    };

    const startEdit = (name: string, index: number, doc: CollectionDocument) => {
        setEditState({ name, index, value: JSON.stringify(doc, null, 2) });
        setAddState(null);
        setEditorError('');
    };

    const saveEdit = () => {
        if (!editState) return;
        let parsed: CollectionDocument;
        try {
            parsed = JSON.parse(editState.value);
        } catch {
            setEditorError('Invalid JSON');
            return;
        }
        updateMutation.mutate({ name: editState.name, index: editState.index, changes: parsed });
    };

    const saveAdd = () => {
        if (!addState) return;
        let parsed: CollectionDocument;
        try {
            parsed = JSON.parse(addState.value);
        } catch {
            setEditorError('Invalid JSON');
            return;
        }
        insertMutation.mutate({ name: addState.name, doc: parsed });
    };

    // Creates a new collection by inserting the first document into it
    const createCollection = () => {
        const name = newColName.trim();
        if (!name) return;
        setNewColName('');
        setShowNewCol(false);
        // Expand and open add form for the new collection name
        setExpanded(name);
        setAddState({ name, value: '{}' });
        setEditorError('');
        // Invalidate so the new collection appears after insert
        queryClient.invalidateQueries({ queryKey: ['collections'] });
    };

    if (isLoading) {
        return <div className="text-sm text-gray-500 p-4">Loading collections…</div>;
    }

    return (
        <div className="space-y-2">
            {/* Header: new collection button */}
            <div className="flex items-center justify-between mb-2">
                <span className="text-xs text-gray-500">
                    {collections.length === 0 ? 'No collections yet' : `${collections.length} collection${collections.length !== 1 ? 's' : ''}`}
                </span>
                <button
                    onClick={() => setShowNewCol(v => !v)}
                    className="flex items-center gap-1 text-xs px-3 py-1.5 bg-primary-600 text-white rounded hover:bg-primary-700 transition-colors"
                >
                    <Plus className="h-3 w-3" /> New Collection
                </button>
            </div>

            {/* New collection input */}
            {showNewCol && (
                <div className="flex items-center gap-2 p-3 bg-blue-50 border border-blue-200 rounded-lg">
                    <Database className="h-4 w-4 text-blue-500 flex-shrink-0" />
                    <input
                        autoFocus
                        type="text"
                        placeholder="collection name"
                        value={newColName}
                        onChange={e => setNewColName(e.target.value)}
                        onKeyDown={e => { if (e.key === 'Enter') createCollection(); if (e.key === 'Escape') { setShowNewCol(false); setNewColName(''); } }}
                        className="flex-1 text-sm border border-blue-300 rounded px-2 py-1 bg-white font-mono focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                    <button
                        onClick={createCollection}
                        disabled={!newColName.trim()}
                        className="text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-40"
                    >Create</button>
                    <button
                        onClick={() => { setShowNewCol(false); setNewColName(''); }}
                        className="text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50"
                    >Cancel</button>
                </div>
            )}

            {collections.length === 0 && !showNewCol && (
                <div className="text-center py-10 text-gray-500">
                    <Database className="h-10 w-10 mx-auto mb-3 opacity-30" />
                    <p className="text-sm">No collections yet.</p>
                    <p className="text-xs mt-1 text-gray-400">Create one above, or use <code className="bg-gray-100 px-1 rounded">store.collection("name")</code> in a script.</p>
                </div>
            )}

            {/* Add form for a not-yet-existing collection (created via New Collection button) */}
            {addState && !collections.find((c: CollectionInfo) => c.name === addState.name) && (
                <div className="border border-blue-300 rounded-lg overflow-hidden">
                    <div className="flex items-center gap-2 px-4 py-3 bg-blue-50">
                        <Database className="h-4 w-4 text-blue-500 flex-shrink-0" />
                        <span className="font-mono text-sm font-medium flex-1">{addState.name}</span>
                        <span className="text-xs text-blue-400 italic">new</span>
                    </div>
                    <div className="p-4 bg-blue-50 space-y-2">
                        <p className="text-xs font-medium text-blue-700">First document</p>
                        <div className="h-40 border border-blue-300 rounded overflow-hidden">
                            <Editor
                                defaultLanguage="json"
                                value={addState.value}
                                onChange={v => setAddState(prev => prev ? { ...prev, value: v ?? '' } : null)}
                                options={{ minimap: { enabled: false }, fontSize: 12, lineNumbers: 'off', scrollBeyondLastLine: false }}
                                theme="light"
                            />
                        </div>
                        {editorError && <p className="text-xs text-red-600">{editorError}</p>}
                        <div className="flex gap-2">
                            <button onClick={saveAdd} className="flex items-center gap-1 text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700">
                                <Plus className="h-3 w-3" /> Insert
                            </button>
                            <button onClick={() => { setAddState(null); setEditorError(''); }} className="flex items-center gap-1 text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50">
                                <X className="h-3 w-3" /> Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {collections.map((col: CollectionInfo) => (
                <div key={col.name} className="border border-gray-200 rounded-lg overflow-hidden">
                    {/* Collection header */}
                    <div
                        className="flex items-center gap-2 px-4 py-3 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
                        onClick={() => toggleExpand(col.name)}
                    >
                        {expanded === col.name
                            ? <ChevronDown className="h-4 w-4 text-gray-400 flex-shrink-0" />
                            : <ChevronRight className="h-4 w-4 text-gray-400 flex-shrink-0" />}
                        <Database className="h-4 w-4 text-blue-500 flex-shrink-0" />
                        <span className="font-mono text-sm font-medium flex-1">{col.name}</span>
                        <span className="text-xs text-gray-400 bg-gray-200 px-2 py-0.5 rounded-full">{col.count} doc{col.count !== 1 ? 's' : ''}</span>
                        <button
                            className="p-1 text-gray-400 hover:text-blue-600 rounded"
                            title="Add document"
                            onClick={e => { e.stopPropagation(); if (expanded !== col.name) toggleExpand(col.name); startAdd(col.name); }}
                        >
                            <Plus className="h-4 w-4" />
                        </button>
                        <button
                            className="p-1 text-gray-400 hover:text-red-500 rounded"
                            title="Clear collection"
                            onClick={e => { e.stopPropagation(); setClearConfirm(col.name); }}
                        >
                            <Trash2 className="h-4 w-4" />
                        </button>
                    </div>

                    {/* Clear confirm */}
                    {clearConfirm === col.name && (
                        <div className="bg-red-50 border-t border-red-200 px-4 py-3 flex items-center gap-3">
                            <AlertTriangle className="h-4 w-4 text-red-500 flex-shrink-0" />
                            <span className="text-sm text-red-700 flex-1">Clear all {col.count} documents?</span>
                            <button
                                className="text-xs px-3 py-1.5 bg-red-600 text-white rounded hover:bg-red-700"
                                onClick={() => clearMutation.mutate(col.name)}
                            >Clear</button>
                            <button
                                className="text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50"
                                onClick={() => setClearConfirm(null)}
                            >Cancel</button>
                        </div>
                    )}

                    {/* Expanded documents */}
                    {expanded === col.name && (
                        <div className="divide-y divide-gray-100">
                            {docsLoading && (
                                <div className="px-4 py-3 text-sm text-gray-500">Loading…</div>
                            )}

                            {!docsLoading && docs.map((doc: CollectionDocument, idx: number) => (
                                <div key={idx} className="p-4">
                                    {editState?.name === col.name && editState.index === idx ? (
                                        /* Edit mode */
                                        <div className="space-y-2">
                                            <div className="h-40 border border-gray-300 rounded overflow-hidden">
                                                <Editor
                                                    defaultLanguage="json"
                                                    value={editState.value}
                                                    onChange={v => setEditState(prev => prev ? { ...prev, value: v ?? '' } : null)}
                                                    options={{ minimap: { enabled: false }, fontSize: 12, lineNumbers: 'off', scrollBeyondLastLine: false }}
                                                    theme="light"
                                                />
                                            </div>
                                            {editorError && <p className="text-xs text-red-600">{editorError}</p>}
                                            <div className="flex gap-2">
                                                <button onClick={saveEdit} className="flex items-center gap-1 text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700">
                                                    <Save className="h-3 w-3" /> Save
                                                </button>
                                                <button onClick={() => { setEditState(null); setEditorError(''); }} className="flex items-center gap-1 text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50">
                                                    <X className="h-3 w-3" /> Cancel
                                                </button>
                                            </div>
                                        </div>
                                    ) : (
                                        /* View mode */
                                        <div className="flex items-start gap-3">
                                            <span className="text-xs text-gray-400 pt-0.5 w-6 flex-shrink-0 text-right">#{idx}</span>
                                            <pre className="text-xs text-gray-700 flex-1 whitespace-pre-wrap break-all bg-gray-50 rounded p-2 font-mono">
                                                {JSON.stringify(doc, null, 2)}
                                            </pre>
                                            <div className="flex gap-1 flex-shrink-0">
                                                <button
                                                    onClick={() => startEdit(col.name, idx, doc)}
                                                    className="p-1 text-gray-400 hover:text-blue-600 rounded"
                                                    title="Edit"
                                                >
                                                    <Edit2 className="h-3.5 w-3.5" />
                                                </button>
                                                <button
                                                    onClick={() => deleteMutation.mutate({ name: col.name, index: idx })}
                                                    className="p-1 text-gray-400 hover:text-red-500 rounded"
                                                    title="Delete"
                                                >
                                                    <Trash2 className="h-3.5 w-3.5" />
                                                </button>
                                            </div>
                                        </div>
                                    )}
                                </div>
                            ))}

                            {!docsLoading && docs.length === 0 && (
                                <div className="px-4 py-3 text-sm text-gray-400 italic">No documents yet.</div>
                            )}

                            {/* Add document row */}
                            {addState?.name === col.name && (
                                <div className="p-4 bg-blue-50 space-y-2">
                                    <p className="text-xs font-medium text-blue-700">New document</p>
                                    <div className="h-40 border border-blue-300 rounded overflow-hidden">
                                        <Editor
                                            defaultLanguage="json"
                                            value={addState.value}
                                            onChange={v => setAddState(prev => prev ? { ...prev, value: v ?? '' } : null)}
                                            options={{ minimap: { enabled: false }, fontSize: 12, lineNumbers: 'off', scrollBeyondLastLine: false }}
                                            theme="light"
                                        />
                                    </div>
                                    {editorError && <p className="text-xs text-red-600">{editorError}</p>}
                                    <div className="flex gap-2">
                                        <button onClick={saveAdd} className="flex items-center gap-1 text-xs px-3 py-1.5 bg-blue-600 text-white rounded hover:bg-blue-700">
                                            <Plus className="h-3 w-3" /> Insert
                                        </button>
                                        <button onClick={() => { setAddState(null); setEditorError(''); }} className="flex items-center gap-1 text-xs px-3 py-1.5 border border-gray-300 rounded hover:bg-gray-50">
                                            <X className="h-3 w-3" /> Cancel
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            ))}
        </div>
    );
}
