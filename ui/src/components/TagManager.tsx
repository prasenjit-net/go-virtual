import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2, Edit2, Tag } from 'lucide-react'
import clsx from 'clsx'
import { tagsApi } from '../services/api'
import type { Tag as TagType } from '../types'

const defaultTagName = 'default'

export default function TagManager() {
    const queryClient = useQueryClient()
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [editing, setEditing] = useState<TagType | null>(null)
    const [isModalOpen, setIsModalOpen] = useState(false)
    const [pendingDelete, setPendingDelete] = useState<TagType | null>(null)

    const { data: tags, isLoading, error } = useQuery<TagType[]>({
        queryKey: ['tags'],
        queryFn: tagsApi.list,
    })

    const createMutation = useMutation({
        mutationFn: tagsApi.create,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tags'] })
            setName('')
            setDescription('')
            setIsModalOpen(false)
        },
    })

    const updateMutation = useMutation({
        mutationFn: ({ oldName, data }: { oldName: string; data: { name: string; description?: string } }) =>
            tagsApi.update(oldName, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tags'] })
            setEditing(null)
            setName('')
            setDescription('')
            setIsModalOpen(false)
        },
    })

    const deleteMutation = useMutation({
        mutationFn: tagsApi.delete,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['tags'] })
        },
    })

    const handleSubmit = () => {
        const trimmed = name.trim()
        if (!trimmed) return

        if (editing) {
            updateMutation.mutate({
                oldName: editing.name,
                data: { name: editing.name, description: description.trim() || undefined },
            })
            return
        }

        createMutation.mutate({ name: trimmed, description: description.trim() || undefined })
    }

    const startEdit = (tag: TagType) => {
        setEditing(tag)
        setName(tag.name)
        setDescription(tag.description || '')
        setIsModalOpen(true)
    }

    const cancelEdit = () => {
        setEditing(null)
        setName('')
        setDescription('')
        setIsModalOpen(false)
    }

    const startCreate = () => {
        setEditing(null)
        setName('')
        setDescription('')
        setIsModalOpen(true)
    }

    const openDeleteModal = (tag: TagType) => {
        setPendingDelete(tag)
    }

    const closeDeleteModal = () => {
        setPendingDelete(null)
    }

    if (isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (error) {
        return (
            <div className="p-8">
                <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700 dark:bg-red-950/40 dark:border-red-900/40 dark:text-red-300">
                    Failed to load tags: {(error as Error).message}
                </div>
            </div>
        )
    }

    return (
        <div className="p-8">
            <div className="flex items-center justify-between mb-8">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Tags</h1>
                    <p className="text-gray-500 dark:text-slate-400 mt-1">
                        Manage global response tags. The default tag is always included in matching.
                    </p>
                </div>
                <button
                    type="button"
                    onClick={startCreate}
                    className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                >
                    <Plus className="w-4 h-4 mr-2" />
                    Create Tag
                </button>
            </div>

            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800">
                <div className="p-6 border-b border-gray-200 dark:border-slate-800">
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">All Tags</h2>
                </div>
                <div className="divide-y divide-gray-100 dark:divide-slate-800">
                    {(tags || []).map((tag) => (
                        <div key={tag.name} className="p-6 flex items-center justify-between">
                            <div className="flex items-center">
                                <div className={clsx(
                                    'p-2 rounded-lg',
                                    tag.name === defaultTagName
                                        ? 'bg-gray-100 dark:bg-slate-800'
                                        : 'bg-primary-100/80 dark:bg-primary-900/30'
                                )}>
                                    <Tag className={clsx(
                                        'w-5 h-5',
                                        tag.name === defaultTagName
                                            ? 'text-gray-500 dark:text-slate-300'
                                            : 'text-primary-600'
                                    )} />
                                </div>
                                <div className="ml-4">
                                    <div className="flex items-center gap-2">
                                        <span className="font-medium text-gray-900 dark:text-slate-100">
                                            {tag.name}
                                        </span>
                                        {tag.name === defaultTagName && (
                                            <span className="text-xs bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-300 px-2 py-0.5 rounded">
                                                built-in
                                            </span>
                                        )}
                                    </div>
                                    <p className="text-sm text-gray-500 dark:text-slate-400">
                                        {tag.description || 'No description'}
                                    </p>
                                </div>
                            </div>
                            <div className="flex items-center gap-2">
                                <button
                                    type="button"
                                    onClick={() => startEdit(tag)}
                                    className="p-2 text-gray-400 dark:text-slate-500 hover:text-primary-600 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/30 transition-colors"
                                    title={tag.name === defaultTagName ? 'Edit default tag description' : 'Edit tag'}
                                >
                                    <Edit2 className="w-5 h-5" />
                                </button>
                                <button
                                    type="button"
                                    onClick={() => openDeleteModal(tag)}
                                    className="p-2 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded-lg hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                                    disabled={tag.name === defaultTagName}
                                    title={tag.name === defaultTagName ? 'Default tag cannot be deleted' : 'Delete tag'}
                                >
                                    <Trash2 className="w-5 h-5" />
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            </div>

            {isModalOpen && (
                <div className="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-lg mx-4 overflow-hidden">
                        <div className="p-6 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">
                                {editing ? 'Edit Tag' : 'Create Tag'}
                            </h2>
                            <button
                                type="button"
                                onClick={cancelEdit}
                                className="text-gray-400 hover:text-gray-600 dark:text-slate-500 dark:hover:text-slate-200"
                            >
                                ✕
                            </button>
                        </div>
                        <div className="p-6 space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                    Tag name
                                </label>
                                <input
                                    value={name}
                                    onChange={(e) => setName(e.target.value)}
                                    placeholder="e.g., happy-path"
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:bg-gray-100 dark:disabled:bg-slate-800"
                                    disabled={Boolean(editing)}
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                    Description
                                </label>
                                <input
                                    value={description}
                                    onChange={(e) => setDescription(e.target.value)}
                                    placeholder="Optional description"
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                />
                            </div>
                        </div>
                        <div className="px-6 pb-6 flex items-center justify-end gap-2">
                            <button
                                type="button"
                                onClick={cancelEdit}
                                className="px-4 py-2 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-slate-800 rounded-lg"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={handleSubmit}
                                className="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                                disabled={createMutation.isPending || updateMutation.isPending}
                            >
                                <Plus className="w-4 h-4 mr-2" />
                                {editing ? 'Update Tag' : 'Create Tag'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {pendingDelete && (
                <div className="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-xl w-full max-w-md mx-4 overflow-hidden">
                        <div className="p-6 border-b border-gray-200 dark:border-slate-800">
                            <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">
                                Delete tag
                            </h2>
                        </div>
                        <div className="p-6 space-y-3 text-sm text-gray-600 dark:text-slate-300">
                            <p>
                                Are you sure you want to delete the tag <span className="font-semibold text-gray-900 dark:text-slate-100">{pendingDelete.name}</span>?
                            </p>
                            <p>
                                Responses using this tag will switch to <span className="font-semibold text-gray-900 dark:text-slate-100">default</span>.
                            </p>
                        </div>
                        <div className="px-6 pb-6 flex items-center justify-end gap-2">
                            <button
                                type="button"
                                onClick={closeDeleteModal}
                                className="px-4 py-2 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-slate-800 rounded-lg"
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                onClick={() => {
                                    deleteMutation.mutate(pendingDelete.name)
                                    closeDeleteModal()
                                }}
                                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                            >
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
