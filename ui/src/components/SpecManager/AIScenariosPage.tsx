import { useEffect, useMemo, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Bot, ChevronRight, Plus, Save, Trash2 } from 'lucide-react'
import clsx from 'clsx'
import { aiScenariosApi } from '../../services/api'
import type { AIScenario, AIScenarioKind } from '../../types'

type ScenarioDraft = {
    name: string
    description: string
    responseKind: AIScenarioKind
    statusCode: string
    count: string
    instructions: string
    enabled: boolean
}

const defaultScenarioNames = new Set(['success', 'client_error', 'server_error'])

const emptyDraft: ScenarioDraft = {
    name: '',
    description: '',
    responseKind: 'success',
    statusCode: '',
    count: '',
    instructions: '',
    enabled: true,
}

function scenarioToDraft(scenario: AIScenario): ScenarioDraft {
    return {
        name: scenario.name,
        description: scenario.description || '',
        responseKind: scenario.responseKind,
        statusCode: scenario.statusCode > 0 ? String(scenario.statusCode) : '',
        count: scenario.count > 0 ? String(scenario.count) : '',
        instructions: scenario.instructions || '',
        enabled: scenario.enabled,
    }
}

function draftToPayload(draft: ScenarioDraft) {
    return {
        name: draft.name.trim(),
        description: draft.description.trim(),
        responseKind: draft.responseKind,
        statusCode: draft.statusCode.trim() ? Number(draft.statusCode) : 0,
        count: draft.count.trim() ? Number(draft.count) : 0,
        instructions: draft.instructions.trim(),
        enabled: draft.enabled,
    }
}

function scenarioSubtitle(scenario: AIScenario) {
    const parts: string[] = [scenario.responseKind === 'success' ? 'Success' : 'Error']
    if (scenario.statusCode > 0) {
        parts.push(`Status ${scenario.statusCode}`)
    } else if (scenario.responseKind === 'success') {
        parts.push('Default success status')
    }
    if (scenario.count > 0) {
        parts.push(`Count ${scenario.count}`)
    }
    if (scenario.description) {
        parts.push(scenario.description)
    }
    return parts.join(' • ')
}

function DraftFields({
    draft,
    setDraft,
}: {
    draft: ScenarioDraft
    setDraft: Dispatch<SetStateAction<ScenarioDraft>>
}) {
    return (
        <div className="space-y-4">
            <label className="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-slate-200">
                <input
                    type="checkbox"
                    checked={draft.enabled}
                    onChange={(e) => setDraft((current) => ({ ...current, enabled: e.target.checked }))}
                    className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                Enabled
            </label>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <label className="space-y-1 text-sm">
                    <span className="text-gray-700 dark:text-slate-200">Name</span>
                    <input
                        value={draft.name}
                        onChange={(e) => setDraft((current) => ({ ...current, name: e.target.value }))}
                        className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                    />
                </label>
                <label className="space-y-1 text-sm">
                    <span className="text-gray-700 dark:text-slate-200">Response kind</span>
                    <select
                        value={draft.responseKind}
                        onChange={(e) => setDraft((current) => ({ ...current, responseKind: e.target.value as AIScenarioKind }))}
                        className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                    >
                        <option value="success">Success</option>
                        <option value="error">Error</option>
                    </select>
                </label>
                <label className="space-y-1 text-sm">
                    <span className="text-gray-700 dark:text-slate-200">Status code</span>
                    <input
                        type="number"
                        min="0"
                        value={draft.statusCode}
                        onChange={(e) => setDraft((current) => ({ ...current, statusCode: e.target.value }))}
                        placeholder={draft.responseKind === 'success' ? 'Default success status' : '400'}
                        className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                    />
                </label>
                <label className="space-y-1 text-sm">
                    <span className="text-gray-700 dark:text-slate-200">Count hint</span>
                    <input
                        type="number"
                        min="0"
                        value={draft.count}
                        onChange={(e) => setDraft((current) => ({ ...current, count: e.target.value }))}
                        placeholder="5"
                        className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                    />
                </label>
            </div>

            <label className="block space-y-1 text-sm">
                <span className="text-gray-700 dark:text-slate-200">Description</span>
                <input
                    value={draft.description}
                    onChange={(e) => setDraft((current) => ({ ...current, description: e.target.value }))}
                    className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                />
            </label>

            <label className="block space-y-1 text-sm">
                <span className="text-gray-700 dark:text-slate-200">Instructions</span>
                <textarea
                    value={draft.instructions}
                    onChange={(e) => setDraft((current) => ({ ...current, instructions: e.target.value }))}
                    rows={4}
                    className="w-full rounded-lg border border-gray-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-gray-900 dark:text-slate-100"
                />
            </label>
        </div>
    )
}

function ScenarioListItem({
    scenario,
    selected,
    onSelect,
    onDelete,
    deleting,
}: {
    scenario: AIScenario
    selected: boolean
    onSelect: () => void
    onDelete: () => void
    deleting: boolean
}) {
    const isDefault = defaultScenarioNames.has(scenario.name)

    return (
        <div
            className={clsx(
                'rounded-xl border transition-colors',
                selected
                    ? 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/30'
                    : 'border-gray-200 bg-white hover:border-gray-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700'
            )}
        >
            <div className="flex items-stretch">
                <button
                    type="button"
                    onClick={onSelect}
                    className="flex-1 min-w-0 px-4 py-3 text-left"
                >
                    <div className="flex items-center gap-2">
                        <h3 className="truncate text-sm font-semibold text-gray-900 dark:text-slate-100">{scenario.name}</h3>
                        {isDefault && (
                            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-slate-800 dark:text-slate-300">
                                Default
                            </span>
                        )}
                        <span
                            className={clsx(
                                'rounded-full px-2 py-0.5 text-[11px] font-medium',
                                scenario.enabled
                                    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
                                    : 'bg-gray-100 text-gray-500 dark:bg-slate-800 dark:text-slate-400'
                            )}
                        >
                            {scenario.enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                    <p className="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-slate-400">
                        {scenarioSubtitle(scenario)}
                    </p>
                </button>

                <div className="flex items-center gap-1 pr-2">
                    <button
                        type="button"
                        onClick={(e) => {
                            e.stopPropagation()
                            onDelete()
                        }}
                        disabled={deleting}
                        className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-300 dark:hover:bg-red-950/30"
                        title={`Delete ${scenario.name}`}
                    >
                        <Trash2 className="h-4 w-4" />
                    </button>
                    <button
                        type="button"
                        onClick={onSelect}
                        className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 dark:text-slate-500 dark:hover:bg-slate-800"
                        title={`Open ${scenario.name}`}
                    >
                        <ChevronRight className="h-4 w-4" />
                    </button>
                </div>
            </div>
        </div>
    )
}

function ScenarioDetail({
    title,
    description,
    draft,
    setDraft,
    onReset,
    onSave,
    saveLabel,
    saving,
}: {
    title: string
    description: string
    draft: ScenarioDraft
    setDraft: Dispatch<SetStateAction<ScenarioDraft>>
    onReset: () => void
    onSave: () => void
    saveLabel: string
    saving: boolean
}) {
    return (
        <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <div className="mb-6">
                <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">{title}</h2>
                <p className="mt-1 text-sm text-gray-500 dark:text-slate-400">{description}</p>
            </div>

            <DraftFields draft={draft} setDraft={setDraft} />

            <div className="mt-6 flex items-center justify-end gap-3">
                <button
                    type="button"
                    onClick={onReset}
                    className="rounded-lg border border-gray-300 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                >
                    Reset
                </button>
                <button
                    type="button"
                    onClick={onSave}
                    disabled={saving}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm text-white hover:bg-primary-700 disabled:opacity-50"
                >
                    <Save className="h-4 w-4" />
                    {saveLabel}
                </button>
            </div>
        </div>
    )
}

export default function AIScenariosPage() {
    const queryClient = useQueryClient()
    const [newDraft, setNewDraft] = useState<ScenarioDraft>(emptyDraft)
    const [editDraft, setEditDraft] = useState<ScenarioDraft>(emptyDraft)
    const [selectedScenarioId, setSelectedScenarioId] = useState<string | 'new' | null>(null)

    const scenariosQuery = useQuery({
        queryKey: ['ai-scenarios'],
        queryFn: () => aiScenariosApi.list(),
    })

    const createMutation = useMutation({
        mutationFn: () => aiScenariosApi.create(draftToPayload(newDraft)),
        onSuccess: (result) => {
            setNewDraft(emptyDraft)
            setSelectedScenarioId(result.scenario.id)
            queryClient.invalidateQueries({ queryKey: ['ai-scenarios'] })
        },
    })

    const updateMutation = useMutation({
        mutationFn: (scenarioId: string) => aiScenariosApi.update(scenarioId, draftToPayload(editDraft)),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['ai-scenarios'] })
        },
    })

    const deleteMutation = useMutation({
        mutationFn: (scenarioId: string) => aiScenariosApi.delete(scenarioId),
        onSuccess: (_, deletedId) => {
            const remaining = scenarios.filter((scenario) => scenario.id !== deletedId)
            setSelectedScenarioId(remaining[0]?.id ?? 'new')
            queryClient.invalidateQueries({ queryKey: ['ai-scenarios'] })
        },
    })

    const scenarios = useMemo(() => scenariosQuery.data?.scenarios || [], [scenariosQuery.data])
    const selectedScenario = useMemo(
        () => scenarios.find((scenario) => scenario.id === selectedScenarioId) ?? null,
        [scenarios, selectedScenarioId]
    )

    useEffect(() => {
        if (!scenarios.length) {
            setSelectedScenarioId('new')
            return
        }
        if (selectedScenarioId === null) {
            setSelectedScenarioId(scenarios[0].id)
            return
        }
        if (selectedScenarioId !== 'new' && !scenarios.some((scenario) => scenario.id === selectedScenarioId)) {
            setSelectedScenarioId(scenarios[0].id)
        }
    }, [scenarios, selectedScenarioId])

    useEffect(() => {
        if (selectedScenario) {
            setEditDraft(scenarioToDraft(selectedScenario))
        }
    }, [selectedScenario])

    if (scenariosQuery.isLoading) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                    <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    if (scenariosQuery.error) {
        return (
            <div className="p-8">
                <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300">
                    Failed to load AI scenarios.
                </div>
            </div>
        )
    }

    return (
        <div className="p-8 space-y-8">
            <div className="flex items-center justify-between">
                <div className="flex items-start gap-3">
                    <div className="rounded-lg bg-fuchsia-100 p-3 dark:bg-fuchsia-900/30">
                        <Bot className="h-6 w-6 text-fuchsia-600 dark:text-fuchsia-400" />
                    </div>
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">AI Scenarios</h1>
                        <p className="mt-1 text-sm text-gray-500 dark:text-slate-400">
                            All specs share these named scenarios when requests send{' '}
                            <code className="rounded bg-gray-100 px-1 py-0.5 dark:bg-slate-800">X-Virtual-AI-Scenario</code>.
                        </p>
                    </div>
                </div>
            </div>

            <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                <div className="flex items-start gap-2">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                    <div>
                        All <code>X-Virtual-*</code> headers are excluded from recorded-response signature hashing, so these control headers do not fragment replayed recordings.
                    </div>
                </div>
            </div>

            <div className="grid grid-cols-1 gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
                <div className="space-y-3">
                    <button
                        type="button"
                        onClick={() => setSelectedScenarioId('new')}
                        className={clsx(
                            'flex w-full items-center justify-center gap-2 rounded-xl border border-dashed px-4 py-3 text-sm font-medium transition-colors',
                            selectedScenarioId === 'new'
                                ? 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-700 dark:bg-primary-950/20 dark:text-primary-300'
                                : 'border-gray-300 bg-white text-gray-700 hover:bg-gray-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800'
                        )}
                    >
                        <Plus className="h-4 w-4" />
                        Create scenario
                    </button>

                    <div className="space-y-3">
                        {scenarios.map((scenario) => (
                            <ScenarioListItem
                                key={scenario.id}
                                scenario={scenario}
                                selected={selectedScenarioId === scenario.id}
                                onSelect={() => setSelectedScenarioId(scenario.id)}
                                onDelete={() => deleteMutation.mutate(scenario.id)}
                                deleting={deleteMutation.isPending}
                            />
                        ))}
                    </div>
                </div>

                <div>
                    {selectedScenarioId === 'new' || !selectedScenario ? (
                        <ScenarioDetail
                            title="Create AI scenario"
                            description="Add a new named runtime behavior for AI fallback. Clients can request it through the X-Virtual-AI-Scenario header."
                            draft={newDraft}
                            setDraft={setNewDraft}
                            onReset={() => setNewDraft(emptyDraft)}
                            onSave={() => createMutation.mutate()}
                            saveLabel="Add Scenario"
                            saving={createMutation.isPending}
                        />
                    ) : (
                        <ScenarioDetail
                            title={`Edit ${selectedScenario.name}`}
                            description={`Runtime selector: ${selectedScenario.name}`}
                            draft={editDraft}
                            setDraft={setEditDraft}
                            onReset={() => setEditDraft(scenarioToDraft(selectedScenario))}
                            onSave={() => updateMutation.mutate(selectedScenario.id)}
                            saveLabel="Save Changes"
                            saving={updateMutation.isPending}
                        />
                    )}
                </div>
            </div>
        </div>
    )
}
