import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { X, Archive } from 'lucide-react'
import { archivesApi } from '../../services/api'

interface Props {
    onClose: () => void
    onCreated: () => void
}

export default function CreateArchiveModal({ onClose, onCreated }: Props) {
    const [label, setLabel] = useState('')

    const mutation = useMutation({
        mutationFn: () => archivesApi.create(label.trim() || undefined),
        onSuccess: () => onCreated(),
    })

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md mx-4">
                <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-gray-100">
                        <Archive className="w-4 h-4 text-indigo-500" />
                        Create Archive
                    </div>
                    <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                        <X className="w-5 h-5" />
                    </button>
                </div>

                <div className="px-5 py-4">
                    <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                        A ZIP snapshot of all specs, responses, scripts, tags, and store entries will be saved to the server.
                        TLS certificates and configuration are excluded.
                    </p>
                    <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Label <span className="text-gray-400 font-normal">(optional)</span>
                    </label>
                    <input
                        type="text"
                        value={label}
                        onChange={(e) => setLabel(e.target.value)}
                        placeholder="e.g. before-release-v2"
                        className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                        onKeyDown={(e) => { if (e.key === 'Enter') mutation.mutate() }}
                    />
                    {mutation.isError && (
                        <p className="mt-2 text-sm text-red-600 dark:text-red-400">
                            {(mutation.error as Error).message}
                        </p>
                    )}
                </div>

                <div className="flex justify-end gap-2 px-5 py-4 border-t border-gray-200 dark:border-gray-700">
                    <button
                        onClick={onClose}
                        className="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={() => mutation.mutate()}
                        disabled={mutation.isPending}
                        className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50"
                    >
                        {mutation.isPending ? 'Creating…' : 'Create Archive'}
                    </button>
                </div>
            </div>
        </div>
    )
}
