import { useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Archive, Plus, Upload, RefreshCw, Download, Trash2, RotateCcw, ChevronDown, ChevronUp } from 'lucide-react'
import { archivesApi } from '../../services/api'
import type { ArchiveMeta } from '../../types'
import CreateArchiveModal from './CreateArchiveModal'
import UploadArchiveModal from './UploadArchiveModal'
import RestoreModal from './RestoreModal'

function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatRelativeTime(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime()
    const seconds = Math.floor(diff / 1000)
    if (seconds < 60) return 'just now'
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}m ago`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    return `${days}d ago`
}

// ── Snapshot mode panel ───────────────────────────────────────────────────────

function SnapshotPanel() {
    const fileInputRef = useRef<HTMLInputElement>(null)
    const [restoreMsg, setRestoreMsg] = useState<string | null>(null)
    const [restoreError, setRestoreError] = useState<string | null>(null)

    const restoreMutation = useMutation({
        mutationFn: archivesApi.restoreSnapshot,
        onSuccess: () => {
            setRestoreError(null)
            setRestoreMsg('Snapshot restored successfully. All data has been replaced.')
        },
        onError: (err: Error) => {
            setRestoreMsg(null)
            setRestoreError(err.message || 'Restore failed')
        },
    })

    const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return
        if (!confirm(`Upload and restore "${file.name}"? This will wipe ALL current data and replace it with the contents of the uploaded file.`)) {
            e.target.value = ''
            return
        }
        setRestoreMsg(null)
        setRestoreError(null)
        restoreMutation.mutate(file)
        e.target.value = ''
    }

    return (
        <div className="p-8">
            <div className="flex items-start gap-3 mb-8">
                <div className="rounded-lg bg-indigo-100 p-3 dark:bg-indigo-900/30">
                    <Archive className="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
                </div>
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Archives — Snapshot Mode</h1>
                    <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                        In-memory or MongoDB storage: download the current state or upload a full replacement snapshot.
                    </p>
                </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {/* Download */}
                <a
                    href={archivesApi.snapshotDownloadUrl()}
                    download
                    className="flex flex-col items-center justify-center gap-3 p-6 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors group"
                >
                    <Download className="w-8 h-8 text-indigo-500 group-hover:text-indigo-600" />
                    <div className="text-center">
                        <div className="font-medium text-gray-900 dark:text-gray-100">Download Snapshot</div>
                        <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            Export all current data as a ZIP file
                        </div>
                    </div>
                </a>

                {/* Upload & Restore */}
                <button
                    onClick={() => fileInputRef.current?.click()}
                    disabled={restoreMutation.isPending}
                    className="flex flex-col items-center justify-center gap-3 p-6 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors group disabled:opacity-60"
                >
                    <Upload className="w-8 h-8 text-amber-500 group-hover:text-amber-600" />
                    <div className="text-center">
                        <div className="font-medium text-gray-900 dark:text-gray-100">
                            {restoreMutation.isPending ? 'Restoring…' : 'Upload & Restore'}
                        </div>
                        <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            Replace all data with an uploaded snapshot ZIP
                        </div>
                    </div>
                </button>
                <input
                    ref={fileInputRef}
                    type="file"
                    accept=".zip"
                    className="hidden"
                    onChange={handleFileChange}
                />
            </div>

            {restoreMsg && (
                <div className="mt-4 p-3 rounded bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-700 text-green-700 dark:text-green-300 text-sm">
                    {restoreMsg}
                </div>
            )}
            {restoreError && (
                <div className="mt-4 p-3 rounded bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 text-red-700 dark:text-red-300 text-sm">
                    {restoreError}
                </div>
            )}

            <p className="mt-6 text-xs text-gray-400 dark:text-gray-500">
                ⚠️ Upload & Restore wipes <strong>all current data</strong> before applying the snapshot. Make sure to download a backup first.
            </p>
        </div>
    )
}

// ── Full mode panel (original UI) ─────────────────────────────────────────────

export default function ArchiveManager() {
    const queryClient = useQueryClient()
    const [showCreate, setShowCreate] = useState(false)
    const [showUpload, setShowUpload] = useState(false)
    const [restoreTarget, setRestoreTarget] = useState<ArchiveMeta | null>(null)
    const [expandedId, setExpandedId] = useState<string | null>(null)

    const { data: info, isLoading: infoLoading } = useQuery({
        queryKey: ['archives-info'],
        queryFn: archivesApi.info,
        retry: false,
    })

    const { data: archives = [], isLoading, refetch } = useQuery({
        queryKey: ['archives'],
        queryFn: archivesApi.list,
        enabled: info?.mode === 'full',
    })

    const deleteMutation = useMutation({
        mutationFn: archivesApi.delete,
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['archives'] }),
    })

    const handleDelete = (meta: ArchiveMeta) => {
        if (!confirm(`Delete archive "${meta.label || meta.filename}"? This cannot be undone.`)) return
        deleteMutation.mutate(meta.id)
    }

    if (infoLoading) {
        return <div className="p-8 text-center text-gray-500 dark:text-gray-400">Loading…</div>
    }

    // Snapshot mode: delegate to dedicated panel
    if (info?.mode === 'snapshot') {
        return <SnapshotPanel />
    }

    return (
        <div className="p-8">
            {/* Header */}
            <div className="flex items-center justify-between mb-8">
                <div className="flex items-start gap-3">
                    <div className="rounded-lg bg-indigo-100 p-3 dark:bg-indigo-900/30">
                        <Archive className="h-6 w-6 text-indigo-600 dark:text-indigo-400" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">Archives</h1>
                        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                            Create and restore point-in-time snapshots of all data
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    <button
                        onClick={() => refetch()}
                        className="p-2 rounded-md text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:bg-gray-700 dark:hover:text-gray-300 transition-colors"
                        title="Refresh"
                    >
                        <RefreshCw className="w-4 h-4" />
                    </button>
                    <button
                        onClick={() => setShowUpload(true)}
                        className="flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                    >
                        <Upload className="w-5 h-5 mr-2" />
                        Upload
                    </button>
                    <button
                        onClick={() => setShowCreate(true)}
                        className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                    >
                        <Plus className="w-5 h-5 mr-2" />
                        Create Archive
                    </button>
                </div>
            </div>

            {/* Archive list */}
            {isLoading ? (
                <div className="text-center py-12 text-gray-500 dark:text-gray-400">Loading archives…</div>
            ) : archives.length === 0 ? (
                <div className="text-center py-16 bg-white dark:bg-gray-800 rounded-lg border border-dashed border-gray-300 dark:border-gray-600">
                    <Archive className="w-10 h-10 mx-auto mb-3 text-gray-300 dark:text-gray-600" />
                    <p className="text-gray-500 dark:text-gray-400 font-medium">No archives yet</p>
                    <p className="text-sm text-gray-400 dark:text-gray-500 mt-1">
                        Click <strong>Create Archive</strong> to take a snapshot of the current data
                    </p>
                </div>
            ) : (
                <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 divide-y divide-gray-100 dark:divide-gray-700">
                    {archives.map((meta) => {
                        const expanded = expandedId === meta.id
                        return (
                            <div key={meta.id}>
                                <div className="flex items-center gap-4 px-4 py-3">
                                    {/* Toggle */}
                                    <button
                                        onClick={() => setExpandedId(expanded ? null : meta.id)}
                                        className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                                    >
                                        {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                                    </button>

                                    {/* Main info */}
                                    <div className="flex-1 min-w-0">
                                        <div className="flex items-center gap-2">
                                            <span className="font-medium text-gray-900 dark:text-gray-100 truncate">
                                                {meta.label || meta.filename}
                                            </span>
                                            {meta.label && (
                                                <span className="text-xs text-gray-400 dark:text-gray-500 font-mono truncate hidden sm:block">
                                                    {meta.filename}
                                                </span>
                                            )}
                                        </div>
                                        <div className="flex items-center gap-3 mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                                            <span title={new Date(meta.createdAt).toLocaleString()}>
                                                {formatRelativeTime(meta.createdAt)}
                                            </span>
                                            <span>{formatBytes(meta.sizeBytes)}</span>
                                            <span>{meta.appVersion}</span>
                                        </div>
                                    </div>

                                    {/* Counts */}
                                    <div className="hidden md:flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
                                        <span>{meta.counts.specs} spec{meta.counts.specs !== 1 ? 's' : ''}</span>
                                        <span>·</span>
                                        <span>{meta.counts.responses} resp</span>
                                        <span>·</span>
                                        <span>{meta.counts.scripts} scripts</span>
                                    </div>

                                    {/* Actions */}
                                    <div className="flex items-center gap-1">
                                        <a
                                            href={archivesApi.downloadUrl(meta.id)}
                                            download={meta.filename}
                                            className="p-1.5 rounded text-gray-400 hover:text-indigo-600 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors"
                                            title="Download"
                                        >
                                            <Download className="w-4 h-4" />
                                        </a>
                                        <button
                                            onClick={() => setRestoreTarget(meta)}
                                            className="p-1.5 rounded text-gray-400 hover:text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors"
                                            title="Restore"
                                        >
                                            <RotateCcw className="w-4 h-4" />
                                        </button>
                                        <button
                                            onClick={() => handleDelete(meta)}
                                            disabled={deleteMutation.isPending}
                                            className="p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                                            title="Delete"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                    </div>
                                </div>

                                {/* Expanded detail */}
                                {expanded && (
                                    <div className="px-4 pb-4 pt-0 ml-8 grid grid-cols-2 sm:grid-cols-3 gap-3">
                                        {[
                                            { label: 'Specs', value: meta.counts.specs },
                                            { label: 'Responses', value: meta.counts.responses },
                                            { label: 'Scripts', value: meta.counts.scripts },
                                            { label: 'Tags', value: meta.counts.tags },
                                            { label: 'Store entries', value: meta.counts.storeEntries },
                                            { label: 'Size', value: formatBytes(meta.sizeBytes) },
                                            { label: 'App version', value: meta.appVersion },
                                        ].map(({ label, value }) => (
                                            <div key={label} className="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
                                                <div className="text-xs text-gray-500 dark:text-gray-400">{label}</div>
                                                <div className="font-semibold text-gray-900 dark:text-gray-100">{value}</div>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        )
                    })}
                </div>
            )}

            {/* Modals */}
            {showCreate && (
                <CreateArchiveModal
                    onClose={() => setShowCreate(false)}
                    onCreated={() => {
                        setShowCreate(false)
                        queryClient.invalidateQueries({ queryKey: ['archives'] })
                    }}
                />
            )}
            {showUpload && (
                <UploadArchiveModal
                    onClose={() => setShowUpload(false)}
                    onUploaded={() => {
                        setShowUpload(false)
                        queryClient.invalidateQueries({ queryKey: ['archives'] })
                    }}
                />
            )}
            {restoreTarget && (
                <RestoreModal
                    archive={restoreTarget}
                    onClose={() => setRestoreTarget(null)}
                    onRestored={() => {
                        setRestoreTarget(null)
                        queryClient.invalidateQueries({ queryKey: ['archives'] })
                    }}
                />
            )}
        </div>
    )
}
