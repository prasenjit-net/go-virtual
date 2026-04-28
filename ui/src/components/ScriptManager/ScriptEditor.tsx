import { useState, useCallback } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import Editor from '@monaco-editor/react'
import {
    ArrowLeft, Save, CheckCircle, XCircle, Play, ChevronDown, ChevronUp,
    Clock, ToggleLeft, ToggleRight, Loader2, Sparkles
} from 'lucide-react'
import clsx from 'clsx'
import { scriptsApi, aiApi } from '../../services/api'
import type { AIStatus, Script } from '../../types'
import AIScriptModal from './AIScriptModal'

const DEFAULT_SOURCE = `# Starlark script — define a run(req) function that returns a dict.
# req.path   → dict of path parameters
# req.query  → dict of query parameters
# req.header → dict of request headers (lowercased keys)
# req.body   → parsed JSON body (or None)
# store      → session key-value store (get, set, has, delete, keys)
# log(...)   → append message to trace logs

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

    // Form state
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [timeout, setTimeout] = useState(0)
    const [enabled, setEnabled] = useState(true)
    const [source, setSource] = useState(DEFAULT_SOURCE)
    const [formInitialised, setFormInitialised] = useState(false)

    // Validate state
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

    const { data: aiStatus = { configured: true, provider: 'openai' } } = useQuery<AIStatus>({
        queryKey: ['ai-status'],
        queryFn: () => aiApi.getStatus(),
        staleTime: 60_000,
    })
    const aiConfigured = aiStatus.configured
    const aiProviderLabel = aiStatus.provider === 'claude' ? 'Claude' : aiStatus.provider === 'openai' ? 'OpenAI' : 'AI provider'

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
                setEnabled(data.enabled)
                if (data.source !== undefined) {
                    setSource(data.source || '')
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
            // Discard the AI conversation context on save.
            setAiHistory([])
            navigate(`/scripts`)
        },
    })

    const handleSave = () => {
        if (!name.trim()) return
        saveMutation.mutate({ name: name.trim(), description, source, timeout, enabled })
    }

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
        if (isNew || !scriptId) return
        setIsTesting(true)
        setTestResult(null)
        try {
            let parsedPath: Record<string, string> = {}
            let parsedQuery: Record<string, string> = {}
            let parsedHeader: Record<string, string> = {}
            let parsedBody: any = null
            try { parsedPath = JSON.parse(testPath) } catch { /* ignore */ }
            try { parsedQuery = JSON.parse(testQuery) } catch { /* ignore */ }
            try { parsedHeader = JSON.parse(testHeader) } catch { /* ignore */ }
            try { parsedBody = JSON.parse(testBody) } catch { /* ignore */ }
            const result = await scriptsApi.test(scriptId, {
                path: parsedPath,
                query: parsedQuery,
                header: parsedHeader,
                body: parsedBody,
            })
            setTestResult(result)
        } catch (e) {
            setTestResult({ output: null, durationMs: 0, error: (e as Error).message })
        } finally {
            setIsTesting(false)
        }
    }, [scriptId, isNew, testPath, testQuery, testHeader, testBody])

    if (!isNew && isLoadingScript && !formInitialised) {
        return (
            <div className="p-8">
                <div className="animate-pulse space-y-4">
                    <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-56"></div>
                    <div className="h-64 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
                </div>
            </div>
        )
    }

    return (
        <>
        <div className="p-8 max-w-5xl">
            {/* Header */}
            <div className="flex items-center gap-4 mb-8">
                <button
                    onClick={() => navigate('/scripts')}
                    className="p-2 text-gray-400 dark:text-slate-500 hover:text-gray-700 dark:hover:text-slate-300 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-800 transition-colors"
                >
                    <ArrowLeft className="w-5 h-5" />
                </button>
                <div>
                    <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">
                        {isNew ? 'New Script' : 'Edit Script'}
                    </h1>
                    <p className="text-gray-500 dark:text-slate-400 mt-0.5 text-sm">
                        Starlark scripts run per request and expose output via{' '}
                        <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">{'{{.script.<key>.*}}'}</code>
                    </p>
                </div>
            </div>

            <div className="space-y-6">
                {/* Metadata card */}
                <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 p-6">
                    <h2 className="text-base font-semibold text-gray-900 dark:text-slate-100 mb-4">Metadata</h2>
                    <div className="grid grid-cols-1 gap-4">
                        {/* Name */}
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

                        {/* Description */}
                        <div>
                            <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                Description
                            </label>
                            <input
                                type="text"
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                placeholder="Optional description"
                                className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                            />
                        </div>

                        <div className="flex gap-6">
                            {/* Timeout */}
                            <div className="flex-1">
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300 mb-1.5">
                                    <span className="flex items-center gap-1.5">
                                        <Clock className="w-4 h-4" />
                                        Timeout (ms)
                                    </span>
                                </label>
                                <input
                                    type="number"
                                    value={timeout}
                                    min={0}
                                    onChange={(e) => setTimeout(Math.max(0, parseInt(e.target.value) || 0))}
                                    placeholder="0 = global default"
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 text-sm"
                                />
                                <p className="text-xs text-gray-400 dark:text-slate-500 mt-1">0 = use global default from config</p>
                            </div>

                            {/* Enabled */}
                            <div className="flex items-end gap-3 pb-0.5">
                                <label className="block text-sm font-medium text-gray-700 dark:text-slate-300">
                                    Enabled
                                </label>
                                <button
                                    type="button"
                                    onClick={() => setEnabled(!enabled)}
                                    className={clsx(
                                        'transition-colors',
                                        enabled
                                            ? 'text-emerald-600 hover:text-emerald-700'
                                            : 'text-gray-400 dark:text-slate-500 hover:text-gray-600'
                                    )}
                                >
                                    {enabled
                                        ? <ToggleRight className="w-8 h-8" />
                                        : <ToggleLeft className="w-8 h-8" />
                                    }
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
                            <p className="text-xs text-gray-400 dark:text-slate-500 mt-0.5">Starlark (Python-like) — must define a <code className="font-mono">run(req)</code> function that returns a dict</p>
                        </div>
                        <div className="flex items-center gap-2">
                            {/* Validate result badge */}
                            {validateResult !== null && (
                                <span className={clsx(
                                    'flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full',
                                    validateResult.valid
                                        ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                                        : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                )}>
                                    {validateResult.valid
                                        ? <><CheckCircle className="w-3.5 h-3.5" /> Valid</>
                                        : <><XCircle className="w-3.5 h-3.5" /> Invalid</>
                                    }
                                </span>
                            )}
                            <div className="relative group">
                                <button
                                    onClick={() => aiConfigured && setShowAIModal(true)}
                                    disabled={!aiConfigured}
                                    className={clsx(
                                        "inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg transition-colors",
                                        aiConfigured
                                            ? "border border-purple-300 dark:border-purple-700 text-purple-700 dark:text-purple-300 hover:bg-purple-50 dark:hover:bg-purple-900/30"
                                            : "border border-purple-200 dark:border-purple-900/40 text-purple-300 dark:text-purple-700 cursor-not-allowed"
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
                        height="360px"
                        language="python"
                        value={source}
                        onChange={(val) => {
                            setSource(val ?? '')
                            setValidateResult(null)
                        }}
                        theme="vs-dark"
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

                {/* Test panel (only in edit mode — needs a saved script to execute) */}
                {!isNew && (
                    <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-gray-200 dark:border-slate-800 overflow-hidden">
                        <button
                            onClick={() => setTestOpen(!testOpen)}
                            className="w-full flex items-center justify-between px-6 py-4 text-left hover:bg-gray-50 dark:hover:bg-slate-800/50 transition-colors"
                        >
                            <div className="flex items-center gap-2">
                                <Play className="w-4 h-4 text-gray-500 dark:text-slate-400" />
                                <span className="text-base font-semibold text-gray-900 dark:text-slate-100">Test Execution</span>
                                <span className="text-xs text-gray-400 dark:text-slate-500 ml-1">(runs the last saved version)</span>
                            </div>
                            {testOpen ? <ChevronUp className="w-5 h-5 text-gray-400" /> : <ChevronDown className="w-5 h-5 text-gray-400" />}
                        </button>

                        {testOpen && (
                            <div className="px-6 pb-6 space-y-4 border-t border-gray-200 dark:border-slate-800 pt-4">
                                <p className="text-sm text-gray-500 dark:text-slate-400">
                                    Provide mock request data as JSON objects. The script receives them via <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">req.path</code>, <code className="font-mono text-xs bg-gray-100 dark:bg-slate-800 px-1 rounded">req.query</code>, etc.
                                </p>

                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">
                                            Path params (JSON object)
                                        </label>
                                        <textarea
                                            value={testPath}
                                            onChange={(e) => setTestPath(e.target.value)}
                                            rows={2}
                                            className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                            placeholder='{"id": "123"}'
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">
                                            Query params (JSON object)
                                        </label>
                                        <textarea
                                            value={testQuery}
                                            onChange={(e) => setTestQuery(e.target.value)}
                                            rows={2}
                                            className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                            placeholder='{"status": "active"}'
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">
                                            Headers (JSON object, lowercase keys)
                                        </label>
                                        <textarea
                                            value={testHeader}
                                            onChange={(e) => setTestHeader(e.target.value)}
                                            rows={2}
                                            className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                            placeholder='{"authorization": "Bearer token"}'
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-400 mb-1">
                                            Request body (JSON or null)
                                        </label>
                                        <textarea
                                            value={testBody}
                                            onChange={(e) => setTestBody(e.target.value)}
                                            rows={2}
                                            className="w-full px-3 py-2 text-sm font-mono border border-gray-300 dark:border-slate-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                                            placeholder='{"name": "Alice"}'
                                        />
                                    </div>
                                </div>

                                <button
                                    onClick={handleTest}
                                    disabled={isTesting}
                                    className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 disabled:opacity-50 transition-colors text-sm"
                                >
                                    {isTesting
                                        ? <Loader2 className="w-4 h-4 animate-spin" />
                                        : <Play className="w-4 h-4" />
                                    }
                                    Run Script
                                </button>

                                {testResult && (
                                    <div className="space-y-3">
                                        <div className="flex items-center gap-3 text-sm">
                                            {testResult.error
                                                ? <span className="flex items-center gap-1.5 text-red-600 dark:text-red-400"><XCircle className="w-4 h-4" /> Execution error</span>
                                                : <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400"><CheckCircle className="w-4 h-4" /> Success</span>
                                            }
                                            <span className="text-gray-400 dark:text-slate-500">
                                                {testResult.durationMs.toFixed(2)}ms
                                            </span>
                                        </div>

                                        {testResult.error && (
                                            <div className="bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/40 rounded-lg p-3 text-sm font-mono text-red-700 dark:text-red-300">
                                                {testResult.error}
                                            </div>
                                        )}

                                        {testResult.output !== null && testResult.output !== undefined && (
                                            <div>
                                                <div className="text-xs font-medium text-gray-500 dark:text-slate-400 mb-1">Output</div>
                                                <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 text-sm overflow-x-auto">
                                                    {JSON.stringify(testResult.output, null, 2)}
                                                </pre>
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
                )}

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
