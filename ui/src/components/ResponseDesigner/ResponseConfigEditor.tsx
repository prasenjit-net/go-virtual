import { useCallback, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { X, Plus, Trash2, AlertCircle, Wand2, Zap } from 'lucide-react'
import Editor from '@monaco-editor/react'
import type * as Monaco from 'monaco-editor'
import { collectionMappingsApi, conditionsApi, responsesApi, scriptBindingsApi, tagsApi, templatesApi } from '../../services/api'
import type { Condition, ConditionOperator, CollectionMappingInput, ResponseConfig, ResponseConfigInput, ScriptBinding, SpecExample } from '../../types'
import ScriptBindingsPanel from '../ScriptManager/ScriptBindingsPanel'
import CollectionMappingsPanel from '../CollectionMapper/CollectionMappingsPanel'
import { useIsDark } from '../../hooks/useIsDark'
import ExamplePickerModal from './ExamplePickerModal'

interface ResponseConfigEditorProps {
    operationId: string
    config: ResponseConfig | null
    onClose: () => void
    variant?: 'modal' | 'page'
    readOnly?: boolean
}

const operators: { value: ConditionOperator; label: string; group?: string }[] = [
    { value: 'eq', label: 'Equals' },
    { value: 'contains', label: 'Contains' },
    { value: 'startsWith', label: 'Starts With' },
    { value: 'endsWith', label: 'Ends With' },
    { value: 'regex', label: 'Regex' },
    { value: 'exists', label: 'Exists' },
    { value: 'gt', label: 'Greater Than' },
    { value: 'lt', label: 'Less Than' },
    { value: 'gte', label: 'Greater or Equal' },
    { value: 'lte', label: 'Less or Equal' },
    // Date operators
    { value: 'dateEq', label: 'Date Equals', group: 'date' },
    { value: 'dateBefore', label: 'Date Before', group: 'date' },
    { value: 'dateAfter', label: 'Date After', group: 'date' },
    { value: 'dateLte', label: 'Date ≤', group: 'date' },
    { value: 'dateGte', label: 'Date ≥', group: 'date' },
    { value: 'dateInPast', label: 'Date In Past', group: 'date' },
    { value: 'dateInFuture', label: 'Date In Future', group: 'date' },
    { value: 'dateToday', label: 'Date Is Today', group: 'date' },
    { value: 'dateBetween', label: 'Date Between', group: 'date' },
]

/** Normalise deprecated operators from stored conditions to new form */
function normaliseCondition(c: Condition): Condition {
    if ((c.operator as string) === 'ne') return { ...c, operator: 'eq', negate: !c.negate }
    if ((c.operator as string) === 'notContains') return { ...c, operator: 'contains', negate: !c.negate }
    if ((c.operator as string) === 'notExists') return { ...c, operator: 'exists', negate: !c.negate }
    return c
}

const DATE_OPERATORS = new Set<ConditionOperator>([
    'dateEq', 'dateBefore', 'dateAfter', 'dateLte', 'dateGte',
    'dateInPast', 'dateInFuture', 'dateToday', 'dateBetween',
])
const DATE_NO_VALUE_OPERATORS = new Set<ConditionOperator>(['dateInPast', 'dateInFuture', 'dateToday'])

/** Named date tokens offered as autocomplete suggestions */
const DATE_TOKENS: { token: string; description: string }[] = [
    { token: 'today',      description: 'Current date at midnight' },
    { token: 'now',        description: 'Current date and time' },
    { token: 'yesterday',  description: 'Yesterday at midnight' },
    { token: 'tomorrow',   description: 'Tomorrow at midnight' },
    { token: 'now+1d',     description: '1 day from now' },
    { token: 'now-1d',     description: '1 day ago' },
    { token: 'now+7d',     description: '7 days from now' },
    { token: 'now-7d',     description: '7 days ago' },
    { token: 'now+14d',    description: '14 days from now' },
    { token: 'now+30d',    description: '30 days from now' },
    { token: 'now-30d',    description: '30 days ago' },
    { token: 'now+90d',    description: '90 days from now' },
    { token: 'now+365d',   description: '1 year from now' },
    { token: 'now+1h',     description: '1 hour from now' },
    { token: 'now-1h',     description: '1 hour ago' },
    { token: 'now+6h',     description: '6 hours from now' },
    { token: 'now+24h',    description: '24 hours from now' },
    { token: 'now+1n',     description: '1 minute from now' },
    { token: 'now-1n',     description: '1 minute ago' },
]

const sources = ['path', 'query', 'header', 'body', 'signature', 'script'] as const

const templateDocs = {
    request: [
        { key: '{{.Path.id}}', desc: 'Path parameter (dot-notation)', example: '.Path.userId → 42' },
        { key: '{{.Query.status}}', desc: 'Query parameter (dot-notation)', example: '.Query.page → 2' },
        { key: '{{.Header.authorization}}', desc: 'Request header (lowercased key)', example: 'Bearer ...' },
        { key: '{{.Body.user.name}}', desc: 'JSON body field (native dot-traversal)', example: 'user.name → Alice' },
        { key: '{{body "items.0.id"}}', desc: 'JSON body via gjson path (arrays/complex)', example: 'items.0.id → 99' },
        { key: '{{.RawBody}}', desc: 'Entire raw request body string', example: '{"name":"alice"}' },
        { key: '{{.Method}}', desc: 'HTTP method of the request', example: 'POST' },
        { key: '{{.URL}}', desc: 'Full request URL string', example: '/api/v1/pets?page=2' },
        { key: '{{.RequestID}}', desc: 'Stable UUID for this request', example: '3d7b9c2e-...' },
        { key: '{{store "key"}}', desc: 'Read from session store', example: 'store "userId" → abc' },
        { key: '{{counter "hits"}}', desc: 'Increment session counter, return value', example: '1, 2, 3 ...' },
        // Legacy function-call style (still works)
        { key: '{{path "id"}}', desc: 'Path param (legacy style)', example: '42' },
        { key: '{{query "status"}}', desc: 'Query param (legacy style)', example: 'active' },
        { key: '{{header "authorization"}}', desc: 'Header value (legacy style)', example: 'Bearer ...' },
    ],
    control: [
        { key: '{{if eq .Query.status "active"}}...{{end}}', desc: 'Conditional on query param', example: 'Show block when status=active' },
        { key: '{{if contains "admin" .Header.roles}}...{{end}}', desc: 'Check if string contains substring', example: 'Role check' },
        { key: '{{if hasPrefix "Bearer" .Header.authorization}}...{{end}}', desc: 'Prefix check', example: 'Auth header present' },
        { key: '{{range $i, $_ := times 3}}...{{end}}', desc: 'Repeat N times (index in $i)', example: 'Generate 3 items' },
        { key: '{{range seq 1 5}}{{.}} {{end}}', desc: 'Iterate inclusive range 1..5', example: '1 2 3 4 5' },
        { key: '{{range list "a" "b" "c"}}{{.}}{{end}}', desc: 'Iterate over inline list', example: 'abc' },
        { key: '{{range $k, $v := .Query}}...{{end}}', desc: 'Loop over all query params', example: 'All query key/values' },
        { key: '{{with .Path}}...{{end}}', desc: 'Scoped block when path params exist', example: 'Use .Path inside block' },
    ],
    string: [
        { key: '{{upper .Query.name}}', desc: 'Uppercase', example: 'ALICE' },
        { key: '{{lower .Header.authorization}}', desc: 'Lowercase', example: 'bearer ...' },
        { key: '{{trim " value "}}', desc: 'Trim whitespace', example: 'value' },
        { key: '{{trimPrefix "Bearer " .Header.authorization}}', desc: 'Remove prefix', example: 'abc123' },
        { key: '{{trimSuffix ".json" .Path.file}}', desc: 'Remove suffix', example: 'data' },
        { key: '{{replace "-" "_" .Path.id}}', desc: 'Replace all occurrences', example: 'hello_world' },
        { key: '{{truncate 20 .Body.description}}', desc: 'Truncate to N characters', example: 'first 20 chars' },
        { key: '{{default "guest" .Query.name}}', desc: 'Fallback for empty value', example: 'guest' },
        { key: '{{coalesce .Query.q .Body.search "all"}}', desc: 'First non-empty value', example: 'found' },
        { key: '{{split "," .Query.tags}}', desc: 'Split string → list (use with range)', example: '["a","b","c"]' },
        { key: '{{join ", " (split "," .Query.tags)}}', desc: 'Join list back to string', example: 'a, b, c' },
        { key: '{{b64enc .Path.id}}', desc: 'Base64 encode', example: 'NDI=' },
        { key: '{{b64dec .Header.encoded}}', desc: 'Base64 decode', example: 'original' },
        { key: '{{urlEnc .Query.redirect}}', desc: 'URL-encode a value', example: 'hello+world' },
        { key: '{{md5 .Body.email}}', desc: 'MD5 hex hash (Gravatar etc.)', example: '5d41402abc4b2a76' },
        { key: '{{sha256 .Path.id}}', desc: 'SHA-256 hex hash', example: '2cf24dba5...' },
        { key: '{{printf "ID-%05d" (toInt .Path.n)}}', desc: 'Printf-style formatting', example: 'ID-00042' },
    ],
    math: [
        { key: '{{add 1 (toInt .Query.page)}}', desc: 'Addition', example: 'page 3 → 4' },
        { key: '{{sub 100 (toInt .Query.offset)}}', desc: 'Subtraction', example: '100 - 20 → 80' },
        { key: '{{mul 10 (toInt .Query.page)}}', desc: 'Multiplication', example: 'page 3 → 30' },
        { key: '{{div (toInt .Query.total) 10}}', desc: 'Division', example: '100 / 10 → 10' },
        { key: '{{mod (toInt .Path.id) 2}}', desc: 'Modulo', example: '7 mod 2 → 1' },
        { key: '{{max 0 (toInt .Query.page)}}', desc: 'Maximum of two values', example: 'clamp at 0' },
        { key: '{{min 100 (toInt .Query.limit)}}', desc: 'Minimum of two values', example: 'cap at 100' },
        { key: '{{toInt .Query.page}}', desc: 'String → integer', example: '"3" → 3' },
        { key: '{{toFloat .Query.price}}', desc: 'String → float', example: '"9.99" → 9.99' },
        { key: '{{toString 42}}', desc: 'Integer/float → string', example: '42 → "42"' },
    ],
    random: [
        { key: '{{random "uuid"}}', desc: 'Random UUID v4', example: '3d7b9c2e-...' },
        { key: '{{random "uuid4"}}', desc: 'Random UUID v4 (alias)', example: '3d7b9c2e-...' },
        { key: '{{random "int"}}', desc: 'Random integer (0-999999)', example: '58231' },
        { key: '{{random "int(1,10)"}}', desc: 'Random integer in range', example: '7' },
        { key: '{{random "float"}}', desc: 'Random float', example: '491.23' },
        { key: '{{random "float(0,1)"}}', desc: 'Random float in range', example: '0.74' },
        { key: '{{random "alpha"}}', desc: 'Random lowercase letters (len 10)', example: 'abcdefghij' },
        { key: '{{random "alpha(6)"}}', desc: 'Random lowercase letters (custom len)', example: 'xkzwmq' },
        { key: '{{random "ALPHA(8)"}}', desc: 'Random uppercase letters', example: 'XKZWMQAB' },
        { key: '{{random "numeric(6)"}}', desc: 'Random digits', example: '482193' },
        { key: '{{random "hex(8)"}}', desc: 'Random hex string', example: 'a3f2e19b' },
        { key: '{{random "alphanumeric(12)"}}', desc: 'Random alphanumeric', example: 'aZ93kLmP0q12' },
        { key: '{{random "bool"}}', desc: 'Random boolean', example: 'true' },
        { key: '{{random "email"}}', desc: 'Random email', example: 'a1b2c3d4@example.com' },
        { key: '{{random "phone"}}', desc: 'Random phone number', example: '+1-415-555-0100' },
    ],
    faker: [
        { key: '{{faker "name.first"}}', desc: 'First name', example: 'Liam' },
        { key: '{{faker "name.last"}}', desc: 'Last name', example: 'Smith' },
        { key: '{{faker "name"}}', desc: 'Full name', example: 'Olivia Johnson' },
        { key: '{{faker "email"}}', desc: 'Email address', example: 'r2d2@mail.test' },
        { key: '{{faker "phone"}}', desc: 'Phone number', example: '+1-212-555-0199' },
        { key: '{{faker "company.name"}}', desc: 'Company name', example: 'Acme Inc' },
        { key: '{{faker "address.street"}}', desc: 'Street address', example: '123 Oak St' },
        { key: '{{faker "address.city"}}', desc: 'City', example: 'Springfield' },
        { key: '{{faker "address.state"}}', desc: 'State code', example: 'CA' },
        { key: '{{faker "address.zip"}}', desc: 'Postal code', example: '94105' },
        { key: '{{faker "internet.username"}}', desc: 'Username', example: 'alpha9delta' },
        { key: '{{faker "internet.domain"}}', desc: 'Domain', example: 'mock.io' },
        { key: '{{faker "internet.url"}}', desc: 'URL', example: 'https://mock.io/abc' },
        { key: '{{faker "internet.ip"}}', desc: 'IPv4 address', example: '192.168.1.42' },
        { key: '{{faker "internet.mac"}}', desc: 'MAC address', example: 'aa:bb:cc:dd:ee:ff' },
        { key: '{{faker "lorem.word"}}', desc: 'Lorem word', example: 'bravo' },
        { key: '{{faker "lorem.sentence"}}', desc: 'Lorem sentence', example: 'alpha bravo charlie.' },
        { key: '{{faker "lorem.paragraph"}}', desc: 'Lorem paragraph', example: 'alpha bravo...' },
        { key: '{{faker "date.past"}}', desc: 'Random past date (ISO)', example: '2025-03-14' },
        { key: '{{faker "date.future"}}', desc: 'Random future date (ISO)', example: '2026-11-22' },
        { key: '{{faker "date.recent"}}', desc: 'Random date in last 30 days (ISO)', example: '2026-05-12' },
        { key: '{{faker "finance.amount"}}', desc: 'Random amount', example: '1234.56' },
        { key: '{{faker "finance.currency"}}', desc: 'Currency code', example: 'USD' },
        { key: '{{faker "finance.iban"}}', desc: 'Fake IBAN', example: 'GB12 MOCK ...' },
        { key: '{{faker "finance.creditCard"}}', desc: 'Fake credit card number', example: '4123-4567-...' },
        { key: '{{faker "product.name"}}', desc: 'Product name', example: 'Ergonomic Steel Chair' },
        { key: '{{faker "product.category"}}', desc: 'Product category', example: 'Electronics' },
        { key: '{{faker "product.price"}}', desc: 'Product price', example: '49.99' },
        { key: '{{faker "product.sku"}}', desc: 'Product SKU', example: 'SKU-ABC-1234' },
        { key: '{{faker "location.country"}}', desc: 'Country name', example: 'Germany' },
        { key: '{{faker "location.countryCode"}}', desc: 'Country code', example: 'DE' },
        { key: '{{faker "location.timezone"}}', desc: 'Timezone', example: 'America/New_York' },
        { key: '{{faker "location.latitude"}}', desc: 'Latitude', example: '37.7749' },
        { key: '{{faker "location.longitude"}}', desc: 'Longitude', example: '-122.4194' },
        { key: '{{faker "id.objectId"}}', desc: '24-char hex ID (MongoDB-style)', example: 'a1b2c3...' },
        { key: '{{faker "id.nanoid"}}', desc: '21-char URL-safe ID', example: 'V1StGXR8_Z5jdHi6B-myT' },
        { key: '{{faker "id.shortId"}}', desc: '8-char alphanumeric ID', example: 'aB3kPq9z' },
        { key: '{{faker "color.hex"}}', desc: 'Hex color', example: '#a3f2e1' },
        { key: '{{faker "color.name"}}', desc: 'Named color', example: 'coral' },
    ],
    timestamp: [
        { key: '{{timestamp}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp "unix"}}', desc: 'Unix timestamp (seconds)', example: '1707480000' },
        { key: '{{timestamp "unix_ms"}}', desc: 'Unix timestamp (milliseconds)', example: '1707480000123' },
        { key: '{{timestamp "unix_ns"}}', desc: 'Unix timestamp (nanoseconds)', example: '1707480000123456789' },
        { key: '{{timestamp "iso"}}', desc: 'ISO-8601 / RFC3339 UTC', example: '2026-02-09T12:00:00Z' },
        { key: '{{timestamp "date"}}', desc: 'Date only (YYYY-MM-DD)', example: '2026-02-09' },
        { key: '{{timestamp "year"}}', desc: 'Current year', example: '2026' },
        { key: '{{timestamp "month"}}', desc: 'Current month (MM)', example: '02' },
        { key: '{{timestamp "day"}}', desc: 'Current day (DD)', example: '09' },
        { key: '{{timestamp "time"}}', desc: 'Time (HH:MM:SS)', example: '12:00:00' },
        { key: '{{timestamp "datetime"}}', desc: 'Datetime (YYYY-MM-DD HH:MM:SS)', example: '2026-02-09 12:00:00' },
        { key: '{{timestamp "format(Jan 2006)"}}', desc: 'Custom Go layout', example: 'Feb 2026' },
        { key: '{{timestamp "add(1h)"}}', desc: 'Add duration (1h, 30m, 24h)', example: 'now + 1h → RFC3339' },
        { key: '{{timestamp "sub(7d)"}}', desc: 'Subtract duration (7d = 168h)', example: 'now - 7d → RFC3339' },
        { key: '{{now | dateFormat "Jan 2006"}}', desc: 'now + dateFormat pipe', example: 'May 2026' },
    ],
    json: [
        { key: '{{toJSON .Script.result}}', desc: 'Marshal any value to JSON string', example: '{"status":"ok"}' },
        { key: '{{jsonGet "user.name" .RawBody}}', desc: 'gjson path from a JSON string field', example: 'carol' },
        { key: '{{numberFormat 1234567.89}}', desc: 'Thousands-separated number', example: '1,234,567.89' },
        { key: '{{currency 19.99 "$"}}', desc: 'Currency formatting', example: '$19.99' },
    ],
}

export default function ResponseConfigEditor({
    operationId,
    config,
    onClose,
    variant = 'modal',
    readOnly = false,
}: ResponseConfigEditorProps) {
    const [name, setName] = useState(config?.name || '')
    const [description, setDescription] = useState(config?.description || '')
    const [statusCode, setStatusCode] = useState(config?.statusCode || 200)
    const [priority, setPriority] = useState(config?.priority || 0)
    const [delay, setDelay] = useState(config?.delay || 0)
    const [enabled, setEnabled] = useState(config?.enabled ?? true)
    const [tag, setTag] = useState(config?.tag || 'default')
    const [conditions, setConditions] = useState<Condition[]>(() =>
        (config?.conditions || []).map(normaliseCondition)
    )
    const [headers, setHeaders] = useState<Record<string, string>>(config?.headers || {})
    const [body, setBody] = useState(config?.body || '')
    const [error, setError] = useState('')
    const [bodyError, setBodyError] = useState('')
    const [isValidating, setIsValidating] = useState(false)
    const [headerKey, setHeaderKey] = useState('')
    const [headerValue, setHeaderValue] = useState('')
    const [showExamplePicker, setShowExamplePicker] = useState(false)
    const editorRef = useRef<Monaco.editor.IStandaloneCodeEditor | null>(null)
    const monacoRef = useRef<typeof Monaco | null>(null)
    const validationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const [pendingCollectionMapping, setPendingCollectionMapping] = useState<CollectionMappingInput | null>(null)

    const queryClient = useQueryClient()
    const isDark = useIsDark()

    const { data: tags } = useQuery({
        queryKey: ['tags'],
        queryFn: tagsApi.list,
    })

    const { data: regexPatterns } = useQuery({
        queryKey: ['conditions', 'regex-patterns'],
        queryFn: conditionsApi.listRegexPatterns,
        staleTime: Infinity,
    })

    const { data: scriptBindings } = useQuery<ScriptBinding[]>({
        queryKey: ['scriptBindings', operationId],
        queryFn: () => scriptBindingsApi.listByOperation(operationId),
        enabled: !!operationId,
    })

    const tagOptions = (tags && tags.length > 0) ? tags : [{ name: 'default' }]

    const createMutation = useMutation({
        mutationFn: (data: ResponseConfigInput) => responsesApi.create(operationId, data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['responses', operationId] })
            onClose()
        },
        onError: (err: Error) => setError(err.message),
    })

    const updateMutation = useMutation({
        mutationFn: (data: ResponseConfigInput) => responsesApi.update(config!.id, data),
        onSuccess: async () => {
            if (pendingCollectionMapping && config?.id) {
                try {
                    await collectionMappingsApi.create(operationId, config.id, pendingCollectionMapping)
                    setPendingCollectionMapping(null)
                    queryClient.invalidateQueries({ queryKey: ['collectionMappings', config.id] })
                } catch { /* silent — pending form stays visible for retry */ }
            }
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

    const applySpecExample = useCallback((example: SpecExample) => {
        setStatusCode(example.statusCode || 200)
        setBody(example.bodyExample || '')
        if (example.contentType) {
            setHeaders(prev => ({ ...prev, 'Content-Type': example.contentType }))
        }
        if (example.bodyExample) {
            scheduleTemplateValidation(example.bodyExample)
        }
    }, [scheduleTemplateValidation])

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
        setConditions((config.conditions || []).map(normaliseCondition))
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
        <>
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

                <form onSubmit={readOnly ? (e) => e.preventDefault() : handleSubmit} className="flex-1 overflow-y-auto p-6 space-y-6">
                    {readOnly && (
                        <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 flex items-start dark:bg-amber-950/40 dark:border-amber-900/40">
                            <AlertCircle className="w-5 h-5 text-amber-600 mr-3 flex-shrink-0 mt-0.5" />
                            <div>
                                <p className="text-amber-700 dark:text-amber-300 font-medium">Read-only — recorded response</p>
                                <p className="text-amber-600 dark:text-amber-400 text-sm mt-0.5">
                                    This response was auto-recorded. Use the <strong>Clone as Manual</strong> action in the response list to create an editable copy.
                                </p>
                            </div>
                        </div>
                    )}

                    {!readOnly && error && (
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                disabled={readOnly}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
                                min={0}
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1">
                                Enabled
                            </label>
                            <button
                                type="button"
                                onClick={() => !readOnly && setEnabled(!enabled)}
                                disabled={readOnly}
                                className={`w-full px-3 py-2 rounded-lg border disabled:opacity-60 disabled:cursor-not-allowed ${enabled
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
                            {!readOnly && (
                            <button
                                type="button"
                                onClick={addCondition}
                                className="text-sm text-primary-600 hover:text-primary-700 flex items-center"
                            >
                                <Plus className="w-4 h-4 mr-1" />
                                Add Condition
                            </button>
                            )}
                        </div>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mb-3">
                            All conditions must match (AND logic)
                        </p>
                        <div className="space-y-2">
                            {conditions.map((cond, index) => (
                                <div key={index} className="space-y-1">
                                <div className="flex items-center gap-2">
                                    <select
                                        value={cond.source}
                                        onChange={(e) =>
                                            updateCondition(index, { source: e.target.value as any })
                                        }
                                        disabled={readOnly}
                                        className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
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
                                        placeholder={
                                            cond.source === 'signature' ? 'computed request signature' :
                                            cond.source === 'script' ? 'binding.fieldName' :
                                            'key'
                                        }
                                        title={cond.source === 'script' ? 'Dot-path into operation-level script output, e.g. authCheck.tier' : undefined}
                                        className="flex-1 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
                                        disabled={readOnly || cond.source === 'signature'}
                                    />
                                    <select
                                        value={cond.operator}
                                        onChange={(e) =>
                                            updateCondition(index, { operator: e.target.value as any })
                                        }
                                        disabled={readOnly}
                                        className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
                                    >
                                        <optgroup label="String / Numeric">
                                            {operators.filter(op => !op.group).map((op) => (
                                                <option key={op.value} value={op.value}>
                                                    {op.label}
                                                </option>
                                            ))}
                                        </optgroup>
                                        <optgroup label="Date">
                                            {operators.filter(op => op.group === 'date').map((op) => (
                                                <option key={op.value} value={op.value}>
                                                    {op.label}
                                                </option>
                                            ))}
                                        </optgroup>
                                    </select>
                                    {/* NOT toggle */}
                                    <button
                                        type="button"
                                        title="Negate — invert the condition result"
                                        onClick={() => !readOnly && updateCondition(index, { negate: !cond.negate })}
                                        disabled={readOnly}
                                        className={`px-2 py-1.5 rounded border text-xs font-semibold transition-colors disabled:opacity-60 disabled:cursor-not-allowed ${
                                            cond.negate
                                                ? 'bg-red-100 dark:bg-red-900/40 border-red-400 dark:border-red-500 text-red-700 dark:text-red-300'
                                                : 'border-gray-300 dark:border-slate-700 text-gray-400 dark:text-slate-500 hover:border-red-400 hover:text-red-500'
                                        }`}
                                    >
                                        NOT
                                    </button>
                                    {cond.operator === 'regex' && (
                                        <datalist id={`regex-patterns-${index}`}>
                                            {(regexPatterns || []).map(p => (
                                                <option key={p.token} value={p.token}>{p.description}</option>
                                            ))}
                                        </datalist>
                                    )}
                                    {DATE_OPERATORS.has(cond.operator) && !DATE_NO_VALUE_OPERATORS.has(cond.operator) && (
                                        <datalist id={`date-tokens-${index}`}>
                                            {DATE_TOKENS.map(t => (
                                                <option key={t.token} value={t.token}>{t.description}</option>
                                            ))}
                                        </datalist>
                                    )}
                                    <input
                                        type="text"
                                        autoComplete="off"
                                        list={
                                            cond.operator === 'regex' ? `regex-patterns-${index}`
                                            : (DATE_OPERATORS.has(cond.operator) && !DATE_NO_VALUE_OPERATORS.has(cond.operator)) ? `date-tokens-${index}`
                                            : undefined
                                        }
                                        value={cond.value}
                                        onChange={(e) => updateCondition(index, { value: e.target.value })}
                                        placeholder={
                                            DATE_NO_VALUE_OPERATORS.has(cond.operator) ? '—'
                                            : cond.operator === 'dateBetween' ? 'from,to  e.g. today,now+7d'
                                            : DATE_OPERATORS.has(cond.operator) ? 'e.g. today, now+7d, 2025-01-01'
                                            : cond.operator === 'regex' ? 'regex or token e.g. uuid, email'
                                            : 'value'
                                        }
                                        className="flex-1 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded text-sm bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 disabled:opacity-60 disabled:cursor-not-allowed"
                                        disabled={
                                            readOnly ||
                                            cond.operator === 'exists' ||
                                            DATE_NO_VALUE_OPERATORS.has(cond.operator)
                                        }
                                    />
                                    {!readOnly && (
                                    <button
                                        type="button"
                                        onClick={() => removeCondition(index)}
                                        className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-red-600 rounded"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                    </button>
                                    )}
                                </div>
                                {DATE_OPERATORS.has(cond.operator) && !DATE_NO_VALUE_OPERATORS.has(cond.operator) && (
                                    <div className="flex items-center gap-2 pl-1">
                                        <span className="text-xs text-gray-400 dark:text-slate-500 w-24 shrink-0">Format hint</span>
                                        <input
                                            type="text"
                                            autoComplete="off"
                                            value={cond.format ?? ''}
                                            onChange={(e) => updateCondition(index, { format: e.target.value || undefined })}
                                            placeholder="auto-detect  (e.g. 2006-01-02 or 01/02/2006)"
                                            disabled={readOnly}
                                            className="flex-1 px-2 py-1 border border-gray-200 dark:border-slate-700 rounded text-xs bg-white dark:bg-slate-950 text-gray-500 dark:text-slate-400 placeholder-gray-300 dark:placeholder-slate-600 disabled:opacity-60 disabled:cursor-not-allowed"
                                        />
                                        {(() => {
                                            const tok = DATE_TOKENS.find(t => t.token.toLowerCase() === cond.value.toLowerCase())
                                            return tok ? (
                                                <span className="text-xs text-violet-600 dark:text-violet-400 font-medium shrink-0">{tok.description}</span>
                                            ) : (
                                                <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0">
                                                    {cond.operator === 'dateBetween' ? 'two comma-separated values' : 'token or date literal'}
                                                </span>
                                            )
                                        })()}
                                    </div>
                                )}
                                {cond.operator === 'regex' && cond.value && regexPatterns && (
                                    (() => {
                                        const match = regexPatterns.find(p => p.token.toLowerCase() === cond.value.toLowerCase())
                                        return match ? (
                                            <div className="flex items-center gap-2 pl-1">
                                                <span className="text-xs text-gray-400 dark:text-slate-500 w-24 shrink-0">Token</span>
                                                <span className="text-xs text-violet-600 dark:text-violet-400 font-medium">{match.description}</span>
                                                <code className="text-xs text-gray-400 dark:text-slate-500 font-mono truncate max-w-xs">{match.pattern}</code>
                                            </div>
                                        ) : (
                                            <div className="flex items-center gap-2 pl-1">
                                                <span className="text-xs text-gray-400 dark:text-slate-500 w-24 shrink-0">Regex</span>
                                                <span className="text-xs text-gray-400 dark:text-slate-500">raw pattern</span>
                                            </div>
                                        )
                                    })()
                                )}
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
                        {!readOnly && (
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
                        )}
                        {Object.entries(headers).length > 0 && (
                            <div className="bg-gray-50 dark:bg-slate-800 rounded-lg p-3 space-y-1">
                                {Object.entries(headers).map(([key, value]) => (
                                    <div key={key} className="flex items-center justify-between text-sm">
                                        <span>
                                            <span className="font-medium text-gray-900 dark:text-slate-100">{key}:</span> <span className="text-gray-700 dark:text-slate-300">{value}</span>
                                        </span>
                                        {!readOnly && (
                                        <button
                                            type="button"
                                            onClick={() => removeHeader(key)}
                                            className="text-gray-400 dark:text-slate-500 hover:text-red-600"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </button>
                                        )}
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
                            {!readOnly && (
                            <div className="flex items-center gap-1.5">
                                <button
                                    type="button"
                                    onClick={() => setShowExamplePicker(true)}
                                    title="Load from spec example"
                                    className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-slate-300 bg-gray-100 dark:bg-slate-800 hover:bg-gray-200 dark:hover:bg-slate-700 border border-gray-300 dark:border-slate-600 rounded-md transition-colors"
                                >
                                    <Zap className="w-3.5 h-3.5" />
                                    Examples
                                </button>
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
                            )}
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
                                    readOnly: readOnly,
                                    theme: isDark ? 'vs-dark' : 'light',
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
                        {readOnly ? 'Close' : 'Cancel'}
                    </button>
                    {!readOnly && (
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
                    )}
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
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-rose-100 dark:border-rose-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-rose-600 dark:text-rose-300 uppercase tracking-wide mb-3">
                                    String helpers
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.string.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-rose-900 dark:text-rose-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-rose-500/90 dark:text-rose-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-orange-100 dark:border-orange-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-orange-600 dark:text-orange-300 uppercase tracking-wide mb-3">
                                    Math helpers
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.math.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-orange-900 dark:text-orange-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-orange-500/90 dark:text-orange-300">Example: {item.example}</span>
                                        </li>
                                    ))}
                                </ul>
                            </div>
                            <div className="bg-white/70 dark:bg-slate-900/70 border border-teal-100 dark:border-teal-900/40 rounded-lg p-4">
                                <h4 className="text-xs font-semibold text-teal-600 dark:text-teal-300 uppercase tracking-wide mb-3">
                                    JSON &amp; formatting
                                </h4>
                                <ul className="space-y-3 text-xs text-gray-700 dark:text-slate-300">
                                    {templateDocs.json.map((item) => (
                                        <li key={item.key} className="flex flex-col gap-1">
                                            <span className="font-mono text-teal-900 dark:text-teal-200">{item.key}</span>
                                            <span className="text-gray-600 dark:text-slate-300">{item.desc}</span>
                                            <span className="text-teal-500/90 dark:text-teal-300">Example: {item.example}</span>
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
        {/* Response-level script bindings — shown below editor when editing an existing config in page mode */}
        {!isModal && config?.id && !readOnly && (
            <ScriptBindingsPanel
                kind="response"
                operationId={operationId}
                responseConfigId={config.id}
            />
        )}
        {/* Collection mappings — shown below script bindings in page mode */}
        {!isModal && config?.id && !readOnly && (
            <CollectionMappingsPanel
                operationId={operationId}
                responseConfigId={config.id}
                pendingMapping={pendingCollectionMapping}
                onPendingMappingChange={setPendingCollectionMapping}
            />
        )}
        {showExamplePicker && (
            <ExamplePickerModal
                operationId={operationId}
                onSelect={applySpecExample}
                onClose={() => setShowExamplePicker(false)}
            />
        )}
        </>
    )
}
