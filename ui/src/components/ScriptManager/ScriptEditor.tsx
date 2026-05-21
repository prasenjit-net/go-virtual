import React, { useState, useCallback, useRef } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import Editor, { type Monaco } from '@monaco-editor/react'
import type * as monacoEditor from 'monaco-editor'
import {
    ArrowLeft, Save, CheckCircle, XCircle, Play, ChevronDown, ChevronUp,
    Clock, ToggleLeft, ToggleRight, Loader2, Sparkles, BookOpen,
    Settings, GripHorizontal
} from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi, aiApi } from '../../services/api'
import type { AIStatus, Script } from '../../types'
import AIScriptModal from './AIScriptModal'
import { useIsDark } from '../../hooks/useIsDark'

// ─── Starlark completion provider (registered once per page lifetime) ─────────

let _starlarkCompletionsRegistered = false

function registerStarlarkCompletions(monaco: Monaco) {
    if (_starlarkCompletionsRegistered) return
    _starlarkCompletionsRegistered = true

    const CIK = monaco.languages.CompletionItemKind
    const CITR = monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet

    // Helper: build a completion item with a shared range
    const mk = (
        range: monacoEditor.IRange,
        label: string,
        kind: number,
        detail: string,
        insertText: string,
        doc?: string,
    ) => ({ label, kind, detail, insertText, insertTextRules: CITR, documentation: doc, range })

    // ── Provider 1: dot-triggered member access ────────────────────────────
    monaco.languages.registerCompletionItemProvider('python', {
        triggerCharacters: ['.'],
        provideCompletionItems: (
            model: monacoEditor.editor.ITextModel,
            position: monacoEditor.Position,
        ) => {
            const textBefore = model.getLineContent(position.lineNumber).substring(0, position.column - 1)
            const word = model.getWordUntilPosition(position)
            const range: monacoEditor.IRange = {
                startLineNumber: position.lineNumber,
                endLineNumber: position.lineNumber,
                startColumn: word.startColumn,
                endColumn: word.endColumn,
            }
            const s = (label: string, kind: number, detail: string, insertText: string, doc?: string) =>
                mk(range, label, kind, detail, insertText, doc)

            // req.*
            if (/\breq\.$/.test(textBefore) || /\breq\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('path',   CIK.Method, 'req.path(key[, default]) → string',  'path("${1:key}", "${2:}")',   'Read a path parameter. Raises if missing and no default given.\nExample: req.path("id", "")'),
                    s('query',  CIK.Method, 'req.query(key[, default]) → string', 'query("${1:key}", "${2:}")',  'Read a query parameter. Raises if missing and no default given.\nExample: req.query("page", "1")'),
                    s('header', CIK.Method, 'req.header(key[, default]) → string','header("${1:key}", "${2:}")', 'Read a request header (key auto-lowercased). Raises if missing and no default given.\nExample: req.header("authorization", "")'),
                    s('body',   CIK.Method, 'req.body([path[, default]]) → value','body()',                      'No args: returns whole body (dict/list/None).\nWith path: req.body("field.nested", default) for gjson-style navigation.\nExample: req.body("user.name", "")'),
                ]}
            }

            // store.*
            if (/\bstore\.$/.test(textBefore) || /\bstore\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('get',        CIK.Method, 'store.get(key[, default]) → value', 'get("${1:key}")',              'Read a value from the session store. Returns None if absent.'),
                    s('set',        CIK.Method, 'store.set(key, value)',              'set("${1:key}", ${2:value})',  'Write a value to the session store.'),
                    s('has',        CIK.Method, 'store.has(key) → bool',             'has("${1:key}")',              'True if key exists in the session store.'),
                    s('delete',     CIK.Method, 'store.delete(key)',                  'delete("${1:key}")',           'Remove a key from the session store.'),
                    s('keys',       CIK.Method, 'store.keys() → list',               'keys()',                       'Return all keys in the session store.'),
                    s('collection', CIK.Method, 'store.collection(name) → col',      'collection("${1:name}")',      'Get a named global collection. Shared across all sessions.'),
                ]}
            }

            // datetime.date.* (static class methods)
            if (/\bdatetime\.date\.$/.test(textBefore) || /\bdatetime\.date\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('today',        CIK.Method, 'datetime.date.today() → date',               'today()',             'Return today\'s date in UTC.'),
                    s('fromisoformat', CIK.Method, 'datetime.date.fromisoformat(str) → date',   'fromisoformat("${1:YYYY-MM-DD}")', 'Parse a date from ISO format string "YYYY-MM-DD".'),
                ]}
            }

            // datetime.datetime.* (static class methods)
            if (/\bdatetime\.datetime\.$/.test(textBefore) || /\bdatetime\.datetime\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('now',           CIK.Method, 'datetime.datetime.now() → datetime',           'now()',              'Return the current datetime (UTC).'),
                    s('utcnow',        CIK.Method, 'datetime.datetime.utcnow() → datetime',        'utcnow()',           'Return the current UTC datetime (alias for now).'),
                    s('fromisoformat', CIK.Method, 'datetime.datetime.fromisoformat(str) → datetime', 'fromisoformat("${1:str}")',  'Parse datetime from RFC3339/ISO string.'),
                    s('fromtimestamp', CIK.Method, 'datetime.datetime.fromtimestamp(secs) → datetime', 'fromtimestamp(${1:secs})', 'Create datetime from Unix timestamp (seconds).'),
                ]}
            }

            // datetime.timedelta.* constructor
            if (/\bdatetime\.timedelta\.$/.test(textBefore) || /\bdatetime\.timedelta\.\w+$/.test(textBefore)) {
                // timedelta is a constructor — no static methods, return empty to avoid noise
                return { suggestions: [] }
            }

            // datetime.* (module members: date, datetime, timedelta)
            if (/\bdatetime\.$/.test(textBefore) || /\bdatetime\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('date',      CIK.Class, 'datetime.date(year, month, day) → date',                'date',      'Date type. Constructor: datetime.date(year, month, day). Static: .today(), .fromisoformat()'),
                    s('datetime',  CIK.Class, 'datetime.datetime(year, month, day, ...) → datetime',   'datetime',  'Datetime type. Constructor and static methods: .now(), .fromisoformat(), .fromtimestamp()'),
                    s('timedelta', CIK.Class, 'datetime.timedelta(days?, hours?, minutes?, seconds?)', 'timedelta', 'Timedelta constructor. kwargs: days, hours, minutes, seconds.'),
                ]}
            }

            // validate.*
            if (/\bvalidate\.$/.test(textBefore) || /\bvalidate\.\w+$/.test(textBefore)) {
                const validateMethods = [
                    s('matches',       CIK.Method, 'validate.matches(value, token_or_pattern) → bool', 'matches(${1:value}, "${2:uuid}")', 'Match against a named token (uuid, email, …) or raw regex.'),
                    s('regex',         CIK.Method, 'validate.regex(value, pattern) → bool',            'regex(${1:value}, "${2:pattern}")', 'Match against a raw regex pattern.'),
                    s('pattern_names', CIK.Method, 'validate.pattern_names() → list',                  'pattern_names()',                  'Return list of all registered token names.'),
                    // Convenience helpers
                    s('is_uuid',        CIK.Method, '(value) → bool', 'is_uuid(${1:v})',        'True if value is a valid UUID.'),
                    s('is_email',       CIK.Method, '(value) → bool', 'is_email(${1:v})',       'True if value is a valid email address.'),
                    s('is_url',         CIK.Method, '(value) → bool', 'is_url(${1:v})',         'True if value is a valid URL.'),
                    s('is_ipv4',        CIK.Method, '(value) → bool', 'is_ipv4(${1:v})',        'True if value is an IPv4 address.'),
                    s('is_ipv6',        CIK.Method, '(value) → bool', 'is_ipv6(${1:v})',        'True if value is an IPv6 address.'),
                    s('is_ip',          CIK.Method, '(value) → bool', 'is_ip(${1:v})',          'True if value is an IPv4 or IPv6 address.'),
                    s('is_us_phone',    CIK.Method, '(value) → bool', 'is_us_phone(${1:v})',    'True if value is a US phone number.'),
                    s('is_us_zip',      CIK.Method, '(value) → bool', 'is_us_zip(${1:v})',      'True if value is a US ZIP code.'),
                    s('is_ssn',         CIK.Method, '(value) → bool', 'is_ssn(${1:v})',         'True if value is a US Social Security Number.'),
                    s('is_date_iso',    CIK.Method, '(value) → bool', 'is_date_iso(${1:v})',    'True if value matches YYYY-MM-DD.'),
                    s('is_datetime_iso',CIK.Method, '(value) → bool', 'is_datetime_iso(${1:v})','True if value is an ISO datetime string.'),
                    s('is_integer',     CIK.Method, '(value) → bool', 'is_integer(${1:v})',     'True if value is an integer string.'),
                    s('is_decimal',     CIK.Method, '(value) → bool', 'is_decimal(${1:v})',     'True if value is a decimal number string.'),
                    s('is_semver',      CIK.Method, '(value) → bool', 'is_semver(${1:v})',      'True if value is a semantic version (e.g. 1.2.3).'),
                    s('is_jwt',         CIK.Method, '(value) → bool', 'is_jwt(${1:v})',         'True if value looks like a JWT token.'),
                    s('is_slug',        CIK.Method, '(value) → bool', 'is_slug(${1:v})',        'True if value is a URL slug.'),
                    s('is_base64',      CIK.Method, '(value) → bool', 'is_base64(${1:v})',      'True if value is valid base64.'),
                    s('is_hex_color',   CIK.Method, '(value) → bool', 'is_hex_color(${1:v})',   'True if value is a hex color (#RGB or #RRGGBB).'),
                    s('is_credit_card', CIK.Method, '(value) → bool', 'is_credit_card(${1:v})', 'True if value is a credit card number.'),
                    s('is_iban',        CIK.Method, '(value) → bool', 'is_iban(${1:v})',        'True if value is an IBAN.'),
                ]
                return { suggestions: validateMethods }
            }

            // Collection methods (for any other variable.xxx pattern)
            if (/\b\w+\.$/.test(textBefore) || /\b\w+\.\w+$/.test(textBefore)) {
                return { suggestions: [
                    s('findAll', CIK.Method, 'col.findAll([filter]) → list',       'findAll()',              'Return all documents; pass a dict to filter by equality.'),
                    s('findOne', CIK.Method, 'col.findOne(filter) → dict|None',    'findOne({${1}})',         'Return first document matching filter, or None.'),
                    s('insert',  CIK.Method, 'col.insert(doc)',                    'insert({${1}})',          'Insert a document into the collection.'),
                    s('update',  CIK.Method, 'col.update(filter, patch)',          'update({${1}}, {${2}})',  'Update all documents matching filter with the patch dict.'),
                    s('remove',  CIK.Method, 'col.remove(filter)',                 'remove({${1}})',          'Remove all documents matching filter.'),
                    s('count',   CIK.Method, 'col.count([filter]) → int',         'count()',                 'Count documents; pass a dict to filter.'),
                    s('clear',   CIK.Method, 'col.clear()',                        'clear()',                 'Remove all documents from the collection.'),
                ]}
            }

            return { suggestions: [] }
        },
    })

    // ── Provider 2: top-level builtins (no dot, Ctrl+Space / normal typing) ──
    monaco.languages.registerCompletionItemProvider('python', {
        triggerCharacters: [],
        provideCompletionItems: (
            model: monacoEditor.editor.ITextModel,
            position: monacoEditor.Position,
        ) => {
            const textBefore = model.getLineContent(position.lineNumber).substring(0, position.column - 1)
            // Skip if we're in a member-access context (after a dot)
            if (/\.\w*$/.test(textBefore)) return { suggestions: [] }

            const word = model.getWordUntilPosition(position)
            const range: monacoEditor.IRange = {
                startLineNumber: position.lineNumber,
                endLineNumber: position.lineNumber,
                startColumn: word.startColumn,
                endColumn: word.endColumn,
            }
            const s = (label: string, kind: number, detail: string, insertText: string, doc?: string) =>
                mk(range, label, kind, detail, insertText, doc)

            return { suggestions: [
                // Core builtins
                s('uuid',           CIK.Function, '() → string',              'uuid()',                          'Generate a random UUID v4.'),
                s('now',            CIK.Function, '([fmt]) → int|string',     'now()',                           'Current time. Use now("iso"), now("date"), now("unix_ms"), or now() for Unix seconds.'),
                s('sleep',          CIK.Function, '(ms)',                      'sleep(${1:100})',                 'Pause execution for ms milliseconds.'),
                s('rand_int',       CIK.Function, '(max) or (min, max) → int','rand_int(${1:100})',              'Random integer. rand_int(max) → [0, max]. rand_int(min, max) → [min, max].'),
                s('rand_choice',    CIK.Function, '(list) → value',           'rand_choice(${1:list})',          'Return a random element from a list.'),
                s('counter',        CIK.Function, '(name[, n]) → int',        'counter("${1:name}")',            'Increment named counter, return new value. Pass n=0 to read without incrementing.'),
                s('log',            CIK.Function, '(...)',                     'log(${1})',                       'Append one or more values to the request trace log.'),
                s('hash',           CIK.Function, '(algo, str) → string',     'hash("${1|sha256,md5,sha1,sha512|}", ${2:s})', 'Hex digest. Supported algos: md5, sha1, sha256, sha512.'),
                s('base64_encode',  CIK.Function, '(str) → string',           'base64_encode(${1:s})',           'Encode a string as standard base64.'),
                s('base64_decode',  CIK.Function, '(str) → string',           'base64_decode(${1:s})',           'Decode a base64-encoded string.'),
                s('json_parse',     CIK.Function, '(str) → value',            'json_parse(${1:s})',              'Parse a JSON string into a Starlark dict/list/value.'),
                s('json_stringify', CIK.Function, '(value) → string',         'json_stringify(${1:v})',          'Serialize a Starlark value to a JSON string.'),
                s('regex_match',    CIK.Function, '(pattern, str) → bool',    'regex_match("${1:pattern}", ${2:s})', 'Return True if pattern matches anywhere in str.'),
                s('regex_find',     CIK.Function, '(pattern, str) → str|None','regex_find("${1:pattern}", ${2:s})', 'Return first match or None.'),
                s('regex_find_all', CIK.Function, '(pattern, str) → list',    'regex_find_all("${1:pattern}", ${2:s})', 'Return list of all non-overlapping matches.'),

                // Modules
                s('store',    CIK.Module,   'session key-value store',  'store',    'Per-request session store. Methods: get, set, has, delete, keys, collection.'),
                s('datetime', CIK.Module,   'datetime utilities',       'datetime', 'datetime module. Access: datetime.date, datetime.datetime, datetime.timedelta.'),
                s('validate', CIK.Module,   'value validation helpers', 'validate', 'validate module. Methods: matches, regex, is_email, is_uuid, is_url, …'),

                // req variable — appears in run(req) context
                s('req', CIK.Variable, 'request object', 'req', 'Request object with callable attrs: req.path(key,default), req.query(key,default), req.header(key,default), req.body([path,default])'),
            ]}
        },
    })
}

const DEFAULT_SOURCE = `# Starlark script — define a run(req) function that returns a dict.
# req.path("id", "")           → path parameter with default
# req.query("page", "1")       → query parameter with default
# req.header("authorization", "") → request header (lowercased key)
# req.body()                   → whole parsed JSON body (dict/list/None)
# req.body("user.name", "")    → gjson-style nested path with default
# store                        → session key-value store (get, set, has, delete, keys)
# log(...)                     → append message to trace logs

def run(req):
    return {
        "message": "Hello from script!",
    }
`

export default function ScriptEditor() {
    const { scriptId } = useParams<{ scriptId: string }>()
    const isNew = !scriptId
    const navigate = useNavigate()
    const queryClient = useQueryClient()
    const isDarkTheme = useIsDark()

    // Form state
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [timeout, setTimeout] = useState(0)
    const [enabled, setEnabled] = useState(true)
    const [source, setSource] = useState(DEFAULT_SOURCE)
    const [formInitialised, setFormInitialised] = useState(false)

    // Keep a ref in sync with source so handleTest always reads the latest value
    // regardless of closure staleness
    const sourceRef = useRef(DEFAULT_SOURCE)
    const timeoutRef = useRef(0)

    // savedSource drives the dirty banner only — has no effect on test execution
    // null = not yet loaded (new script or loading); string = last persisted source
    const [savedSource, setSavedSource] = useState<string | null>(isNew ? '' : null)
    const isDirty = savedSource !== null && source !== savedSource


    const [validateResult, setValidateResult] = useState<{ valid: boolean; error: string | null } | null>(null)
    const [isValidating, setIsValidating] = useState(false)

    // AI modal state — conversation persists until the script is saved
    const [showAIModal, setShowAIModal] = useState(false)
    const [aiHistory, setAiHistory] = useState<Array<{ role: 'user' | 'assistant'; content: string }>>([])

    // Test panel state
    const [testOpen, setTestOpen] = useState(false)
    const [testPath, setTestPath] = useState('{}')
    const [testQuery, setTestQuery] = useState('{}')
    const [testHeader, setTestHeader] = useState('{}')
    const [testBody, setTestBody] = useState('null')
    const [testResult, setTestResult] = useState<{ output: any; durationMs: number; error: string | null; logs?: string[] } | null>(null)
    const [isTesting, setIsTesting] = useState(false)

    // Reference panel state (mobile only)
    const [refOpen, setRefOpen] = useState(false)

    // ── IDE layout state (desktop only) ───────────────────────────────────────
    const [ideActiveTab, setIdeActiveTab] = useState<'test' | 'metadata' | 'reference'>('test')
    const [ideBottomHeight, setIdeBottomHeight] = useState(280)
    const ideDragRef = useRef<{ startY: number; startHeight: number } | null>(null)

    // Stable ref for Ctrl+S binding in Monaco (always points at latest handleSave)
    const handleSaveRef = useRef<() => void>(() => {})

    const { data: aiStatus = { configured: true, provider: 'openai' } } = useQuery<AIStatus>({
        queryKey: ['ai-status'],
        queryFn: () => aiApi.getStatus(),
        staleTime: 60_000,
    })
    const aiConfigured = aiStatus.configured
    const aiProviderLabel = aiStatus.provider === 'claude' ? 'Claude' : aiStatus.provider === 'openai' ? 'OpenAI' : 'AI provider'

    // Reset all form state when scriptId changes (e.g. navigating between scripts)
    const prevScriptIdRef = useRef<string | undefined>(undefined)
    if (prevScriptIdRef.current !== scriptId) {
        prevScriptIdRef.current = scriptId
        if (formInitialised) {
            setFormInitialised(false)
            setName('')
            setDescription('')
            setTimeout(0)
            setEnabled(true)
            setSource(DEFAULT_SOURCE)
            sourceRef.current = DEFAULT_SOURCE
            timeoutRef.current = 0
            setSavedSource(isNew ? '' : null)
            setValidateResult(null)
            setTestResult(null)
        }
    }

    // Load existing script for edit mode
    const { isLoading: isLoadingScript } = useQuery<Script>({
        queryKey: ['script', scriptId],
        queryFn: () => scriptsApi.get(scriptId!),
        enabled: !isNew,
        staleTime: 0,
        select: (data) => {
            if (!formInitialised) {
                setName(data.name)
                setDescription(data.description)
                setTimeout(data.timeout)
                timeoutRef.current = data.timeout
                setEnabled(data.enabled)
                if (data.source !== undefined) {
                    setSource(data.source || '')
                    sourceRef.current = data.source || ''
                    setSavedSource(data.source || '')
                }
                setFormInitialised(true)
            }
            return data
        },
    })

    const saveMutation = useMutation({
        mutationFn: (payload: { name: string; description: string; source: string; timeout: number; enabled: boolean }) =>
            isNew
                ? scriptsApi.create(payload)
                : scriptsApi.update(scriptId!, payload),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['scripts'] })
            if (!isNew) {
                queryClient.invalidateQueries({ queryKey: ['script', scriptId] })
            }
            setSavedSource(sourceRef.current)
            // Discard the AI conversation context on save.
            setAiHistory([])
            navigate(`/scripts`)
        },
    })

    const handleSave = () => {
        if (!name.trim()) return
        saveMutation.mutate({ name: name.trim(), description, source, timeout, enabled })
    }
    // Keep Ctrl+S binding always pointing at the latest handleSave
    handleSaveRef.current = handleSave

    const handleValidate = useCallback(async () => {
        setIsValidating(true)
        setValidateResult(null)
        try {
            const result = await scriptsApi.validate(source)
            setValidateResult(result)
        } catch (e) {
            setValidateResult({ valid: false, error: (e as Error).message })
        } finally {
            setIsValidating(false)
        }
    }, [source])

    const handleTest = useCallback(async () => {
        // Read directly from refs — never stale, no closure capture issues
        const currentSource = sourceRef.current
        const currentTimeout = timeoutRef.current
        setIsTesting(true)
        setTestResult(null)
        // Switch to test tab in IDE view when running
        setIdeActiveTab('test')
        try {
            // Validate first — bail with a clear message if syntax is bad
            const validation = await scriptsApi.validate(currentSource)
            if (!validation.valid) {
                setTestResult({ output: null, durationMs: 0, error: `Script has errors: ${validation.error}` })
                return
            }

            let parsedPath: Record<string, string> = {}
            let parsedQuery: Record<string, string> = {}
            let parsedHeader: Record<string, string> = {}
            let parsedBody: any = null
            try { parsedPath = JSON.parse(testPath) } catch { /* ignore */ }
            try { parsedQuery = JSON.parse(testQuery) } catch { /* ignore */ }
            try { parsedHeader = JSON.parse(testHeader) } catch { /* ignore */ }
            try { parsedBody = JSON.parse(testBody) } catch { /* ignore */ }
            const input = { path: parsedPath, query: parsedQuery, header: parsedHeader, body: parsedBody }
            const result = await scriptsApi.testSource(currentSource, currentTimeout, input)
            setTestResult(result)
        } catch (e) {
            setTestResult({ output: null, durationMs: 0, error: (e as Error).message })
        } finally {
            setIsTesting(false)
        }
    }, [testPath, testQuery, testHeader, testBody])

    const handleIdeDragStart = (e: React.MouseEvent) => {
        ideDragRef.current = { startY: e.clientY, startHeight: ideBottomHeight }
        const onMove = (ev: MouseEvent) => {
            if (!ideDragRef.current) return
            const delta = ideDragRef.current.startY - ev.clientY
            setIdeBottomHeight(Math.max(120, Math.min(560, ideDragRef.current.startHeight + delta)))
        }
        const onUp = () => {
            ideDragRef.current = null
            window.removeEventListener('mousemove', onMove)
            window.removeEventListener('mouseup', onUp)
        }
        window.addEventListener('mousemove', onMove)
        window.addEventListener('mouseup', onUp)
    }

    const handleEditorMount = (editor: any, monaco: Monaco) => {
        editor.addCommand(
            monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS,
            () => handleSaveRef.current(),
        )
        registerStarlarkCompletions(monaco)
    }

    if (!isNew && isLoadingScript && !formInitialised) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-56" />
                    <div className="h-64 bg-gray-200 dark:bg-slate-800 rounded-xl" />
                </div>
            </div>
        )
    }

    return (
        <>
        {/* ════════════════════════════════════════════════════════════════════
            DESKTOP IDE LAYOUT  (hidden on mobile, shown md+)
        ════════════════════════════════════════════════════════════════════ */}
        <div className="hidden md:flex flex-col h-full overflow-hidden bg-gray-50 dark:bg-slate-950">

            {/* ── Toolbar ─────────────────────────────────────────────────── */}
            <div className="flex-shrink-0 flex items-center gap-2 px-3 h-12 bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-800">
                <button
                    onClick={() => navigate('/scripts')}
                    title="Back to scripts"
                    className="p-1.5 text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 rounded-md hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors flex-shrink-0"
                >
                    <ArrowLeft className="w-4 h-4" />
                </button>
                <div className="w-px h-5 bg-gray-200 dark:bg-slate-700 flex-shrink-0" />

                <span className="text-sm font-semibold text-gray-800 dark:text-slate-200 truncate max-w-xs">
                    {name || (isNew ? 'New Script' : 'Script')}
                </span>
                {isDirty && (
                    <span className="inline-flex items-center gap-1 text-xs text-amber-500 dark:text-amber-400 flex-shrink-0">
                        <span className="w-1.5 h-1.5 rounded-full bg-amber-400 inline-block" />
                        unsaved
                    </span>
                )}

                <div className="ml-auto flex items-center gap-2 flex-shrink-0">
                    {/* Validate result badge */}
                    {validateResult !== null && (
                        <span className={clsx(
                            'flex items-center gap-1 text-xs px-2 py-0.5 rounded-full',
                            validateResult.valid
                                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                                : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                        )}>
                            {validateResult.valid
                                ? <><CheckCircle className="w-3 h-3" /> Valid</>
                                : <><XCircle className="w-3 h-3" /> Invalid</>
                            }
                        </span>
                    )}

                    {/* AI button */}
                    <div className="relative group">
                        <button
                            onClick={() => aiConfigured && setShowAIModal(true)}
                            disabled={!aiConfigured}
                            className={clsx(
                                'inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md transition-colors',
                                aiConfigured
                                    ? 'border border-purple-300 dark:border-purple-700 text-purple-700 dark:text-purple-300 hover:bg-purple-50 dark:hover:bg-purple-900/30'
                                    : 'border border-purple-200 dark:border-purple-900/40 text-purple-300 dark:text-purple-700 cursor-not-allowed'
                            )}
                        >
                            <Sparkles className="w-3.5 h-3.5" />
                            Generate with AI
                        </button>
                        {!aiConfigured && (
                            <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-1.5 text-xs text-white bg-gray-900 dark:bg-slate-700 rounded-lg whitespace-nowrap opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity z-10">
                                {aiProviderLabel} is not configured
                                <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900 dark:border-t-slate-700" />
                            </div>
                        )}
                    </div>

                    {/* Validate button */}
                    <button
                        onClick={handleValidate}
                        disabled={isValidating}
                        title="Validate Starlark syntax"
                        className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-md hover:bg-gray-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors"
                    >
                        {isValidating ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <CheckCircle className="w-3.5 h-3.5" />}
                        Validate
                    </button>

                    {/* Save button */}
                    <button
                        onClick={handleSave}
                        disabled={saveMutation.isPending || !name.trim()}
                        title="Save script (Ctrl+S)"
                        className="inline-flex items-center gap-1.5 px-3 py-1 text-xs bg-primary-600 text-white rounded-md hover:bg-primary-700 disabled:opacity-50 transition-colors"
                    >
                        {saveMutation.isPending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />}
                        {saveMutation.isPending ? 'Saving…' : 'Save'}
                    </button>

                    {saveMutation.isError && (
                        <span className="text-xs text-red-600 dark:text-red-400 flex items-center gap-1">
                            <XCircle className="w-3.5 h-3.5" />
                            {(saveMutation.error as Error).message}
                        </span>
                    )}
                </div>
            </div>

            {/* Validate error bar */}
            {validateResult?.error && (
                <div className="flex-shrink-0 px-4 py-1.5 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/40 text-xs text-red-700 dark:text-red-300 font-mono">
                    {validateResult.error}
                </div>
            )}

            {/* ── Body: editor + bottom panel (full width, no sidebar) ──────── */}
            <div className="flex flex-1 min-h-0 overflow-hidden">

                {/* Editor pane + resizable bottom panel */}
                <div className="flex flex-1 flex-col min-w-0 min-h-0 overflow-hidden">

                    {/* Monaco Editor */}
                    <div className="flex-1 min-h-0 overflow-hidden">
                        <Editor
                            key={scriptId ?? 'new'}
                            height="100%"
                            language="python"
                            value={source}
                            onChange={(val) => {
                                const v = val ?? ''
                                setSource(v)
                                sourceRef.current = v
                                setValidateResult(null)
                            }}
                            theme={isDarkTheme ? 'vs-dark' : 'light'}
                            onMount={handleEditorMount}
                            options={{
                                minimap: { enabled: true },
                                fontSize: 14,
                                lineNumbers: 'on',
                                scrollBeyondLastLine: false,
                                wordWrap: 'on',
                                tabSize: 4,
                                folding: true,
                                bracketPairColorization: { enabled: true },
                                automaticLayout: true,
                                padding: { top: 12 },
                            }}
                        />
                    </div>

                    {/* Drag handle */}
                    <div
                        className="flex-shrink-0 h-2 bg-gray-100 dark:bg-slate-800 border-y border-gray-200 dark:border-slate-700 cursor-row-resize hover:bg-primary-100 dark:hover:bg-primary-900/30 flex items-center justify-center select-none transition-colors"
                        onMouseDown={handleIdeDragStart}
                        title="Drag to resize panel"
                    >
                        <GripHorizontal className="w-5 h-3 text-gray-300 dark:text-slate-600" />
                    </div>

                    {/* Bottom panel */}
                    <div
                        className="flex-shrink-0 flex flex-col overflow-hidden bg-white dark:bg-slate-900"
                        style={{ height: ideBottomHeight }}
                    >
                        {/* Tab bar */}
                        <div className="flex-shrink-0 flex items-center border-b border-gray-200 dark:border-slate-800 px-1">
                            <button
                                onClick={() => setIdeActiveTab('test')}
                                className={clsx(
                                    'flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors',
                                    ideActiveTab === 'test'
                                        ? 'border-primary-600 text-primary-600 dark:text-primary-400 dark:border-primary-400'
                                        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
                                )}
                            >
                                <Play className="w-3 h-3" /> Test
                            </button>
                            <button
                                onClick={() => setIdeActiveTab('metadata')}
                                className={clsx(
                                    'flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors',
                                    ideActiveTab === 'metadata'
                                        ? 'border-primary-600 text-primary-600 dark:text-primary-400 dark:border-primary-400'
                                        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
                                )}
                            >
                                <Settings className="w-3 h-3" /> Metadata
                            </button>
                            <button
                                onClick={() => setIdeActiveTab('reference')}
                                className={clsx(
                                    'flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors',
                                    ideActiveTab === 'reference'
                                        ? 'border-primary-600 text-primary-600 dark:text-primary-400 dark:border-primary-400'
                                        : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
                                )}
                            >
                                <BookOpen className="w-3 h-3" /> Reference
                            </button>

                            {ideActiveTab === 'test' && isDirty && (
                                <span className="ml-3 inline-flex items-center gap-1 text-xs text-amber-500 dark:text-amber-400">
                                    <span className="w-1.5 h-1.5 rounded-full bg-amber-400 inline-block" />
                                    unsaved changes
                                </span>
                            )}
                            {ideActiveTab === 'test' && (
                                <button
                                    onClick={handleTest}
                                    disabled={isTesting}
                                    className="ml-auto mr-2 inline-flex items-center gap-1.5 px-2.5 py-1 text-xs bg-emerald-600 text-white rounded-md hover:bg-emerald-700 disabled:opacity-50 transition-colors"
                                >
                                    {isTesting ? <Loader2 className="w-3 h-3 animate-spin" /> : <Play className="w-3 h-3" />}
                                    Run
                                </button>
                            )}
                        </div>

                        {/* Tab content */}
                        <div className="flex-1 overflow-y-auto">
                            {ideActiveTab === 'test' && (
                                <div className="p-3 space-y-3">
                                    <p className="text-xs text-gray-400 dark:text-slate-500">
                                        Provide mock request data as JSON. Script receives via <code className="font-mono bg-gray-100 dark:bg-slate-800 px-0.5 rounded">req.path</code>, <code className="font-mono bg-gray-100 dark:bg-slate-800 px-0.5 rounded">req.query</code>, etc.
                                    </p>
                                    <div className="grid grid-cols-2 gap-2">
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Path params</label>
                                            <textarea
                                                value={testPath}
                                                onChange={(e) => setTestPath(e.target.value)}
                                                rows={2}
                                                className="w-full px-2 py-1.5 text-xs font-mono border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-primary-500 resize-none"
                                                placeholder='{"id": "123"}'
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Query params</label>
                                            <textarea
                                                value={testQuery}
                                                onChange={(e) => setTestQuery(e.target.value)}
                                                rows={2}
                                                className="w-full px-2 py-1.5 text-xs font-mono border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-primary-500 resize-none"
                                                placeholder='{"status": "active"}'
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Headers (lowercase keys)</label>
                                            <textarea
                                                value={testHeader}
                                                onChange={(e) => setTestHeader(e.target.value)}
                                                rows={2}
                                                className="w-full px-2 py-1.5 text-xs font-mono border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-primary-500 resize-none"
                                                placeholder='{"authorization": "Bearer token"}'
                                            />
                                        </div>
                                        <div>
                                            <label className="block text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Request body</label>
                                            <textarea
                                                value={testBody}
                                                onChange={(e) => setTestBody(e.target.value)}
                                                rows={2}
                                                className="w-full px-2 py-1.5 text-xs font-mono border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-primary-500 resize-none"
                                                placeholder='{"name": "Alice"}'
                                            />
                                        </div>
                                    </div>

                                    {testResult && (
                                        <div className="space-y-2">
                                            <div className="flex items-center gap-3 text-xs">
                                                {testResult.error
                                                    ? <span className="flex items-center gap-1 text-red-600 dark:text-red-400"><XCircle className="w-3.5 h-3.5" /> Error</span>
                                                    : <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400"><CheckCircle className="w-3.5 h-3.5" /> Success</span>
                                                }
                                                <span className="text-gray-400 dark:text-slate-500">{testResult.durationMs.toFixed(2)}ms</span>
                                            </div>
                                            {testResult.error && (
                                                <div className="bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-md p-2.5 text-xs font-mono text-red-700 dark:text-red-300">
                                                    {testResult.error}
                                                </div>
                                            )}
                                            {testResult.output !== null && testResult.output !== undefined && (
                                                <div>
                                                    <div className="text-xs font-medium text-gray-400 dark:text-slate-500 mb-1">Output</div>
                                                    <pre className="bg-gray-900 text-gray-100 rounded-md p-2.5 text-xs overflow-x-auto">
                                                        {JSON.stringify(testResult.output, null, 2)}
                                                    </pre>
                                                </div>
                                            )}
                                            {testResult.logs && testResult.logs.length > 0 && (
                                                <div>
                                                    <div className="text-xs font-medium text-gray-400 dark:text-slate-500 mb-1">Logs</div>
                                                    <div className="bg-gray-900 rounded-md p-2.5 space-y-0.5">
                                                        {testResult.logs.map((line, i) => (
                                                            <div key={i} className="text-xs font-mono text-emerald-300">
                                                                <span className="select-none text-gray-500 mr-2">{String(i + 1).padStart(2, '0')}</span>{line}
                                                            </div>
                                                        ))}
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    )}
                                </div>
                            )}

                            {ideActiveTab === 'reference' && (
                                <div className="p-3">
                                    <BuiltinReferenceContent />
                                </div>
                            )}

                            {ideActiveTab === 'metadata' && (
                                <div className="p-4 space-y-4 max-w-2xl">
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        {/* Name */}
                                        <div>
                                            <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">
                                                Name <span className="text-red-500">*</span>
                                            </label>
                                            <input
                                                type="text"
                                                value={name}
                                                onChange={(e) => setName(e.target.value)}
                                                placeholder="e.g. Enrich User Data"
                                                className="w-full px-2.5 py-1.5 border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-xs"
                                            />
                                        </div>

                                        {/* Description */}
                                        <div>
                                            <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">Description</label>
                                            <input
                                                type="text"
                                                value={description}
                                                onChange={(e) => setDescription(e.target.value)}
                                                placeholder="Optional description"
                                                className="w-full px-2.5 py-1.5 border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-xs"
                                            />
                                        </div>

                                        {/* Timeout */}
                                        <div>
                                            <label className="block text-xs font-medium text-gray-700 dark:text-slate-300 mb-1">
                                                <span className="flex items-center gap-1"><Clock className="w-3 h-3" /> Timeout (ms)</span>
                                            </label>
                                            <input
                                                type="number"
                                                value={timeout}
                                                min={0}
                                                onChange={(e) => { const v = Math.max(0, parseInt(e.target.value) || 0); setTimeout(v); timeoutRef.current = v; }}
                                                placeholder="0 = global default"
                                                className="w-full px-2.5 py-1.5 border border-gray-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-xs"
                                            />
                                            <p className="text-[10px] text-gray-400 dark:text-slate-500 mt-1">0 = use global default from config</p>
                                        </div>

                                        {/* Enabled */}
                                        <div className="flex items-center gap-3 pt-4">
                                            <label className="text-xs font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                                            <button
                                                type="button"
                                                onClick={() => setEnabled(!enabled)}
                                                className={clsx(
                                                    'transition-colors',
                                                    enabled ? 'text-emerald-600 hover:text-emerald-700' : 'text-gray-400 dark:text-slate-500 hover:text-gray-600'
                                                )}
                                            >
                                                {enabled ? <ToggleRight className="w-7 h-7" /> : <ToggleLeft className="w-7 h-7" />}
                                            </button>
                                        </div>
                                    </div>

                                    <div className="pt-1 text-[10px] text-gray-400 dark:text-slate-500">
                                        Press <kbd className="font-mono bg-gray-100 dark:bg-slate-700 px-1 rounded">Ctrl+S</kbd> to save after editing
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>

        {/* ════════════════════════════════════════════════════════════════════
            MOBILE LAYOUT  (shown on mobile, hidden md+)
        ════════════════════════════════════════════════════════════════════ */}
        <div className="md:hidden p-4 space-y-6">
            {/* Header */}
            <div className="flex items-center gap-4">
                <button
                    onClick={() => navigate('/scripts')}
                    className="p-2 text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                >
                    <ArrowLeft className="w-5 h-5" />
                </button>
                <div>
                    <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">
                        {isNew ? 'New Script' : 'Edit Script'}
                    </h1>
                    <p className="text-gray-500 dark:text-slate-400 mt-0.5 text-sm">
                        Starlark scripts run per request and expose output via{' '}
                        <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">{'{{.script.<key>.*}}'}</code>
                    </p>
                </div>
            </div>

            {/* Metadata card */}
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">Metadata</h2>
                <div className="grid grid-cols-1 gap-4">
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                            Name <span className="text-red-500">*</span>
                        </label>
                        <input
                            type="text"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder="e.g. Enrich User Data"
                            className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">Description</label>
                        <input
                            type="text"
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            placeholder="Optional description"
                            className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                        />
                    </div>
                    <div className="flex gap-6">
                        <div className="flex-1">
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                <span className="flex items-center gap-1.5"><Clock className="w-4 h-4" /> Timeout (ms)</span>
                            </label>
                            <input
                                type="number"
                                value={timeout}
                                min={0}
                                onChange={(e) => { const v = Math.max(0, parseInt(e.target.value) || 0); setTimeout(v); timeoutRef.current = v; }}
                                placeholder="0 = global default"
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                            />
                            <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">0 = use global default from config</p>
                        </div>
                        <div className="flex items-end gap-3 pb-0.5">
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300">Enabled</label>
                            <button
                                type="button"
                                onClick={() => setEnabled(!enabled)}
                                className={clsx(
                                    'transition-colors',
                                    enabled ? 'text-emerald-600 hover:text-emerald-700' : 'text-gray-400 dark:text-slate-500 hover:text-gray-600'
                                )}
                            >
                                {enabled ? <ToggleRight className="w-8 h-8" /> : <ToggleLeft className="w-8 h-8" />}
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            {/* Source editor card */}
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 overflow-hidden">
                <div className="p-4 border-b border-gray-200 dark:border-slate-800 flex items-center justify-between">
                    <div>
                        <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100">Source</h2>
                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-0.5">Starlark — must define a <code className="font-mono">run(req)</code> function</p>
                    </div>
                    <div className="flex items-center gap-2">
                        {validateResult !== null && (
                            <span className={clsx(
                                'flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full',
                                validateResult.valid
                                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                                    : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                            )}>
                                {validateResult.valid ? <><CheckCircle className="w-3.5 h-3.5" /> Valid</> : <><XCircle className="w-3.5 h-3.5" /> Invalid</>}
                            </span>
                        )}
                        <div className="relative group">
                            <button
                                onClick={() => aiConfigured && setShowAIModal(true)}
                                disabled={!aiConfigured}
                                className={clsx(
                                    'inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-colors',
                                    aiConfigured
                                        ? 'border border-purple-300 dark:border-purple-700 text-purple-700 dark:text-purple-300 hover:bg-purple-50 dark:hover:bg-purple-900/30'
                                        : 'border border-purple-200 dark:border-purple-900/40 text-purple-300 dark:text-purple-700 cursor-not-allowed'
                                )}
                            >
                                <Sparkles className="w-4 h-4" />
                                Generate with AI
                            </button>
                            {!aiConfigured && (
                                <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-3 py-1.5 text-xs text-white bg-gray-900 dark:bg-slate-700 rounded-lg whitespace-nowrap opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity z-10">
                                    {aiProviderLabel} is not configured
                                    <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900 dark:border-t-slate-700" />
                                </div>
                            )}
                        </div>
                        <button
                            onClick={handleValidate}
                            disabled={isValidating}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors"
                        >
                            {isValidating ? <Loader2 className="w-4 h-4 animate-spin" /> : <CheckCircle className="w-4 h-4" />}
                            Validate
                        </button>
                    </div>
                </div>
                {validateResult?.error && (
                    <div className="px-4 py-2 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/40 text-xs text-red-700 dark:text-red-300 font-mono">
                        {validateResult.error}
                    </div>
                )}
                <Editor
                    key={scriptId ?? 'new'}
                    height="360px"
                    language="python"
                    value={source}
                    onChange={(val) => {
                        const v = val ?? ''
                        setSource(v)
                        sourceRef.current = v
                        setValidateResult(null)
                    }}
                    theme={isDarkTheme ? 'vs-dark' : 'light'}
                    onMount={handleEditorMount}
                    options={{
                        minimap: { enabled: false },
                        fontSize: 13,
                        lineNumbers: 'on',
                        scrollBeyondLastLine: false,
                        wordWrap: 'on',
                        tabSize: 4,
                    }}
                />
            </div>

            {/* Test panel */}
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 overflow-hidden">
                <button
                    onClick={() => setTestOpen(!testOpen)}
                    className="w-full flex items-center justify-between px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-slate-800/50 transition-colors"
                >
                    <div className="flex items-center gap-2">
                        <Play className="w-4 h-4 text-gray-500 dark:text-slate-400" />
                        <span className="text-base font-semibold text-gray-900 dark:text-slate-100">Test Execution</span>
                        {isDirty
                            ? <span className="inline-flex items-center gap-1 text-xs text-amber-500 dark:text-amber-400 ml-1">
                                <span className="w-1.5 h-1.5 rounded-full bg-amber-400 inline-block" />
                                unsaved changes
                              </span>
                            : <span className="text-xs text-gray-400 dark:text-slate-500 ml-1">(runs current editor source)</span>
                        }
                    </div>
                    {testOpen ? <ChevronUp className="w-5 h-5 text-gray-400" /> : <ChevronDown className="w-5 h-5 text-gray-400" />}
                </button>

                {testOpen && (
                    <div className="px-6 pb-6 space-y-4 border-t border-gray-200 dark:border-slate-800 pt-4">
                        <p className="text-sm text-gray-500 dark:text-slate-400">
                            Provide mock request data as JSON objects.
                        </p>
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">Path params (JSON object)</label>
                                <textarea value={testPath} onChange={(e) => setTestPath(e.target.value)} rows={2}
                                    className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                    placeholder='{"id": "123"}' />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">Query params (JSON object)</label>
                                <textarea value={testQuery} onChange={(e) => setTestQuery(e.target.value)} rows={2}
                                    className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                    placeholder='{"status": "active"}' />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">Headers (JSON object, lowercase keys)</label>
                                <textarea value={testHeader} onChange={(e) => setTestHeader(e.target.value)} rows={2}
                                    className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                    placeholder='{"authorization": "Bearer token"}' />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">Request body (JSON or null)</label>
                                <textarea value={testBody} onChange={(e) => setTestBody(e.target.value)} rows={2}
                                    className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                    placeholder='{"name": "Alice"}' />
                            </div>
                        </div>
                        <button
                            onClick={handleTest}
                            disabled={isTesting}
                            className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 disabled:opacity-50 transition-colors text-sm"
                        >
                            {isTesting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                            Run Script
                        </button>
                        {testResult && (
                            <div className="space-y-3">
                                <div className="flex items-center gap-3 text-sm">
                                    {testResult.error
                                        ? <span className="flex items-center gap-1.5 text-red-600 dark:text-red-400"><XCircle className="w-4 h-4" /> Execution error</span>
                                        : <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400"><CheckCircle className="w-4 h-4" /> Success</span>
                                    }
                                    <span className="text-gray-400 dark:text-slate-500">{testResult.durationMs.toFixed(2)}ms</span>
                                </div>
                                {testResult.error && (
                                    <div className="bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3 text-sm font-mono text-red-700 dark:text-red-300">
                                        {testResult.error}
                                    </div>
                                )}
                                {testResult.output !== null && testResult.output !== undefined && (
                                    <div>
                                        <div className="text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Output</div>
                                        <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 text-sm overflow-x-auto">{JSON.stringify(testResult.output, null, 2)}</pre>
                                    </div>
                                )}
                                {testResult.logs && testResult.logs.length > 0 && (
                                    <div>
                                        <div className="text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Logs</div>
                                        <div className="bg-gray-900 rounded-lg p-4 space-y-0.5">
                                            {testResult.logs.map((line, i) => (
                                                <div key={i} className="text-sm font-mono text-emerald-300">
                                                    <span className="select-none text-gray-500 mr-2">{String(i + 1).padStart(2, '0')}</span>{line}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* Builtin Reference panel */}
            <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 overflow-hidden">
                <button
                    onClick={() => setRefOpen(!refOpen)}
                    className="w-full flex items-center justify-between px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-slate-800/50 transition-colors"
                >
                    <div className="flex items-center gap-2">
                        <BookOpen className="w-4 h-4 text-gray-500 dark:text-slate-400" />
                        <span className="text-base font-semibold text-gray-900 dark:text-slate-100">Builtin Reference</span>
                        <span className="text-xs text-gray-400 dark:text-slate-500 ml-1">all available functions and modules</span>
                    </div>
                    {refOpen ? <ChevronUp className="w-5 h-5 text-gray-400" /> : <ChevronDown className="w-5 h-5 text-gray-400" />}
                </button>
                {refOpen && (
                    <div className="px-6 pb-6 border-t border-gray-200 dark:border-slate-800 pt-4">
                        <BuiltinReferenceContent />
                    </div>
                )}
            </div>

            {/* Save actions */}
            <div className="flex items-center gap-3">
                <button
                    onClick={handleSave}
                    disabled={saveMutation.isPending || !name.trim()}
                    className="inline-flex items-center gap-2 px-5 py-2.5 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
                >
                    {saveMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
                    {saveMutation.isPending ? 'Saving…' : 'Save Script'}
                </button>
                <button
                    onClick={() => navigate('/scripts')}
                    className="px-4 py-2.5 border border-gray-300 dark:border-slate-600 text-gray-700 dark:text-slate-300 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                >
                    Cancel
                </button>
                {saveMutation.isError && (
                    <span className="text-sm text-red-600 dark:text-red-400 flex items-center gap-1">
                        <XCircle className="w-4 h-4" />
                        {(saveMutation.error as Error).message}
                    </span>
                )}
            </div>
        </div>

        {showAIModal && (
            <AIScriptModal
                currentSource={source}
                history={aiHistory}
                onHistoryChange={setAiHistory}
                onGenerated={(generatedSource) => {
                    setSource(generatedSource)
                    setValidateResult(null)
                }}
                onClose={() => setShowAIModal(false)}
            />
        )}
        </>
    )
}

// ─── Reference content (shared between desktop IDE and mobile accordion) ─────

function BuiltinReferenceContent() {
    return (
        <div className="space-y-5 text-sm">
            <RefSection title="Request Object — req">
                <div className="text-xs text-gray-400 dark:text-slate-500 mb-1">
                    <code className="font-mono">req</code> is passed to <code className="font-mono">run(req)</code>. Each attribute is a callable, not a dict.
                </div>
                <RefEntry sig='req.path("key")' ret="string" desc="Path param — raises if missing" />
                <RefEntry sig='req.path("key", "")' ret="string" desc="Path param with default" />
                <RefEntry sig='req.query("key", "default")' ret="string" desc="Query parameter with default" />
                <RefEntry sig='req.header("authorization", "")' ret="string" desc="Request header (auto-lowercased)" />
                <RefEntry sig='req.body()' ret="dict|list|None" desc="Whole parsed JSON body" />
                <RefEntry sig='req.body("name", "")' ret="value" desc="Top-level body field with default" />
                <RefEntry sig='req.body("user.name", "")' ret="value" desc="Nested gjson path" />
                <RefEntry sig='req.body("items.0.id", None)' ret="value" desc="Array index navigation" />
                <div className="text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30 rounded px-2 py-1 mt-1">
                    ⚠ Do NOT use <code className="font-mono">req["path"]</code> or <code className="font-mono">req["body"]</code> — req is not a dict.
                </div>
            </RefSection>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                <RefSection title="Identifiers &amp; Time">
                    <RefEntry sig="uuid()" ret="string" desc="Random UUID v4" />
                    <RefEntry sig='now()' ret="int" desc="Unix timestamp (seconds)" />
                    <RefEntry sig='now("unix_ms")' ret="int" desc="Milliseconds" />
                    <RefEntry sig='now("iso")' ret="string" desc='RFC3339 e.g. "2025-01-15T10:30:00Z"' />
                    <RefEntry sig='now("date")' ret="string" desc='"YYYY-MM-DD"' />
                    <RefEntry sig="sleep(ms)" ret="None" desc="Pause for ms milliseconds" />
                </RefSection>

                <RefSection title="Random">
                    <RefEntry sig="rand_int(max)" ret="int" desc="[0, max] inclusive" />
                    <RefEntry sig="rand_int(min, max)" ret="int" desc="[min, max] inclusive" />
                    <RefEntry sig="rand_choice(list)" ret="value" desc="Random element from list" />
                    <RefEntry sig='counter("name")' ret="int" desc="Increment by 1, return new value" />
                    <RefEntry sig='counter("name", n)' ret="int" desc="Increment by n (0 = read)" />
                </RefSection>

                <RefSection title="Encoding &amp; JSON">
                    <RefEntry sig="base64_encode(str)" ret="string" desc="Standard base64" />
                    <RefEntry sig="base64_decode(str)" ret="string" desc="Decode base64" />
                    <RefEntry sig='hash("sha256", str)' ret="string" desc="Hex digest. Algos: md5, sha1, sha256, sha512" />
                    <RefEntry sig="json_parse(str)" ret="value" desc="JSON string → dict/list" />
                    <RefEntry sig="json_stringify(value)" ret="string" desc="Value → JSON string" />
                </RefSection>

                <RefSection title="Regex">
                    <RefEntry sig="regex_match(pattern, str)" ret="bool" desc="True if pattern matches anywhere" />
                    <RefEntry sig="regex_find(pattern, str)" ret="string|None" desc="First match or None" />
                    <RefEntry sig="regex_find_all(pattern, str)" ret="list" desc="All non-overlapping matches" />
                </RefSection>

                <RefSection title="Store (session key-value)">
                    <RefEntry sig='store.get("key")' ret="value|None" desc="Read value" />
                    <RefEntry sig='store.get("key", default)' ret="value" desc="Read with fallback" />
                    <RefEntry sig='store.set("key", value)' ret="None" desc="Write value" />
                    <RefEntry sig='store.has("key")' ret="bool" desc="Check existence" />
                    <RefEntry sig='store.delete("key")' ret="None" desc="Remove key" />
                    <RefEntry sig="store.keys()" ret="list" desc="All session keys" />
                </RefSection>

                <RefSection title='Collections (global) — store.collection("name")'>
                    <div className="text-xs text-gray-400 dark:text-slate-500 mb-1">Global across all sessions. Use for shared data sets.</div>
                    <RefEntry sig='col = store.collection("name")' ret="col" desc="Get collection handle" />
                    <RefEntry sig="col.findAll()" ret="list" desc="All documents" />
                    <RefEntry sig='col.findAll({"field": "value"})' ret="list" desc="Filtered (equality)" />
                    <RefEntry sig='col.findOne({"id": "x"})' ret="dict|None" desc="First match or None" />
                    <RefEntry sig="col.insert({...})" ret="None" desc="Append document" />
                    <RefEntry sig='col.update({"id":"x"}, {...})' ret="None" desc="Update matching docs" />
                    <RefEntry sig='col.remove({"id": "x"})' ret="None" desc="Remove matching docs" />
                    <RefEntry sig="col.count()" ret="int" desc="Total documents" />
                    <RefEntry sig='col.count({"status":"ok"})' ret="int" desc="Matching documents" />
                    <RefEntry sig="col.clear()" ret="None" desc="Remove all documents" />
                </RefSection>

                <RefSection title="Logging">
                    <RefEntry sig="log(...)" ret="None" desc="Append to request trace log" />
                    <div className="text-xs text-gray-400 dark:text-slate-500 font-mono mt-1">log("msg", value, ...)</div>
                </RefSection>
            </div>

            <RefSection title="datetime module">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6">
                    <div className="space-y-0.5">
                        <div className="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wide mb-1">datetime.date</div>
                        <RefEntry sig="datetime.date(year, month, day)" ret="date" />
                        <RefEntry sig="datetime.date.today()" ret="date" desc="Today UTC" />
                        <RefEntry sig='datetime.date.fromisoformat("YYYY-MM-DD")' ret="date" />
                        <RefEntry sig="date.year / .month / .day" ret="int" />
                        <RefEntry sig="date.weekday()" ret="int" desc="0=Mon … 6=Sun" />
                        <RefEntry sig="date.isoformat()" ret="string" desc='"YYYY-MM-DD"' />
                        <RefEntry sig='date.strftime("2006/01/02")' ret="string" desc="Go layout tokens" />
                        <RefEntry sig="date + timedelta" ret="date" />
                        <RefEntry sig="date - timedelta" ret="date" />
                        <RefEntry sig="date - date" ret="timedelta" />
                        <RefEntry sig="date == / < / > date" ret="bool" />
                    </div>
                    <div className="space-y-0.5">
                        <div className="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wide mb-1">datetime.datetime</div>
                        <RefEntry sig="datetime.datetime(year, month, day, hour?, minute?, second?)" ret="datetime" />
                        <RefEntry sig="datetime.datetime.now()" ret="datetime" />
                        <RefEntry sig='datetime.datetime.fromisoformat(str)' ret="datetime" desc="RFC3339 / ISO" />
                        <RefEntry sig="datetime.datetime.fromtimestamp(secs)" ret="datetime" />
                        <RefEntry sig="dt.year / .month / .day / .hour / .minute / .second" ret="int" />
                        <RefEntry sig="dt.date()" ret="date" desc="Strip time" />
                        <RefEntry sig="dt.isoformat()" ret="string" desc="RFC3339" />
                        <RefEntry sig="dt.timestamp()" ret="int" desc="Unix seconds" />
                        <RefEntry sig="dt +/- timedelta" ret="datetime" />
                        <RefEntry sig="dt - datetime" ret="timedelta" />
                        <div className="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wide mb-1 mt-2">datetime.timedelta</div>
                        <RefEntry sig="datetime.timedelta(days?, hours?, minutes?, seconds?)" ret="timedelta" />
                        <RefEntry sig="td.days / .hours / .minutes / .seconds" ret="int" />
                        <RefEntry sig="td.total_seconds()" ret="float" />
                        <RefEntry sig="timedelta +/- timedelta" ret="timedelta" />
                    </div>
                </div>
                <div className="mt-2 text-xs text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30 rounded px-2 py-1">
                    ⚠ strftime uses Go layout tokens, not Python %d/%m/%Y — use <code className="font-mono">2006</code> for year, <code className="font-mono">01</code> for month, <code className="font-mono">02</code> for day.
                </div>
            </RefSection>

            <RefSection title="validate module">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6">
                    <div className="space-y-0.5">
                        <RefEntry sig='validate.matches(value, "uuid")' ret="bool" desc="Token name or raw regex" />
                        <RefEntry sig="validate.regex(value, pattern)" ret="bool" desc="Raw regex only" />
                        <RefEntry sig="validate.pattern_names()" ret="list" desc="All registered token names" />
                        <div className="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wide mt-2 mb-1">Convenience helpers</div>
                        <RefEntry sig="validate.is_uuid(v)" ret="bool" />
                        <RefEntry sig="validate.is_email(v)" ret="bool" />
                        <RefEntry sig="validate.is_url(v)" ret="bool" />
                        <RefEntry sig="validate.is_ipv4(v) / is_ipv6(v) / is_ip(v)" ret="bool" />
                        <RefEntry sig="validate.is_us_phone(v)" ret="bool" />
                        <RefEntry sig="validate.is_us_zip(v)" ret="bool" />
                        <RefEntry sig="validate.is_ssn(v)" ret="bool" />
                    </div>
                    <div className="space-y-0.5">
                        <RefEntry sig="validate.is_date_iso(v)" ret="bool" desc='"YYYY-MM-DD"' />
                        <RefEntry sig="validate.is_datetime_iso(v)" ret="bool" />
                        <RefEntry sig="validate.is_integer(v)" ret="bool" />
                        <RefEntry sig="validate.is_decimal(v)" ret="bool" />
                        <RefEntry sig="validate.is_semver(v)" ret="bool" />
                        <RefEntry sig="validate.is_jwt(v)" ret="bool" />
                        <RefEntry sig="validate.is_slug(v)" ret="bool" />
                        <RefEntry sig="validate.is_base64(v)" ret="bool" />
                        <RefEntry sig="validate.is_hex_color(v)" ret="bool" />
                        <RefEntry sig="validate.is_credit_card(v)" ret="bool" />
                        <RefEntry sig="validate.is_iban(v)" ret="bool" />
                    </div>
                </div>
            </RefSection>
        </div>
    )
}

// ─── Reference panel helpers ──────────────────────────────────────────────────

function RefSection({ title, children }: { title: string; children: React.ReactNode }) {
    return (
        <div>
            <div
                className="text-xs font-bold text-primary-600 dark:text-primary-400 uppercase tracking-wider mb-2 border-b border-gray-100 dark:border-slate-800 pb-1"
                dangerouslySetInnerHTML={{ __html: title }}
            />
            <div className="space-y-0.5">{children}</div>
        </div>
    )
}

function RefEntry({ sig, ret, desc }: { sig: string; ret?: string; desc?: string }) {
    return (
        <div className="flex flex-wrap items-baseline gap-x-2 text-xs py-0.5">
            <code className="font-mono text-gray-800 dark:text-slate-200 text-[11px]">{sig}</code>
            {ret && <span className="text-[10px] text-emerald-600 dark:text-emerald-400 font-mono shrink-0">→ {ret}</span>}
            {desc && <span className="text-gray-400 dark:text-slate-500">{desc}</span>}
        </div>
    )
}
