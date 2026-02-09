import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Plus, Trash2, AlertCircle } from 'lucide-react'
import Editor from '@monaco-editor/react'
import { responsesApi } from '../../services/api'
import type { ResponseConfig, Condition, ConditionOperator } from '../../types'

interface ResponseConfigEditorProps {
    operationId: string
    config: ResponseConfig | null
    onClose: () => void
}

const operators: { value: ConditionOperator; label: string }[] = [
    { value: 'eq', label: 'Equals' },
    { value: 'ne', label: 'Not Equals' },
    { value: 'contains', label: 'Contains' },
    { value: 'notContains', label: 'Not Contains' },
    { value: 'startsWith', label: 'Starts With' },
    { value: 'endsWith', label: 'Ends With' },
    { value: 'regex', label: 'Regex' },
    { value: 'exists', label: 'Exists' },
    { value: 'notExists', label: 'Not Exists' },
    { value: 'gt', label: 'Greater Than' },
    { value: 'lt', label: 'Less Than' },
    { value: 'gte', label: 'Greater or Equal' },
    { value: 'lte', label: 'Less or Equal' },
]

const sources = ['path', 'query', 'header', 'body'] as const

const templateDocs = {
    request: [
        { key: '{{path.param}}', desc: 'Replace param with any path parameter name', example: 'path.userId → 42' },
        { key: '{{query.param}}', desc: 'Replace param with any query parameter name (first value)', example: 'query.status → active' },
        { key: '{{header.name}}', desc: 'Replace name with any request header (case-insensitive)', example: 'header.Authorization → Bearer ...' },
        { key: '{{body.jsonPath}}', desc: 'Replace jsonPath with any JSON path in request body', example: 'body.user.name → Alice' },
    ],
    random: [
        { key: '{{random.uuid}}', desc: 'Random UUID', example: '3d7b9c2e-...' },
        { key: '{{random.int}}', desc: 'Random integer (0-999999)', example: '58231' },
        { key: '{{random.int(min,max)}}', desc: 'Random integer in range', example: 'random.int(1,10) → 7' },
        { key: '{{random.float}}', desc: 'Random float', example: '491.23' },
        { key: '{{random.float(min,max)}}', desc: 'Random float in range', example: 'random.float(1,5) → 3.14' },
        { key: '{{random.string}}', desc: 'Random string (len 10)', example: 'aZ93kLmP0q' },
        { key: '{{random.string(len)}}', desc: 'Random string (len)', example: 'random.string(6) → Kd9pQ2' },
        { key: '{{random.bool}}', desc: 'Random boolean', example: 'true' },
        { key: '{{random.email}}', desc: 'Random email', example: 'a1b2c3d4@example.com' },
        { key: '{{random.name}}', desc: 'Random name', example: 'Alice' },
        { key: '{{random.phone}}', desc: 'Random phone', example: '+1-415-555-0100' },
    ],
    faker: [
        { key: '{{faker.name.first}}', desc: 'First name', example: 'Liam' },
        { key: '{{faker.name.last}}', desc: 'Last name', example: 'Smith' },
        { key: '{{faker.name}}', desc: 'Full name', example: 'Olivia Johnson' },
        { key: '{{faker.email}}', desc: 'Email address', example: 'r2d2@mail.test' },
        { key: '{{faker.phone}}', desc: 'Phone number', example: '+1-212-555-0199' },
        { key: '{{faker.company.name}}', desc: 'Company name', example: 'Acme Corp' },
        { key: '{{faker.address.street}}', desc: 'Street address', example: '123 Oak St' },
        { key: '{{faker.address.city}}', desc: 'City', example: 'Springfield' },
        { key: '{{faker.address.state}}', desc: 'State', example: 'CA' },
        { key: '{{faker.address.zip}}', desc: 'Postal code', example: '94105' },
        { key: '{{faker.internet.username}}', desc: 'Username', example: 'alpha9delta' },
        { key: '{{faker.internet.domain}}', desc: 'Domain', example: 'mock.io' },
        { key: '{{faker.internet.url}}', desc: 'URL', example: 'https://mock.io/abc123' },
        { key: '{{faker.lorem.word}}', desc: 'Lorem word', example: 'bravo' },
        { key: '{{faker.lorem.sentence}}', desc: 'Lorem sentence', example: 'alpha bravo charlie.' },
        { key: '{{faker.lorem.paragraph}}', desc: 'Lorem paragraph', example: 'alpha bravo charlie. delta echo foxtrot.' },
    ],
    timestamp: [
        { key: '{{timestamp}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp.unix}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp.unixMilli}}', desc: 'Unix timestamp (ms)', example: '1707480000123' },
        { key: '{{timestamp.unixNano}}', desc: 'Unix timestamp (ns)', example: '1707480000123456789' },
        { key: '{{timestamp.iso}}', desc: 'ISO-8601 timestamp', example: '2026-02-09T12:00:00Z' },
        { key: '{{timestamp.date}}', desc: 'Date (YYYY-MM-DD)', example: '2026-02-09' },
        { key: '{{timestamp.time}}', desc: 'Time (HH:MM:SS)', example: '12:00:00' },
        { key: '{{timestamp.datetime}}', desc: 'Datetime (YYYY-MM-DD HH:MM:SS)', example: '2026-02-09 12:00:00' },
        { key: '{{timestamp.format(layout)}}', desc: 'Format using Go layout', example: 'timestamp.format(2006/01/02) → 2026/02/09' },
        { key: '{{timestamp.add(duration)}}', desc: 'Add duration (e.g., 1h, 30m)', example: 'timestamp.add(1h) → 2026-02-09T13:00:00Z' },
    ],
}

export default function ResponseConfigEditor({
    operationId,
    config,
    onClose,
}: ResponseConfigEditorProps) {
    const [name, setName] = useState(config?.name || '')
    const [description, setDescription] = useState(config?.description || '')
    const [statusCode, setStatusCode] = useState(config?.statusCode || 200)
    const [priority, setPriority] = useState(config?.priority || 0)
    const [delay, setDelay] = useState(config?.delay || 0)
    const [enabled, setEnabled] = useState(config?.enabled ?? true)
    const [conditions, setConditions] = useState<Condition[]>(config?.conditions || [])
    const [headers, setHeaders] = useState<Record<string, string>>(config?.headers || {})
    const [body, setBody] = useState(config?.body || '')
    const [error, setError] = useState('')
    const [headerKey, setHeaderKey] = useState('')
    const [headerValue, setHeaderValue] = useState('')

    const queryClient = useQueryClient()

    const createMutation = useMutation({
        mutationFn: (data: any) => responsesApi.create(operationId, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
        onError: (err: Error) => setError(err.message),
    })

    const updateMutation = useMutation({
        mutationFn: (data: any) => responsesApi.update(config!.id, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
        onError: (err: Error) => setError(err.message),
    })

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        if (!name.trim()) {
            setError('Name is required')
            return
        }

        const data = {
            name: name.trim(),
            description: description.trim(),
            statusCode,
            priority,
            delay,
            enabled,
            conditions,
            headers,
            body,
        }

        if (config) {
            updateMutation.mutate(data)
        } else {
            createMutation.mutate(data)
        }
    }

    const addCondition = () => {
        setConditions([
            ...conditions,
            { source: 'query', key: '', operator: 'eq', value: '' },
        ])
    }

    const updateCondition = (index: number, updates: Partial<Condition>) => {
        const newConditions = [...conditions]
        newConditions[index] = { ...newConditions[index], ...updates }
        setConditions(newConditions)
    }

    const removeCondition = (index: number) => {
        setConditions(conditions.filter((_, i) => i !== index))
    }

    const addHeader = () => {
        if (headerKey.trim()) {
            setHeaders({ ...headers, [headerKey.trim()]: headerValue })
            setHeaderKey('')
            setHeaderValue('')
        }
    }

    const removeHeader = (key: string) => {
        const newHeaders = { ...headers }
        delete newHeaders[key]
        setHeaders(newHeaders)
    }

    return (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white rounded-xl shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-hidden flex flex-col">
                <div className="flex items-center justify-between p-6 border-b border-gray-200">
                    <h2 className="text-xl font-semibold text-gray-900">
                        {config ? 'Edit Response Configuration' : 'New Response Configuration'}
                    </h2>
                    <button
                        onClick={onClose}
                        className="p-2 text-gray-400 hover:text-gray-600 rounded-lg hover:bg-gray-100"
                    >
                        <X className="w-5 h-5" />
                    </button>
                </div>

                <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
                    {error && (
                        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-start">
                            <AlertCircle className="w-5 h-5 text-red-600 mr-3 flex-shrink-0" />
                            <p className="text-red-700">{error}</p>
                        </div>
                    )}

                    {/* Basic Info */}
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Name *
                            </label>
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                                placeholder="Success Response"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Description
                            </label>
                            <input
                                type="text"
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                                placeholder="Returns when..."
                            />
                        </div>
                    </div>

                    {/* Status, Priority, Delay */}
                    <div className="grid grid-cols-4 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Status Code
                            </label>
                            <input
                                type="number"
                                value={statusCode}
                                onChange={(e) => setStatusCode(parseInt(e.target.value) || 200)}
                                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                                min={100}
                                max={599}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Priority
                            </label>
                            <input
                                type="number"
                                value={priority}
                                onChange={(e) => setPriority(parseInt(e.target.value) || 0)}
                                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                                min={0}
                            />
                            <p className="text-xs text-gray-400 mt-1">Lower = higher priority</p>
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Delay (ms)
                            </label>
                            <input
                                type="number"
                                value={delay}
                                onChange={(e) => setDelay(parseInt(e.target.value) || 0)}
                                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                                min={0}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 mb-1">
                                Enabled
                            </label>
                            <button
                                type="button"
                                onClick={() => setEnabled(!enabled)}
                                className={`w-full px-3 py-2 rounded-lg border ${enabled
                                    ? 'bg-green-50 border-green-300 text-green-700'
                                    : 'bg-gray-50 border-gray-300 text-gray-500'
                                    }`}
                            >
                                {enabled ? 'Yes' : 'No'}
                            </button>
                        </div>
                    </div>

                    {/* Conditions */}
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <label className="text-sm font-medium text-gray-700">Conditions</label>
                            <button
                                type="button"
                                onClick={addCondition}
                                className="text-sm text-primary-600 hover:text-primary-700 flex items-center"
                            >
                                <Plus className="w-4 h-4 mr-1" />
                                Add Condition
                            </button>
                        </div>
                        <p className="text-xs text-gray-400 mb-3">
                            All conditions must match (AND logic)
                        </p>
                        <div className="space-y-2">
                            {conditions.map((cond, index) => (
                                <div key={index} className="flex items-center gap-2">
                                    <select
                                        value={cond.source}
                                        onChange={(e) =>
                                            updateCondition(index, { source: e.target.value as any })
                                        }
                                        className="px-2 py-1.5 border border-gray-300 rounded text-sm"
                                    >
                                        {sources.map((s) => (
                                            <option key={s} value={s}>
                                                {s}
                                            </option>
                                        ))}
                                    </select>
                                    <input
                                        type="text"
                                        value={cond.key}
                                        onChange={(e) => updateCondition(index, { key: e.target.value })}
                                        placeholder="key"
                                        className="flex-1 px-2 py-1.5 border border-gray-300 rounded text-sm"
                                    />
                                    <select
                                        value={cond.operator}
                                        onChange={(e) =>
                                            updateCondition(index, { operator: e.target.value as any })
                                        }
                                        className="px-2 py-1.5 border border-gray-300 rounded text-sm"
                                    >
                                        {operators.map((op) => (
                                            <option key={op.value} value={op.value}>
                                                {op.label}
                                            </option>
                                        ))}
                                    </select>
                                    <input
                                        type="text"
                                        value={cond.value}
                                        onChange={(e) => updateCondition(index, { value: e.target.value })}
                                        placeholder="value"
                                        className="flex-1 px-2 py-1.5 border border-gray-300 rounded text-sm"
                                        disabled={cond.operator === 'exists' || cond.operator === 'notExists'}
                                    />
                                    <button
                                        type="button"
                                        onClick={() => removeCondition(index)}
                                        className="p-1.5 text-gray-400 hover:text-red-600 rounded"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                    </button>
                                </div>
                            ))}
                        </div>
                    </div>

                    {/* Headers */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Response Headers
                        </label>
                        <div className="flex gap-2 mb-2">
                            <input
                                type="text"
                                value={headerKey}
                                onChange={(e) => setHeaderKey(e.target.value)}
                                placeholder="Header name"
                                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm"
                            />
                            <input
                                type="text"
                                value={headerValue}
                                onChange={(e) => setHeaderValue(e.target.value)}
                                placeholder="Header value"
                                className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm"
                            />
                            <button
                                type="button"
                                onClick={addHeader}
                                className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200"
                            >
                                Add
                            </button>
                        </div>
                        {Object.entries(headers).length > 0 && (
                            <div className="bg-gray-50 rounded-lg p-3 space-y-1">
                                {Object.entries(headers).map(([key, value]) => (
                                    <div key={key} className="flex items-center justify-between text-sm">
                                        <span>
                                            <span className="font-medium">{key}:</span> {value}
                                        </span>
                                        <button
                                            type="button"
                                            onClick={() => removeHeader(key)}
                                            className="text-gray-400 hover:text-red-600"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>

                    {/* Body */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                            Response Body
                        </label>
                        <p className="text-xs text-gray-400 mb-2">
                            Use {'{{path.param}}'}, {'{{query.param}}'}, {'{{header.name}}'}, {'{{body.path}}'},
                            {'{{random.uuid}}'}, {'{{timestamp}}'} for templates
                        </p>
                        <div className="border border-gray-300 rounded-lg overflow-hidden">
                            <Editor
                                height="200px"
                                defaultLanguage="json"
                                value={body}
                                onChange={(value) => setBody(value || '')}
                                options={{
                                    minimap: { enabled: false },
                                    fontSize: 13,
                                    lineNumbers: 'off',
                                    folding: false,
                                    scrollBeyondLastLine: false,
                                }}
                            />
                        </div>
                    </div>

                    {/* Template Documentation */}
                    <div className="bg-gradient-to-br from-indigo-50 via-white to-sky-50 border border-indigo-100 rounded-xl p-5 shadow-sm">
                        <details className="group">
                            <summary className="cursor-pointer text-sm font-semibold text-indigo-700 flex items-center justify-between">
                                Template variables reference
                                <span className="text-xs text-indigo-400 group-open:rotate-180 transition-transform">▾</span>
                            </summary>
                            <p className="text-xs text-indigo-500 mt-2">
                                Faker values are deterministically seeded per request using path and query parameters.
                            </p>
                            <div className="mt-4 space-y-5">
                                <div className="bg-white/70 border border-indigo-100 rounded-lg p-4">
                                    <h4 className="text-xs font-semibold text-indigo-600 uppercase tracking-wide mb-3">
                                        Request data
                                    </h4>
                                    <ul className="space-y-3 text-xs text-gray-700">
                                        {templateDocs.request.map((item) => (
                                            <li key={item.key} className="flex flex-col gap-1">
                                                <span className="font-mono text-indigo-900">{item.key}</span>
                                                <span className="text-gray-600">{item.desc}</span>
                                                <span className="text-indigo-500/90">Example: {item.example}</span>
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                                <div className="bg-white/70 border border-emerald-100 rounded-lg p-4">
                                    <h4 className="text-xs font-semibold text-emerald-600 uppercase tracking-wide mb-3">
                                        Random
                                    </h4>
                                    <ul className="space-y-3 text-xs text-gray-700">
                                        {templateDocs.random.map((item) => (
                                            <li key={item.key} className="flex flex-col gap-1">
                                                <span className="font-mono text-emerald-900">{item.key}</span>
                                                <span className="text-gray-600">{item.desc}</span>
                                                <span className="text-emerald-500/90">Example: {item.example}</span>
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                                <div className="bg-white/70 border border-amber-100 rounded-lg p-4">
                                    <h4 className="text-xs font-semibold text-amber-600 uppercase tracking-wide mb-3">
                                        Faker
                                    </h4>
                                    <ul className="space-y-3 text-xs text-gray-700">
                                        {templateDocs.faker.map((item) => (
                                            <li key={item.key} className="flex flex-col gap-1">
                                                <span className="font-mono text-amber-900">{item.key}</span>
                                                <span className="text-gray-600">{item.desc}</span>
                                                <span className="text-amber-500/90">Example: {item.example}</span>
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                                <div className="bg-white/70 border border-sky-100 rounded-lg p-4">
                                    <h4 className="text-xs font-semibold text-sky-600 uppercase tracking-wide mb-3">
                                        Timestamp
                                    </h4>
                                    <ul className="space-y-3 text-xs text-gray-700">
                                        {templateDocs.timestamp.map((item) => (
                                            <li key={item.key} className="flex flex-col gap-1">
                                                <span className="font-mono text-sky-900">{item.key}</span>
                                                <span className="text-gray-600">{item.desc}</span>
                                                <span className="text-sky-500/90">Example: {item.example}</span>
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                            </div>
                        </details>
                    </div>
                </form>

                {/* Actions */}
                <div className="flex justify-end gap-4 p-6 border-t border-gray-200">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={handleSubmit}
                        disabled={createMutation.isPending || updateMutation.isPending}
                        className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50"
                    >
                        {createMutation.isPending || updateMutation.isPending
                            ? 'Saving...'
                            : config
                                ? 'Update'
                                : 'Create'}
                    </button>
                </div>
            </div>
        </div>
    )
}
