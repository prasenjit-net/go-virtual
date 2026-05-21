import { useState } from 'react'
import { AlertCircle, Upload, X } from 'lucide-react'
import type { ResponseConfigInput } from '../../types'
import { parseResponseImportPayload } from './responseTransfer'

interface ResponseImportModalProps {
    onClose: () => void
    onImport: (input: ResponseConfigInput) => void
    isSubmitting?: boolean
}

export default function ResponseImportModal({
    onClose,
    onImport,
    isSubmitting = false,
}: ResponseImportModalProps) {
    const [rawPayload, setRawPayload] = useState('')
    const [error, setError] = useState('')

    const handleImport = (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        try {
            const parsed = parseResponseImportPayload(rawPayload)
            onImport(parsed)
        } catch (err) {
            setError((err as Error).message)
        }
    }

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-2xl w-full max-w-2xl border border-gray-200 dark:border-slate-700">
                <div className="flex items-center justify-between p-5 border-b border-gray-200 dark:border-slate-700">
                    <div>
                        <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">Import Response</h2>
                        <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5">
                            Paste a copied response payload to import into this operation.
                        </p>
                    </div>
                    <button
                        onClick={onClose}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                <form onSubmit={handleImport} className="p-5 space-y-4">
                    <textarea
                        value={rawPayload}
                        onChange={(e) => setRawPayload(e.target.value)}
                        rows={14}
                        placeholder="Paste copied response payload JSON here..."
                        className="w-full px-3 py-2 text-sm font-mono rounded-lg border border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-y"
                        disabled={isSubmitting}
                    />

                    {error && (
                        <div className="flex items-start gap-2 p-3 bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900/40 rounded-lg text-sm text-red-700 dark:text-red-300">
                            <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                            <span>{error}</span>
                        </div>
                    )}

                    <div className="flex items-center justify-end gap-2">
                        <button
                            type="button"
                            onClick={onClose}
                            disabled={isSubmitting}
                            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-slate-300 bg-white dark:bg-slate-800 border border-gray-300 dark:border-slate-600 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors disabled:opacity-50"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={isSubmitting}
                            className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                        >
                            <Upload className="w-4 h-4" />
                            {isSubmitting ? 'Importing…' : 'Import'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
