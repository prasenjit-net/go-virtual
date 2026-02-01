import { useState, useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Upload, FileText, AlertCircle } from 'lucide-react'
import { specsApi } from '../../services/api'

interface UploadSpecModalProps {
    onClose: () => void
}

export default function UploadSpecModal({ onClose }: UploadSpecModalProps) {
    const [name, setName] = useState('')
    const [basePath, setBasePath] = useState('')
    const [description, setDescription] = useState('')
    const [content, setContent] = useState('')
    const [fileName, setFileName] = useState('')
    const [error, setError] = useState('')
    const fileInputRef = useRef<HTMLInputElement>(null)
    const queryClient = useQueryClient()

    const createMutation = useMutation({
        mutationFn: specsApi.create,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['specs'] })
            onClose()
        },
        onError: (err: Error) => {
            setError(err.message)
        },
    })

    const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return

        setFileName(file.name)
        const reader = new FileReader()
        reader.onload = (event) => {
            const text = event.target?.result as string
            setContent(text)

            // Try to extract name from spec
            try {
                const spec = JSON.parse(text)
                if (spec.info?.title && !name) {
                    setName(spec.info.title)
                }
                if (spec.info?.description && !description) {
                    setDescription(spec.info.description)
                }
            } catch {
                // Try YAML parsing (basic)
                const titleMatch = text.match(/title:\s*(.+)/i)
                if (titleMatch && !name) {
                    setName(titleMatch[1].trim().replace(/^['"]|['"]$/g, ''))
                }
            }
        }
        reader.readAsText(file)
    }

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        if (!content) {
            setError('Please provide the OpenAPI specification content')
            return
        }

        createMutation.mutate({
            name: name || undefined,
            content,
            basePath: basePath || '/',
            description: description || undefined,
        })
    }

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white rounded-xl shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
                <div className="flex items-center justify-between p-6 border-b border-gray-200">
                    <h2 className="text-xl font-semibold text-gray-900">Upload OpenAPI Specification</h2>
                    <button
                        onClick={onClose}
                        className="p-2 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100"
                    >
                        <X className="w-5 h-5" />
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="p-6 space-y-6">
                    {error && (
                        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-start">
                            <AlertCircle className="w-5 h-5 text-red-600 mr-3 flex-shrink-0 mt-0.5" />
                            <p className="text-red-700">{error}</p>
                        </div>
                    )}

                    {/* File Upload */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Specification File
                        </label>
                        <div
                            onClick={() => fileInputRef.current?.click()}
                            className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center cursor-pointer hover:border-primary-400 hover:bg-primary-50 transition-colors"
                        >
                            <input
                                ref={fileInputRef}
                                type="file"
                                accept=".json,.yaml,.yml"
                                onChange={handleFileChange}
                                className="hidden"
                            />
                            {fileName ? (
                                <div className="flex items-center justify-center text-primary-600">
                                    <FileText className="w-8 h-8 mr-3" />
                                    <span className="font-medium">{fileName}</span>
                                </div>
                            ) : (
                                <>
                                    <Upload className="w-12 h-12 text-gray-400 mx-auto mb-4" />
                                    <p className="text-gray-600">
                                        Click to upload or drag and drop
                                    </p>
                                    <p className="text-sm text-gray-400 mt-1">
                                        Supports JSON and YAML formats
                                    </p>
                                </>
                            )}
                        </div>
                    </div>

                    {/* Or paste content */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Or Paste Specification Content
                        </label>
                        <textarea
                            value={content}
                            onChange={(e) => setContent(e.target.value)}
                            rows={8}
                            className="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 font-mono text-sm"
                            placeholder='{"openapi": "3.0.0", ...}'
                        />
                    </div>

                    {/* Name */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Name (optional)
                        </label>
                        <input
                            type="text"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                            placeholder="Will be extracted from spec if not provided"
                        />
                    </div>

                    {/* Base Path */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Base Path
                        </label>
                        <input
                            type="text"
                            value={basePath}
                            onChange={(e) => setBasePath(e.target.value)}
                            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                            placeholder="/api/v1"
                        />
                        <p className="text-sm text-gray-500 mt-1">
                            The path prefix where this API will be mounted
                        </p>
                    </div>

                    {/* Description */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Description (optional)
                        </label>
                        <textarea
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            rows={2}
                            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                            placeholder="A brief description of this API"
                        />
                    </div>

                    {/* Actions */}
                    <div className="flex justify-end gap-4 pt-4 border-t border-gray-200">
                        <button
                            type="button"
                            onClick={onClose}
                            className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            disabled={createMutation.isPending || !content}
                            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {createMutation.isPending ? 'Uploading...' : 'Upload Specification'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    )
}
