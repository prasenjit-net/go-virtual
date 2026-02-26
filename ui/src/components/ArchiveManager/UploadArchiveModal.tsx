import { useState, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { X, Upload, FileArchive } from 'lucide-react'
import { archivesApi } from '../../services/api'

interface Props {
    onClose: () => void
    onUploaded: () => void
}

export default function UploadArchiveModal({ onClose, onUploaded }: Props) {
    const [label, setLabel] = useState('')
    const [file, setFile] = useState<File | null>(null)
    const inputRef = useRef<HTMLInputElement>(null)

    const mutation = useMutation({
        mutationFn: () => archivesApi.upload(file!, label.trim() || undefined),
        onSuccess: () => onUploaded(),
    })

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-md mx-4">
                <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-gray-100">
                        <Upload className="w-4 h-4 text-indigo-500" />
                        Upload Archive
                    </div>
                    <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                        <X className="w-5 h-5" />
                    </button>
                </div>

                <div className="px-5 py-4 space-y-4">
                    {/* Drop zone */}
                    <div
                        onClick={() => inputRef.current?.click()}
                        className="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-6 text-center cursor-pointer hover:border-indigo-400 dark:hover:border-indigo-500 transition-colors"
                    >
                        <input
                            ref={inputRef}
                            type="file"
                            accept=".zip"
                            className="hidden"
                            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                        />
                        {file ? (
                            <div className="flex items-center justify-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                                <FileArchive className="w-5 h-5 text-indigo-500" />
                                {file.name}
                            </div>
                        ) : (
                            <div className="text-gray-500 dark:text-gray-400 text-sm">
                                <Upload className="w-6 h-6 mx-auto mb-1 text-gray-300 dark:text-gray-600" />
                                Click to select a <strong>.zip</strong> archive (max 50 MB)
                            </div>
                        )}
                    </div>

                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Label <span className="text-gray-400 font-normal">(optional)</span>
                        </label>
                        <input
                            type="text"
                            value={label}
                            onChange={(e) => setLabel(e.target.value)}
                            placeholder="Override label from archive"
                            className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
                        />
                    </div>

                    {mutation.isError && (
                        <p className="text-sm text-red-600 dark:text-red-400">
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
                        disabled={!file || mutation.isPending}
                        className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:opacity-50"
                    >
                        {mutation.isPending ? 'Uploading…' : 'Upload Archive'}
                    </button>
                </div>
            </div>
        </div>
    )
}
