import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Sparkles, Loader2, AlertCircle } from 'lucide-react'
import { aiApi } from '../../services/api'

interface AIGenerateModalProps {
    operationId: string
    operationMethod: string
    operationPath: string
    onClose: () => void
}

export default function AIGenerateModal({
    operationId,
    operationMethod,
    operationPath,
    onClose,
}: AIGenerateModalProps) {
    const [userPrompt, setUserPrompt] = useState('')
    const queryClient = useQueryClient()

    const generateMutation = useMutation({
        mutationFn: () => aiApi.generateResponse(operationId, userPrompt.trim() || undefined),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
    })

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        generateMutation.mutate()
    }

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-2xl w-full max-w-lg border border-gray-200 dark:border-slate-700">
                {/* Header */}
                <div className="flex items-center justify-between p-5 border-b border-gray-200 dark:border-slate-700">
                    <div className="flex items-center gap-2">
                        <div className="p-1.5 bg-purple-100 dark:bg-purple-900/40 rounded-lg">
                            <Sparkles className="w-4 h-4 text-purple-600 dark:text-purple-400" />
                        </div>
                        <div>
                            <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">
                                Generate with AI
                            </h2>
                            <p className="text-xs text-gray-500 dark:text-slate-400 mt-0.5 font-mono">
                                {operationMethod} {operationPath}
                            </p>
                        </div>
                    </div>
                    <button
                        onClick={onClose}
                        className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                    >
                        <X className="w-4 h-4" />
                    </button>
                </div>

                {/* Body */}
                <form onSubmit={handleSubmit} className="p-5 space-y-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                            Custom instructions{' '}
                            <span className="text-gray-400 dark:text-slate-500 font-normal">(optional)</span>
                        </label>
                        <textarea
                            value={userPrompt}
                            onChange={(e) => setUserPrompt(e.target.value)}
                            placeholder="e.g. Return a 404 error response for a missing resource, or return a list of 3 users with realistic names and emails…"
                            rows={4}
                            className="w-full px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-slate-600 bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 placeholder-gray-400 dark:placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-purple-500 resize-none"
                            disabled={generateMutation.isPending}
                        />
                        <p className="text-xs text-gray-500 dark:text-slate-500 mt-1.5">
                            Leave empty to generate a success response with realistic fake data.
                        </p>
                    </div>

                    {generateMutation.isError && (
                        <div className="flex items-start gap-2 p-3 bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900/40 rounded-lg text-sm text-red-700 dark:text-red-300">
                            <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                            <span>{(generateMutation.error as Error).message}</span>
                        </div>
                    )}

                    {/* Actions */}
                    <div className="flex items-center justify-end gap-2 pt-1">
                        <button
                            type="button"
                            onClick={onClose}
                            disabled={generateMutation.isPending}
                            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-slate-300 bg-white dark:bg-slate-800 border border-gray-300 dark:border-slate-600 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors disabled:opacity-50"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={generateMutation.isPending}
                            className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-purple-600 hover:bg-purple-700 rounded-lg transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                        >
                            {generateMutation.isPending ? (
                                <>
                                    <Loader2 className="w-4 h-4 animate-spin" />
                                    Generating…
                                </>
                            ) : (
                                <>
                                    <Sparkles className="w-4 h-4" />
                                    Generate Response
                                </>
                            )}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
