import { useCallback, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { X, Plus, Trash2, AlertCircle, Wand2 } from 'lucide-react'
import Editor from '@monaco-editor/react'
import type * as Monaco from 'monaco-editor'
import { responsesApi, scriptBindingsApi, tagsApi, templatesApi } from '../../services/api'
import type { ResponseConfig, Condition, ConditionOperator, ScriptBinding } from '../../types'

interface ResponseConfigEditorProps {
    operationId: string
    config: ResponseConfig | null
    onClose: () => void
    variant?: 'modal' | 'page'
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

const sources = ['path', 'query', 'header', 'body', 'signature'] as const

const templateDocs = {
    request: [
        { key: '{{.Path.id}}', desc: 'Path parameters map (use .Path.<name>)', example: '.Path.userId → 42' },
        { key: '{{index .Query "status"}}', desc: 'Query params (lowercased keys, first value)', example: 'query status → active' },
        { key: '{{index .Header "authorization"}}', desc: 'Headers (lowercased keys, first value)', example: 'authorization → Bearer ...' },
        { key: '{{body "user.name"}}', desc: 'JSON path from request body', example: 'user.name → Alice' },
    ],
    control: [
        { key: '{{if (eq (index .Query "status") "active")}}...{{else}}...{{end}}', desc: 'Conditional rendering with comparisons', example: 'Show block when status=active' },
        { key: '{{if (index .Header "x-feature")}}...{{end}}', desc: 'Truthy checks (non-empty values)', example: 'Feature flag header present' },
        { key: '{{range $k, $v := .Query}}...{{end}}', desc: 'Loop over query params (lowercased keys)', example: 'Render all query key/value pairs' },
        { key: '{{range $k, $v := .Header}}...{{end}}', desc: 'Loop over headers (lowercased keys)', example: 'Render all header key/value pairs' },
        { key: '{{with .Path}}...{{end}}', desc: 'Scoped block when path params exist', example: 'Use .Path inside block' },
    ],
    random: [
        { key: '{{random "uuid"}}', desc: 'Random UUID', example: '3d7b9c2e-...' },
        { key: '{{random "int"}}', desc: 'Random integer (0-999999)', example: '58231' },
        { key: '{{random "int" 1 10}}', desc: 'Random integer in range', example: 'random 1..10 → 7' },
        { key: '{{random "float"}}', desc: 'Random float', example: '491.23' },
        { key: '{{random "float" 1 5}}', desc: 'Random float in range', example: 'random 1..5 → 3.14' },
        { key: '{{random "string"}}', desc: 'Random string (len 10)', example: 'aZ93kLmP0q' },
        { key: '{{random "string" 6}}', desc: 'Random string (len)', example: 'len 6 → Kd9pQ2' },
        { key: '{{random "bool"}}', desc: 'Random boolean', example: 'true' },
        { key: '{{random "email"}}', desc: 'Random email', example: 'a1b2c3d4@example.com' },
        { key: '{{random "name"}}', desc: 'Random name', example: 'Alice' },
        { key: '{{random "phone"}}', desc: 'Random phone', example: '+1-415-555-0100' },
    ],
    faker: [
        { key: '{{faker "name.first"}}', desc: 'First name', example: 'Liam' },
        { key: '{{faker "name.last"}}', desc: 'Last name', example: 'Smith' },
        { key: '{{faker "name"}}', desc: 'Full name', example: 'Olivia Johnson' },
        { key: '{{faker "email"}}', desc: 'Email address', example: 'r2d2@mail.test' },
        { key: '{{faker "phone"}}', desc: 'Phone number', example: '+1-212-555-0199' },
        { key: '{{faker "company.name"}}', desc: 'Company name', example: 'Acme Corp' },
        { key: '{{faker "address.street"}}', desc: 'Street address', example: '123 Oak St' },
        { key: '{{faker "address.city"}}', desc: 'City', example: 'Springfield' },
        { key: '{{faker "address.state"}}', desc: 'State', example: 'CA' },
        { key: '{{faker "address.zip"}}', desc: 'Postal code', example: '94105' },
        { key: '{{faker "internet.username"}}', desc: 'Username', example: 'alpha9delta' },
        { key: '{{faker "internet.domain"}}', desc: 'Domain', example: 'mock.io' },
        { key: '{{faker "internet.url"}}', desc: 'URL', example: 'https://mock.io/abc123' },
        { key: '{{faker "lorem.word"}}', desc: 'Lorem word', example: 'bravo' },
        { key: '{{faker "lorem.sentence"}}', desc: 'Lorem sentence', example: 'alpha bravo charlie.' },
        { key: '{{faker "lorem.paragraph"}}', desc: 'Lorem paragraph', example: 'alpha bravo charlie. delta echo foxtrot.' },
    ],
    timestamp: [
        { key: '{{timestamp}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp "unix"}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp "unixMilli"}}', desc: 'Unix timestamp (ms)', example: '1707480000123' },
        { key: '{{timestamp "unixNano"}}', desc: 'Unix timestamp (ns)', example: '1707480000123456789' },
        { key: '{{timestamp "iso"}}', desc: 'ISO-8601 timestamp', example: '2026-02-09T12:00:00Z' },
        { key: '{{timestamp "date"}}', desc: 'Date (YYYY-MM-DD)', example: '2026-02-09' },
        { key: '{{timestamp "time"}}', desc: 'Time (HH:MM:SS)', example: '12:00:00' },
        { key: '{{timestamp "datetime"}}', desc: 'Datetime (YYYY-MM-DD HH:MM:SS)', example: '2026-02-09 12:00:00' },
        { key: '{{timestamp "format" "2006/01/02"}}', desc: 'Format using Go layout', example: 'format 2006/01/02 → 2026/02/09' },
        { key: '{{timestamp "add" "1h"}}', desc: 'Add duration (e.g., 1h, 30m)', example: 'add 1h → 2026-02-09T13:00:00Z' },
    ],
}

export default function ResponseConfigEditor({
    operationId,
    config,
    onClose,
    variant = 'modal',
}: ResponseConfigEditorProps) {
    const [name, setName] = useState(config?.name || '')
    const [description, setDescription] = useState(config?.description || '')
    const [statusCode, setStatusCode] = useState(config?.statusCode || 200)
    const [priority, setPriority] = useState(config?.priority || 0)
    const [delay, setDelay] = useState(config?.delay || 0)
    const [enabled, setEnabled] = useState(config?.enabled ?? true)
    const [tag, setTag] = useState(config?.tag || 'default')
    const [conditions, setConditions] = useState<Condition[]>(config?.conditions || [])
    const [headers, setHeaders] = useState<Record<string, string>>(config?.headers || {})
    const [body, setBody] = useState(config?.body || '')
    const [error, setError] = useState('')
    const [bodyError, setBodyError] = useState('')
    const [isValidating, setIsValidating] = useState(false)
    const [headerKey, setHeaderKey] = useState('')
    const [headerValue, setHeaderValue] = useState('')
    const editorRef = useRef<Monaco.editor.IStandaloneCodeEditor | null>(null)
    const monacoRef = useRef<typeof Monaco | null>(null)
    const validationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

    const queryClient = useQueryClient()

    const { data: tags } = useQuery({
        queryKey: ['tags'],
        queryFn: tagsApi.list,
    })

    const { data: scriptBindings } = useQuery<ScriptBinding[]>({
        queryKey: ['scriptBindings', operationId],
        queryFn: () => scriptBindingsApi.listByOperation(operationId),
        enabled: !!operationId,
    })

    const tagOptions = (tags && tags.length > 0) ? tags : [{ name: 'default' }]

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

    const parseTemplateErrorLine = (message: string) => {
        const match = message.match(/:(\d+):/)
        if (match) {
            return Number(match[1]) || 1
        }
        return 1
    }

    const registerTemplateLanguage = useCallback((monaco: typeof Monaco) => {
        const exists = monaco.languages.getLanguages().some((lang) => lang.id === 'go-template')
        if (exists) {
            return
        }

        monaco.languages.register({ id: 'go-template' })
        monaco.languages.setMonarchTokensProvider('go-template', {
            tokenizer: {
                root: [
                    [/\{\{/, { token: 'keyword', next: '@template' }],
                    [/[^{}]+/, ''],
                ],
                template: [
                    [/\}\}/, { token: 'keyword', next: '@root' }],
                    [/[^}]+/, 'keyword'],
                ],
            },
        })
    }, [])

    const validateBodyTemplate = useCallback(async (value: string) => {
        const editor = editorRef.current
        const monaco = monacoRef.current
        const model = editor?.getModel()

        const clearMarkers = () => {
            if (monaco && model) {
                monaco.editor.setModelMarkers(model, 'template-validation', [])
            }
        }

        const trimmed = value.trim()
        if (!trimmed) {
            setBodyError('')
            clearMarkers()
            setIsValidating(false)
            return true
        }

        setIsValidating(true)
        const result = await templatesApi.validate(value)
        setIsValidating(false)

        if (result.valid) {
            setBodyError('')
            clearMarkers()
            return true
        }

        const message = result.error || 'Invalid template'
        setBodyError(message)

        if (monaco && model) {
            const line = parseTemplateErrorLine(message)
            monaco.editor.setModelMarkers(model, 'template-validation', [
                {
                    severity: monaco.MarkerSeverity.Error,
                    message,
                    startLineNumber: line,
                    startColumn: 1,
                    endLineNumber: line,
                    endColumn: 1,
                },
            ])
        }

        return false
    }, [])

    const scheduleTemplateValidation = useCallback((value: string) => {
        if (validationTimeoutRef.current) {
            clearTimeout(validationTimeoutRef.current)
        }

        validationTimeoutRef.current = setTimeout(() => {
            void validateBodyTemplate(value)
        }, 300)
    }, [validateBodyTemplate])

    const prettifyBody = useCallback(() => {
        const trimmed = body.trim()
        if (!trimmed) return
        try {
            const pretty = JSON.stringify(JSON.parse(trimmed), null, 2)
            setBody(pretty)
            scheduleTemplateValidation(pretty)
        } catch {
            // Not valid JSON (e.g. contains template tags) — leave as-is
        }
    }, [body, scheduleTemplateValidation])

    useEffect(() => {
        if (body.trim()) {
            scheduleTemplateValidation(body)
        } else {
            void validateBodyTemplate(body)
        }
        return () => {
            if (validationTimeoutRef.current) {
                clearTimeout(validationTimeoutRef.current)
            }
        }
    }, [body, scheduleTemplateValidation, validateBodyTemplate])

    useEffect(() => {
        if (!config) {
            return
        }
        setName(config.name || '')
        setDescription(config.description || '')
        setStatusCode(config.statusCode || 200)
        setPriority(config.priority || 0)
        setDelay(config.delay || 0)
        setEnabled(config.enabled ?? true)
        setTag(config.tag || 'default')
        setConditions(config.conditions || [])
        setHeaders(config.headers || {})
        setBody(config.body || '')
    }, [config])

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')

        if (!name.trim()) {
            setError('Name is required')
            return
        }

        if (isValidating) {
            setError('Wait for template validation to finish.')
            return
        }

        if (!await validateBodyTemplate(body)) {
            setError('Fix response body template before saving.')
            return
        }

        const data = {
            name: name.trim(),
            description: description.trim(),
            tag,
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
        const nextCondition = { ...newConditions[index], ...updates }

        if (updates.source === 'signature') {
            nextCondition.key = ''
        }

        newConditions[index] = nextCondition
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

    const isModal = variant === 'modal'
    const wrapperClass = isModal
        ? 'fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50'
        : 'w-full'
    const containerClass = isModal
        ? 'bg-white dark:bg-slate-900 rounded-xl shadow-xl max-w-4xl w-full mx-4 max-h-[90vh] overflow-hidden flex flex-col'
        : 'bg-white dark:bg-slate-900 rounded-2xl shadow-sm border border-gray-200 dark:border-slate-800 w-full flex flex-col'

    return (
        <div className={wrapperClass}>
            <div className={containerClass}>
                <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-slate-800">
                    <h2 className="text-xl font-semibold text-gray-900 dark:text-slate-100">
                        {config ? 'Edit Response Configuration' : 'New Response Configuration'}
                    </h2>
                    {isModal && (
                        <button
                            onClick={onClose}
                            className="p-2 text-gray-400 hover:text-gray-600 dark:text-slate-500 dark:hover:text-slate-200 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800"
                        >
                            <X className="w-5 h-5" />
                        </button>
                    )}
                </div>

                <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
                    {error && (
                        <div className="bg-red-50 border border-red-200 rounded-lg p-4 flex items-start dark:bg-red-950/40 dark:border-red-900/40">
                            <AlertCircle className="w-5 h-5 text-red-600 mr-3 flex-shrink-0" />
                            <p className="text-red-700 dark:text-red-300">{error}</p>
                        </div>
                    )}

                    {/* Basic Info */}
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Name *
                            </label>
                            <input
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                placeholder="Success Response"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Description
                            </label>
                            <input
                                type="text"
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                placeholder="Returns when..."
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Tag
                            </label>
                            <select
                                value={tag}
                                onChange={(e) => setTag(e.target.value)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                            >
                                {tagOptions.map((t: { name: string }) => (
                                    <option key={t.name} value={t.name}>
                                        {t.name}
                                    </option>
                                ))}
                            </select>
                            <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">
                                Tags are managed globally and enabled per spec.
                            </p>
                        </div>
                    </div>

                    {/* Status, Priority, Delay */}
                    <div className="grid grid-cols-4 gap-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Status Code
                            </label>
                            <input
                                type="number"
                                value={statusCode}
                                onChange={(e) => setStatusCode(parseInt(e.target.value) || 200)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                min={100}
                                max={599}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Priority
                            </label>
                            <input
                                type="number"
                                value={priority}
                                onChange={(e) => setPriority(parseInt(e.target.value) || 0)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                min={0}
                            />
                            <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">Lower = higher priority</p>
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Delay (ms)
                            </label>
                            <input
                                type="number"
                                value={delay}
                                onChange={(e) => setDelay(parseInt(e.target.value) || 0)}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                min={0}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Enabled
                            </label>
                            <button
                                type="button"
                                onClick={() => setEnabled(!enabled)}
                                className={`w-full px-3 py-2 rounded-lg border ${enabled
                                    ? 'bg-green-50 border-green-300 text-green-700 dark:bg-green-950/40 dark:border-green-900/50 dark:text-green-300'
                                    : 'bg-gray-50 border-gray-300 text-gray-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-400'
                                    }`}
                            >
                                {enabled ? 'Yes' : 'No'}
                            </button>
                        </div>
                    </div>

                    {/* Conditions */}
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <label className="text-sm font-medium text-gray-700 dark:text-slate-300">Conditions</label>
                            <button
                                type="button"
                                onClick={addCondition}
                                className="text-sm text-primary-600 hover:text-primary-700 flex items-center"
                            >
                                <Plus className="w-4 h-4 mr-1" />
                                Add Condition
                            </button>
                        </div>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mb-3">
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
                                        className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
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
                                        placeholder={cond.source === 'signature' ? 'computed request signature' : 'key'}
                                        className="flex-1 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                        disabled={cond.source === 'signature'}
                                    />
                                    <select
                                        value={cond.operator}
                                        onChange={(e) =>
                                            updateCondition(index, { operator: e.target.value as any })
                                        }
                                        className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
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
                                        className="flex-1 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                                        disabled={cond.operator === 'exists' || cond.operator === 'notExists'}
                                    />
                                    <button
                                        type="button"
                                        onClick={() => removeCondition(index)}
                                        className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                    </button>
                                </div>
                            ))}
                        </div>
                        {conditions.some((cond) => cond.source === 'signature') && (
                            <p className="mt-2 text-xs text-violet-600 dark:text-violet-400">
                                Recorded responses use a computed request signature hash, so the key is fixed and only the hash value is stored.
                            </p>
                        )}
                    </div>

                    {/* Headers */}
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">
                            Response Headers
                        </label>
                        <div className="flex gap-2 mb-2">
                            <input
                                type="text"
                                value={headerKey}
                                onChange={(e) => setHeaderKey(e.target.value)}
                                placeholder="Header name"
                                className="flex-1 px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                            />
                            <input
                                type="text"
                                value={headerValue}
                                onChange={(e) => setHeaderValue(e.target.value)}
                                placeholder="Header value"
                                className="flex-1 px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
                            />
                            <button
                                type="button"
                                onClick={addHeader}
                                className="px-4 py-2 bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-200 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-700"
                            >
                                Add
                            </button>
                        </div>
                        {Object.entries(headers).length > 0 && (
                            <div className="bg-gray-50 dark:bg-slate-800 rounded-lg p-3 space-y-1">
                                {Object.entries(headers).map(([key, value]) => (
                                    <div key={key} className="flex items-center justify-between text-sm">
                                        <span>
                                            <span className="font-medium text-gray-900 dark:text-slate-100">{key}:</span> <span className="text-gray-700 dark:text-slate-300">{value}</span>
                                        </span>
                                        <button
                                            type="button"
                                            onClick={() => removeHeader(key)}
                                            className="text-gray-400 dark:text-slate-500 hover:text-red-600"
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
                        <div className="flex items-center justify-between mb-2">
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300">
                                Response Body
                            </label>
                            <button
                                type="button"
                                onClick={prettifyBody}
                                title="Prettify JSON"
                                className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-slate-300 bg-gray-100 dark:bg-slate-800 hover:bg-gray-200 dark:hover:bg-slate-700 border border-gray-300 dark:border-slate-600 rounded-md transition-colors"
                            >
                                <Wand2 className="w-3.5 h-3.5" />
                                Prettify
                            </button>
                        </div>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mb-2">
                            Body templates use Go text/template. Try {'{{.Path.id}}'},
                            {'{{index .Query "status"}}'}, {'{{index .Header "authorization"}}'},
                            {'{{body "user.name"}}'}, {'{{random "uuid"}}'}, {'{{timestamp}}'}.
                            Header and query keys are lowercased.
                        </p>
                        <div
                            className={`border rounded-lg overflow-hidden ${bodyError
                                ? 'border-red-300 dark:border-red-700'
                                : 'border-gray-300 dark:border-slate-700'
                                }`}
                        >
                            <Editor
                                height="200px"
                                defaultLanguage="go-template"
                                value={body}
                                onChange={(value) => {
                                    const next = value || ''
                                    setBody(next)
                                    scheduleTemplateValidation(next)
                                }}
                                onMount={(editor, monaco) => {
                                    editorRef.current = editor
                                    monacoRef.current = monaco
                                    registerTemplateLanguage(monaco)
                                    const model = editor.getModel()
                                    if (model) {
                                        monaco.editor.setModelLanguage(model, 'go-template')
                                    }
                                    validateBodyTemplate(body)
                                }}
                                options={{
                                    minimap: { enabled: false },
                                    fontSize: 13,
                                    lineNumbers: 'off',
                                    folding: false,
                                    scrollBeyondLastLine: false,
                                    theme: document.documentElement.classList.contains('dark')
                                        ? 'vs-dark'
                                        : 'light',
                                }}
                            />
                        </div>
                        {bodyError && (
                            <div className="mt-2 flex items-start text-xs text-red-600 dark:text-red-300">
                                <AlertCircle className="w-4 h-4 mr-2 mt-0.5 flex-shrink-0" />
                                <span>{bodyError}</span>
                            </div>
                        )}
                        {isValidating && !bodyError && (
                            <div className="mt-2 text-xs text-gray-500 dark:text-slate-400">
                                Validating template...
                            </div>
                        )}
                    </div>

                </form>

                {/* Actions */}
                <div className="flex justify-end gap-4 p-6 border-t border-gray-200 dark:border-slate-800">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-4 py-2 text-gray-700 dark:text-slate-200 hover:bg-gray-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
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

                {/* Template Documentation */}
                <div className="bg-gradient-to-br from-indigo-50 via-white to-sky-50 border border-indigo-100 dark:border-slate-800 dark:from-slate-950 dark:via-slate-900 dark:to-slate-900 rounded-xl p-5 shadow-sm mx-6 mb-6">
                    <details className="group">
                        <summary className="cursor-pointer text-sm font-semibold text-indigo-700 dark:text-slate-200 flex items-center justify-between">
                            Template variables reference
                            <span className="text-xs text-indigo-400 dark:text-slate-400 group-open:rotate-180 transition-transform">▾</span>
                        </summary>
                        <p className="text-xs text-indigo-500 dark:text-slate-400 mt-2">
                            Body templates use Go text/template. Query and header keys are lowercased before lookup.
                        </p>
                        <div className="mt-4 grid grid-cols-1 lg:grid-cols-2 gap-5">
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-indigo-100 dark:border-slate-800 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-indigo-600 dark:text-indigo-300 uppercase tracking-wide mb-3">
                                    Request data
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.request.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-indigo-900 dark:text-indigo-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-indigo-500/90 dark:text-indigo-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-purple-100 dark:border-purple-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-purple-600 dark:text-purple-300 uppercase tracking-wide mb-3">
                                    Control flow (loops & conditions)
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.control.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-purple-900 dark:text-purple-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-purple-500/90 dark:text-purple-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-emerald-100 dark:border-emerald-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-emerald-600 dark:text-emerald-300 uppercase tracking-wide mb-3">
                                    Random
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.random.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-emerald-900 dark:text-emerald-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-emerald-500/90 dark:text-emerald-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-amber-100 dark:border-amber-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-amber-600 dark:text-amber-300 uppercase tracking-wide mb-3">
                                    Faker
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.faker.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-amber-900 dark:text-amber-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-amber-500/90 dark:text-amber-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-sky-100 dark:border-sky-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-sky-600 dark:text-sky-300 uppercase tracking-wide mb-3">
                                    Timestamp
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.timestamp.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-sky-900 dark:text-sky-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-sky-500/90 dark:text-sky-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            {scriptBindings && scriptBindings.length > 0 && (
                                <div className="bg-white/70 dark:bg-slate-900/70 border border-emerald-200 dark:border-emerald-900/40 rounded-lg p-4 lg:col-span-2">
                                    <h4 className="text-xs font-semibold text-emerald-600 dark:text-emerald-300 uppercase tracking-wide mb-3">
                                        Script Variables (from bound scripts)
                                    </h4>
                                    <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300 columns-2">
                                        {scriptBindings
                                            .slice()
                                            .sort((a, b) => a.order - b.order)
                                            .map((binding) => (
                                                <li key={binding.id} className="flex flex-col gap-1 break-inside-avoid">
                                                    <span className="font-mono text-emerald-900 dark:text-emerald-200">
                                                        {`{{script "${binding.outputKey}"}}`}
                                                    </span>
                                                    <span className="text-gray-600 dark:text-slate-300">
                                                        Full output of <strong>{binding.scriptName || binding.scriptId}</strong>
                                                    </span>
                                                    <span className="font-mono text-emerald-700/80 dark:text-emerald-400/80">
                                                        {`{{script "${binding.outputKey}.fieldName"}}`} — nested field
                                                    </span>
                                                </li>
                                            ))}
                                    </ul>
                                    <p className="mt-3 text-xs text-gray-400 dark:text-slate-500">
                                        Scripts run in order before the response is rendered. Manage bindings in the operation's Script Bindings section.
                                    </p>
                                </div>
                            )}
                        </div>
                    </details>
                </div>
            </div>
        </div>
    )
}
