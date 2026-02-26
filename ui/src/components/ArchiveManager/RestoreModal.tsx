import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { X, RotateCcw, AlertTriangle, CheckCircle, ChevronRight } from 'lucide-react'
import { archivesApi } from '../../services/api'
import type { ArchiveMeta, RestoreResponse } from '../../types'

interface Props {
    archive: ArchiveMeta
    onClose: () => void
    onRestored: () => void
}

type Step = 'backup' | 'mode' | 'result'

export default function RestoreModal({ archive, onClose, onRestored }: Props) {
    const [step, setStep] = useState<Step>('backup')
    const [createBackup, setCreateBackup] = useState(true)
    const [backupLabel, setBackupLabel] = useState(
        `auto-before-restore-${new Date().toISOString().slice(0, 19).replace('T', '-')}`
    )
    const [wipeFirst, setWipeFirst] = useState(false)
    const [result, setResult] = useState<RestoreResponse | null>(null)

    const mutation = useMutation({
        mutationFn: () =>
            archivesApi.restore(archive.id, {
                createBackupFirst: createBackup,
                backupLabel: createBackup ? backupLabel : undefined,
                wipeFirst,
            }),
        onSuccess: (data) => {
            setResult(data)
            setStep('result')
        },
    })

    const archiveLabel = archive.label || archive.filename

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-lg mx-4">
                {/* Header */}
                <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-gray-100">
                        <RotateCcw className="w-4 h-4 text-green-500" />
                        Restore Archive
                    </div>
                    <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                        <X className="w-5 h-5" />
                    </button>
                </div>

                {step === 'result' && result ? (
                    /* ── Result panel ── */
                    <div className="px-5 py-4 space-y-4">
                        <div className="flex items-center gap-2 text-green-600 dark:text-green-400 font-medium">
                            <CheckCircle className="w-5 h-5" />
                            Restore complete
                        </div>

                        {result.backupCreated && (
                            <div className="text-sm bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-700 rounded p-3 text-indigo-800 dark:text-indigo-300">
                                Pre-restore backup created: <strong>{result.backupCreated.label}</strong>
                            </div>
                        )}

                        <div className="grid grid-cols-2 gap-2 text-sm">
                            {Object.entries(result.result.created).map(([k, v]) =>
                                v > 0 ? (
                                    <div key={k} className="bg-green-50 dark:bg-green-900/20 rounded p-2">
                                        <span className="text-gray-500 dark:text-gray-400">{k} created: </span>
                                        <strong className="text-green-700 dark:text-green-400">{v}</strong>
                                    </div>
                                ) : null
                            )}
                            {Object.entries(result.result.updated).map(([k, v]) =>
                                v > 0 ? (
                                    <div key={k} className="bg-blue-50 dark:bg-blue-900/20 rounded p-2">
                                        <span className="text-gray-500 dark:text-gray-400">{k} updated: </span>
                                        <strong className="text-blue-700 dark:text-blue-400">{v}</strong>
                                    </div>
                                ) : null
                            )}
                        </div>

                        {result.result.errors && result.result.errors.length > 0 && (
                            <div className="space-y-1">
                                <p className="text-sm font-medium text-red-600 dark:text-red-400">
                                    {result.result.errors.length} error(s):
                                </p>
                                {result.result.errors.map((e, i) => (
                                    <div key={i} className="text-xs bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded p-2 text-red-700 dark:text-red-400">
                                        <span className="font-mono">{e.path}</span>: {e.message}
                                    </div>
                                ))}
                            </div>
                        )}

                        <div className="flex justify-end">
                            <button
                                onClick={() => { onRestored(); onClose() }}
                                className="px-4 py-2 text-sm bg-green-600 text-white rounded-md hover:bg-green-700"
                            >
                                Done
                            </button>
                        </div>
                    </div>
                ) : (
                    /* ── Config steps ── */
                    <div className="px-5 py-4 space-y-5">
                        <p className="text-sm text-gray-600 dark:text-gray-400">
                            Restoring <strong className="text-gray-900 dark:text-gray-100">{archiveLabel}</strong>
                        </p>

                        {/* Step 1 — Safety backup */}
                        <div className={`rounded-lg border p-4 space-y-3 ${step === 'backup' ? 'border-indigo-300 dark:border-indigo-600 bg-indigo-50/40 dark:bg-indigo-900/10' : 'border-gray-200 dark:border-gray-700'}`}>
                            <div className="flex items-start justify-between">
                                <div>
                                    <p className="font-medium text-sm text-gray-900 dark:text-gray-100">Step 1 — Safety backup</p>
                                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                                        Archive current state before overwriting anything
                                    </p>
                                </div>
                                <label className="relative inline-flex items-center cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={createBackup}
                                        onChange={(e) => setCreateBackup(e.target.checked)}
                                        className="sr-only peer"
                                    />
                                    <div className="w-9 h-5 bg-gray-200 peer-focus:outline-none rounded-full peer peer-checked:bg-indigo-600 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-4" />
                                </label>
                            </div>
                            {createBackup && (
                                <input
                                    type="text"
                                    value={backupLabel}
                                    onChange={(e) => setBackupLabel(e.target.value)}
                                    className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                                />
                            )}
                        </div>

                        {/* Step 2 — Restore mode */}
                        <div className={`rounded-lg border p-4 space-y-2 ${step === 'mode' ? 'border-indigo-300 dark:border-indigo-600 bg-indigo-50/40 dark:bg-indigo-900/10' : 'border-gray-200 dark:border-gray-700'}`}>
                            <p className="font-medium text-sm text-gray-900 dark:text-gray-100">Step 2 — Restore mode</p>
                            <label className="flex items-start gap-3 cursor-pointer">
                                <input
                                    type="radio"
                                    name="restore-mode"
                                    checked={!wipeFirst}
                                    onChange={() => setWipeFirst(false)}
                                    className="mt-0.5"
                                />
                                <div>
                                    <p className="text-sm font-medium text-gray-900 dark:text-gray-100">Merge</p>
                                    <p className="text-xs text-gray-500 dark:text-gray-400">
                                        Upsert items from the archive. Items not in the archive are kept.
                                    </p>
                                </div>
                            </label>
                            <label className="flex items-start gap-3 cursor-pointer">
                                <input
                                    type="radio"
                                    name="restore-mode"
                                    checked={wipeFirst}
                                    onChange={() => setWipeFirst(true)}
                                    className="mt-0.5"
                                />
                                <div>
                                    <p className="text-sm font-medium text-gray-900 dark:text-gray-100">Wipe &amp; Restore</p>
                                    <p className="text-xs text-gray-500 dark:text-gray-400">
                                        Delete <em>all</em> existing data first, then apply the archive.
                                    </p>
                                </div>
                            </label>
                            {wipeFirst && (
                                <div className="flex items-center gap-2 mt-2 p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded text-xs text-red-700 dark:text-red-400">
                                    <AlertTriangle className="w-4 h-4 shrink-0" />
                                    All current data will be permanently deleted before restoring.
                                </div>
                            )}
                        </div>

                        {mutation.isError && (
                            <p className="text-sm text-red-600 dark:text-red-400">
                                {(mutation.error as Error).message}
                            </p>
                        )}

                        <div className="flex justify-end gap-2">
                            <button
                                onClick={onClose}
                                className="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={() => mutation.mutate()}
                                disabled={mutation.isPending}
                                className={`flex items-center gap-1.5 px-4 py-2 text-sm text-white rounded-md disabled:opacity-50 ${wipeFirst ? 'bg-red-600 hover:bg-red-700' : 'bg-green-600 hover:bg-green-700'}`}
                            >
                                {mutation.isPending ? (
                                    'Restoring…'
                                ) : (
                                    <>
                                        {wipeFirst ? 'Wipe & Restore' : 'Restore'}
                                        <ChevronRight className="w-4 h-4" />
                                    </>
                                )}
                            </button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}
