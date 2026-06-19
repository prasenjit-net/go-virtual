import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
    ArrowLeft, ArrowDownUp, Bot, Radio, Globe, Fingerprint, Copy, Check,
    Clock, Activity, ChevronDown, ChevronRight as ChevronRightIcon,
    Code2, ShieldCheck, Database, Terminal, Users, Layers,
} from 'lucide-react'
import clsx from 'clsx'
import { tracesApi } from '../../services/api'
import type {
    Trace, PipelineTraceItem, ScriptTrace, ValidationTrace, CollectionTrace, SessionTrace,
} from '../../types'

// ── Helpers ──────────────────────────────────────────────────────────────────

const methodColors: Record<string, string> = {
    GET:    'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
    POST:   'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
    PUT:    'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300',
    DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    PATCH:  'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
}

const skipReasonLabels: Record<string, string> = {
    disabled:              'disabled',
    'not-configured':      'not configured',
    'no-backend':          'no upstream',
    'conditions-not-matched': 'conditions not matched',
}

const responseTierLabels: Record<string, string> = {
    configured: 'Configured',
    recorded:   'Recorded',
    fallback:   'Fallback',
}

function formatDuration(ns: number): string {
    const ms = ns / 1e6
    if (ms < 1)    return `${(ns / 1e3).toFixed(0)}µs`
    if (ms < 1000) return `${ms.toFixed(0)}ms`
    return `${(ms / 1000).toFixed(2)}s`
}

function tryFormatJson(s: string): string {
    try { return JSON.stringify(JSON.parse(s), null, 2) } catch { return s }
}

// ── Shared sub-components ────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)
    return (
        <button
            onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 1500) }}
            className="ml-1 p-1 rounded hover:bg-gray-200 dark:hover:bg-slate-700 text-gray-400 hover:text-gray-700 dark:hover:text-slate-200 transition-colors"
        >
            {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
        </button>
    )
}

function HeadersBlock({ headers }: { headers: Record<string, string[]> }) {
    const entries = Object.entries(headers)
    if (!entries.length) return null
    return (
        <div className="p-4 border-b border-gray-100 dark:border-slate-800">
            <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Headers</h4>
            <div className="font-mono text-xs space-y-1">
                {entries.map(([k, vs]) => (
                    <div key={k}>
                        <span className="text-purple-600 dark:text-purple-400">{k}:</span>{' '}
                        <span className="text-gray-600 dark:text-slate-300">{vs.join(', ')}</span>
                    </div>
                ))}
            </div>
        </div>
    )
}

function BodyBlock({ body }: { body: string }) {
    if (!body) return null
    return (
        <div className="p-4">
            <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Body</h4>
            <pre className="bg-gray-900 dark:bg-slate-950 text-gray-100 rounded p-3 text-xs overflow-x-auto whitespace-pre-wrap break-all">
                {tryFormatJson(body)}
            </pre>
        </div>
    )
}

// ── Scope badge & type badge ─────────────────────────────────────────────────

function ScopeBadge({ scope }: { scope: string }) {
    const cls =
        scope === 'spec'      ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' :
        scope === 'operation' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' :
                                'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
    return (
        <span className={`text-xs px-1.5 py-0.5 rounded font-medium shrink-0 ${cls}`}>{scope}</span>
    )
}

function TypeBadge({ type }: { type: string }) {
    if (type === 'script') return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 whitespace-nowrap shrink-0">
            <Code2 className="w-3 h-3" />SCRIPT
        </span>
    )
    if (type === 'validation') return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 whitespace-nowrap shrink-0">
            <ShieldCheck className="w-3 h-3" />VALIDATION
        </span>
    )
    return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300 whitespace-nowrap shrink-0">
            <Database className="w-3 h-3" />COLLECTION
        </span>
    )
}

// ── Individual step rows ──────────────────────────────────────────────────────

function ScriptStepRow({ st, idx, aborted }: { st: ScriptTrace; idx: number; aborted?: boolean }) {
    const [open, setOpen] = useState(false)
    const hasDetail = !!(st.error || (st.logs && st.logs.length > 0) || st.output != null)
    return (
        <div className={clsx('rounded-lg border overflow-hidden', aborted ? 'opacity-50' : 'opacity-100',
            'bg-white dark:bg-slate-900 border-gray-200 dark:border-slate-800')}>
            <button
                className="w-full px-4 py-3 flex items-center gap-3 text-left hover:bg-gray-50 dark:hover:bg-slate-800 transition-colors"
                onClick={() => hasDetail && setOpen(o => !o)}
            >
                <span className="text-xs text-gray-400 dark:text-slate-500 font-mono w-6 shrink-0 text-right">#{idx}</span>
                <TypeBadge type="script" />
                {st.scope && <ScopeBadge scope={st.scope} />}
                <span className="font-medium text-sm text-gray-900 dark:text-slate-100 truncate flex-1">{st.scriptName}</span>
                <span className="text-xs font-mono text-gray-400 dark:text-slate-500 shrink-0">→ .script.{st.outputKey}</span>
                <span className={clsx('text-xs font-semibold shrink-0', st.error ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400')}>
                    {st.error ? 'ERROR' : 'OK'}
                </span>
                <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0">{st.durationMs.toFixed(2)}ms</span>
                {hasDetail && (open
                    ? <ChevronDown className="w-3.5 h-3.5 text-gray-400 shrink-0" />
                    : <ChevronRightIcon className="w-3.5 h-3.5 text-gray-400 shrink-0" />)}
            </button>
            {open && (
                <div className="border-t border-gray-100 dark:border-slate-800">
                    {st.error && (
                        <div className="px-4 py-2 bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/40 text-xs font-mono text-red-700 dark:text-red-300">
                            {st.error}
                        </div>
                    )}
                    {st.logs && st.logs.length > 0 && (
                        <div className="border-b border-gray-100 dark:border-slate-800">
                            <div className="px-4 pt-3 pb-1 flex items-center gap-1.5 text-xs font-medium text-gray-500 dark:text-slate-400 uppercase">
                                <Terminal className="w-3.5 h-3.5" />Logs
                            </div>
                            <div className="px-4 pb-3 space-y-0.5">
                                {st.logs.map((line, j) => (
                                    <div key={j} className="flex items-start gap-2 font-mono text-xs">
                                        <span className="select-none text-gray-400 dark:text-slate-600 w-5 text-right shrink-0">{j + 1}</span>
                                        <span className="text-emerald-700 dark:text-emerald-300 break-all">{line}</span>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                    {st.output != null && (
                        <div className="px-4 py-3">
                            <div className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-1">Output</div>
                            <pre className="bg-gray-900 dark:bg-slate-950 text-gray-100 rounded p-3 text-xs overflow-x-auto">
                                {JSON.stringify(st.output, null, 2)}
                            </pre>
                        </div>
                    )}
                </div>
            )}
        </div>
    )
}

function ValidationStepRow({ vt, idx, aborted }: { vt: ValidationTrace; idx: number; aborted?: boolean }) {
    return (
        <div className={clsx('bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3 flex items-center gap-3 flex-wrap',
            aborted && 'ring-2 ring-red-400/50')}>
            <span className="text-xs text-gray-400 dark:text-slate-500 font-mono w-6 shrink-0 text-right">#{idx}</span>
            <TypeBadge type="validation" />
            <ScopeBadge scope={vt.scope} />
            <code className="text-sm font-mono font-medium text-gray-900 dark:text-slate-100 flex-1 truncate">{vt.ruleName}</code>
            {vt.properties && Object.keys(vt.properties).length > 0 && (
                <span className="text-xs text-gray-400 dark:text-slate-500 font-mono truncate">
                    {Object.entries(vt.properties).map(([k, v]) => `${k}=${v}`).join(', ')}
                </span>
            )}
            <span className={clsx('text-xs font-semibold shrink-0', vt.status === 'pass'
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-red-600 dark:text-red-400')}>
                {vt.status.toUpperCase()}
            </span>
            {aborted && (
                <span className="text-xs px-1.5 py-0.5 rounded font-semibold bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 shrink-0">
                    SCOPE ABORTED
                </span>
            )}
            <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0">{vt.durationMs}ms</span>
        </div>
    )
}

function CollectionStepRow({ ct, idx, aborted }: { ct: CollectionTrace; idx: number; aborted?: boolean }) {
    return (
        <div className={clsx('bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3 flex items-center gap-3 flex-wrap',
            aborted && 'opacity-50')}>
            <span className="text-xs text-gray-400 dark:text-slate-500 font-mono w-6 shrink-0 text-right">#{idx}</span>
            <TypeBadge type="collection" />
            {ct.scope && <ScopeBadge scope={ct.scope} />}
            <span className="text-xs px-1.5 py-0.5 rounded font-mono font-medium bg-teal-100 dark:bg-teal-900/30 text-teal-700 dark:text-teal-300 shrink-0">
                {ct.operation}
            </span>
            <code className="text-sm font-mono font-medium text-gray-900 dark:text-slate-100">{ct.collectionName}</code>
            {ct.mappingName && ct.mappingName !== ct.collectionName && (
                <span className="text-xs text-gray-400 dark:text-slate-500 truncate">({ct.mappingName})</span>
            )}
            <span className="text-xs text-gray-400 dark:text-slate-500 font-mono flex-1">→ .{ct.outputKey}</span>
            {ct.error ? (
                <span className="text-xs text-red-600 dark:text-red-400 font-medium shrink-0">{ct.error}</span>
            ) : (
                <span className="text-xs text-emerald-600 dark:text-emerald-400 font-medium shrink-0">
                    {ct.recordCount} record{ct.recordCount !== 1 ? 's' : ''}
                </span>
            )}
            <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0">{ct.durationMs}ms</span>
        </div>
    )
}

// ── Pipeline Timeline ─────────────────────────────────────────────────────────

function scopeHeaderClass(scope: string): string {
    if (scope === 'spec')      return 'bg-purple-50 dark:bg-purple-900/10 border-purple-200 dark:border-purple-800/40 text-purple-700 dark:text-purple-300'
    if (scope === 'operation') return 'bg-blue-50 dark:bg-blue-900/10 border-blue-200 dark:border-blue-800/40 text-blue-700 dark:text-blue-300'
    return 'bg-green-50 dark:bg-green-900/10 border-green-200 dark:border-green-800/40 text-green-700 dark:text-green-300'
}

function scopeIcon(scope: string) {
    if (scope === 'spec')      return <Layers className="w-3.5 h-3.5" />
    if (scope === 'operation') return <Layers className="w-3.5 h-3.5" />
    return <Layers className="w-3.5 h-3.5" />
}

function buildFallbackPipeline(trace: Trace): PipelineTraceItem[] {
    // Reconstruct order from separate arrays by scope (spec → operation → response)
    const result: PipelineTraceItem[] = []
    const scopeOrder = ['spec', 'operation', 'response']
    for (const scope of scopeOrder) {
        for (const s of trace.scripts ?? []) {
            if ((s.scope ?? 'spec') === scope) result.push({ type: 'script', script: s })
        }
        for (const v of trace.validations ?? []) {
            if (v.scope === scope) result.push({ type: 'validation', validation: v })
        }
        for (const c of trace.collections ?? []) {
            if ((c.scope ?? 'spec') === scope) result.push({ type: 'collection', collection: c })
        }
    }
    return result
}

function PipelineTimeline({ trace }: { trace: Trace }) {
    const items: PipelineTraceItem[] = trace.pipeline?.length
        ? trace.pipeline
        : buildFallbackPipeline(trace)

    if (!items.length) return null

    // Group into scope segments preserving order
    const segments: { scope: string; items: { item: PipelineTraceItem; globalIdx: number }[] }[] = []
    let currentScope = ''
    items.forEach((item, i) => {
        const scope =
            item.script?.scope ?? item.validation?.scope ?? item.collection?.scope ?? 'spec'
        if (scope !== currentScope) {
            currentScope = scope
            segments.push({ scope, items: [] })
        }
        segments[segments.length - 1].items.push({ item, globalIdx: i + 1 })
    })

    return (
        <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center gap-2">
                <Layers className="w-4 h-4 text-gray-500" />
                Pipeline Execution
                <span className="text-xs font-normal text-gray-400 dark:text-slate-500 normal-case tracking-normal">
                    {items.length} step{items.length !== 1 ? 's' : ''}
                </span>
            </h3>

            <div className="space-y-4">
                {segments.map((seg, si) => (
                    <div key={`${seg.scope}-${si}`}>
                        {/* Scope header */}
                        <div className={clsx(
                            'flex items-center gap-2 px-3 py-1.5 rounded-t-lg border text-xs font-semibold uppercase tracking-wide',
                            scopeHeaderClass(seg.scope)
                        )}>
                            {scopeIcon(seg.scope)}
                            {seg.scope} scope
                        </div>

                        {/* Steps in this scope */}
                        <div className="space-y-1.5 pt-1.5">
                            {seg.items.map(({ item, globalIdx }) => {
                                if (item.type === 'script' && item.script) {
                                    return (
                                        <ScriptStepRow
                                            key={`s-${globalIdx}`}
                                            st={item.script}
                                            idx={globalIdx}
                                            aborted={item.aborted}
                                        />
                                    )
                                }
                                if (item.type === 'validation' && item.validation) {
                                    return (
                                        <ValidationStepRow
                                            key={`v-${globalIdx}`}
                                            vt={item.validation}
                                            idx={globalIdx}
                                            aborted={item.aborted}
                                        />
                                    )
                                }
                                if (item.type === 'collection' && item.collection) {
                                    return (
                                        <CollectionStepRow
                                            key={`c-${globalIdx}`}
                                            ct={item.collection}
                                            idx={globalIdx}
                                            aborted={item.aborted}
                                        />
                                    )
                                }
                                return null
                            })}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}

// ── Session section ───────────────────────────────────────────────────────────

function SessionSection({ session }: { session: SessionTrace }) {
    return (
        <div>
            <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center gap-2">
                <Users className="w-4 h-4 text-indigo-500" />
                Session
            </h3>
            <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3 flex items-center gap-4 text-sm flex-wrap">
                <div>
                    <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase mr-1">ID</span>
                    <span className="font-mono text-xs text-gray-700 dark:text-slate-200">{session.id}</span>
                </div>
                {session.isNew && (
                    <span className="px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">New</span>
                )}
                {session.storeAccess && session.storeAccess.length > 0 && (
                    <span className="text-xs text-gray-400 dark:text-slate-500">
                        {session.storeAccess.length} store op{session.storeAccess.length !== 1 ? 's' : ''}
                    </span>
                )}
            </div>
        </div>
    )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function TraceDetailPage() {
    const { traceId } = useParams<{ traceId: string }>()
    const navigate    = useNavigate()

    const { data: trace, isLoading, error } = useQuery<Trace>({
        queryKey: ['trace', traceId],
        queryFn: () => tracesApi.get(traceId!),
        enabled: !!traceId,
    })

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-full text-gray-400 dark:text-slate-500">
                <Activity className="w-6 h-6 animate-pulse mr-2" />
                Loading trace…
            </div>
        )
    }

    if (error || !trace) {
        return (
            <div className="p-8 text-center text-gray-500 dark:text-slate-400">
                <p>Trace not found.</p>
                <button onClick={() => navigate('/traces')} className="mt-3 text-sm text-primary-600 hover:underline">
                    ← Back to traces
                </button>
            </div>
        )
    }

    const method = trace.request.method

    return (
        <div className="h-full flex flex-col overflow-hidden">
            {/* Breadcrumb / title bar */}
            <div className="bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-800 px-6 py-3 flex items-center gap-3 flex-wrap shrink-0">
                <button
                    onClick={() => navigate('/traces')}
                    className="flex items-center gap-1.5 text-sm text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-slate-100 transition-colors"
                >
                    <ArrowLeft className="w-4 h-4" />
                    Traces
                </button>
                <span className="text-gray-300 dark:text-slate-600">/</span>
                <span className={clsx(
                    'px-2 py-0.5 rounded text-xs font-bold uppercase',
                    methodColors[method] || 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                )}>
                    {method}
                </span>
                <span className="font-mono text-sm text-gray-900 dark:text-slate-100 truncate">{trace.request.path}</span>
                <span className={clsx(
                    'ml-auto px-2 py-0.5 rounded text-xs font-medium shrink-0',
                    trace.response.statusCode >= 200 && trace.response.statusCode < 300
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                        : trace.response.statusCode >= 400
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                            : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                )}>
                    {trace.response.statusCode}
                </span>
                <span className="text-xs text-gray-400 dark:text-slate-500 shrink-0 flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {new Date(trace.timestamp).toLocaleTimeString()}
                    <span className="mx-1">·</span>
                    {formatDuration(trace.duration)}
                </span>
            </div>

            {/* Content (scrollable) */}
            <div className="flex-1 overflow-y-auto bg-gray-50 dark:bg-slate-950">
                <div className="max-w-4xl mx-auto px-6 py-6 space-y-6">

                    {/* Proxy / AI banner */}
                    {(trace.responseSource === 'proxy' || trace.responseSource === 'ai') && (
                        <div className="rounded-lg border border-violet-200 dark:border-violet-800 bg-violet-50 dark:bg-violet-900/20 p-4">
                            <div className="flex items-center gap-2 mb-3">
                                {trace.responseSource === 'proxy'
                                    ? <Radio className="w-4 h-4 text-violet-600 dark:text-violet-400 shrink-0" />
                                    : <Bot className="w-4 h-4 text-fuchsia-600 dark:text-fuchsia-400 shrink-0" />}
                                <span className="text-sm font-semibold text-violet-800 dark:text-violet-200">
                                    {trace.responseSource === 'proxy' ? 'Proxy Recording' : 'AI Generation'}
                                </span>
                                <span className="ml-auto text-xs text-violet-500 dark:text-violet-400">
                                    {formatDuration(trace.duration)}
                                </span>
                            </div>
                            {trace.backendUri && (
                                <div className="flex items-start gap-2 text-sm">
                                    <Globe className="w-3.5 h-3.5 text-violet-500 mt-0.5 shrink-0" />
                                    <div className="min-w-0">
                                        <span className="text-xs text-violet-600 dark:text-violet-400 uppercase font-medium">Backend</span>
                                        <div className="flex items-center gap-1">
                                            <span className="font-mono text-xs text-violet-900 dark:text-violet-100 break-all">{trace.backendUri}</span>
                                            <CopyButton text={trace.backendUri} />
                                        </div>
                                    </div>
                                </div>
                            )}
                            {trace.signature && (
                                <div className="flex items-start gap-2 text-sm mt-2">
                                    <Fingerprint className="w-3.5 h-3.5 text-violet-500 mt-0.5 shrink-0" />
                                    <div className="min-w-0">
                                        <span className="text-xs text-violet-600 dark:text-violet-400 uppercase font-medium">Request Signature</span>
                                        <div className="flex items-center gap-1">
                                            <span className="font-mono text-xs text-violet-900 dark:text-violet-100 break-all">{trace.signature}</span>
                                            <CopyButton text={trace.signature} />
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    )}

                    {/* Fallback decision */}
                    {(trace.aiSkippedReason || trace.proxySkippedReason || trace.mode || trace.responseTier || trace.aiScenarioRequested || trace.aiScenarioApplied) && (
                        <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3">
                            <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3">
                                Fallback decision
                            </h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                                {trace.mode && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Selected mode</span>
                                        <div className="text-gray-700 dark:text-slate-200 capitalize">{trace.mode}</div>
                                    </div>
                                )}
                                {trace.responseTier && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Response tier</span>
                                        <div className="text-gray-700 dark:text-slate-200">{responseTierLabels[trace.responseTier] || trace.responseTier}</div>
                                    </div>
                                )}
                                {trace.aiSkippedReason && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">AI skip reason</span>
                                        <div className="text-gray-700 dark:text-slate-200">{skipReasonLabels[trace.aiSkippedReason] || trace.aiSkippedReason}</div>
                                    </div>
                                )}
                                {trace.proxySkippedReason && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Proxy skip reason</span>
                                        <div className="text-gray-700 dark:text-slate-200">{skipReasonLabels[trace.proxySkippedReason] || trace.proxySkippedReason}</div>
                                    </div>
                                )}
                                {trace.aiScenarioRequested && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Requested AI scenario</span>
                                        <div className="text-gray-700 dark:text-slate-200">{trace.aiScenarioRequested}</div>
                                    </div>
                                )}
                                {trace.aiScenarioApplied && (
                                    <div>
                                        <span className="text-xs font-medium text-gray-400 dark:text-slate-500 uppercase">Applied AI scenario</span>
                                        <div className="text-gray-700 dark:text-slate-200">{trace.aiScenarioApplied}</div>
                                    </div>
                                )}
                            </div>
                        </div>
                    )}

                    {/* Request */}
                    <div>
                        <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center">
                            <ArrowDownUp className="w-4 h-4 mr-2 text-blue-600" />
                            {trace.proxyMode ? 'Client Request → Backend' : 'Request'}
                        </h3>
                        <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden">
                            <div className="p-4 border-b border-gray-100 dark:border-slate-800 flex items-center gap-2 flex-wrap">
                                <span className={clsx('px-2 py-1 rounded text-sm font-bold uppercase',
                                    methodColors[method] || 'bg-gray-100 text-gray-700')}>
                                    {method}
                                </span>
                                <span className="font-mono text-sm text-gray-900 dark:text-slate-100 break-all">{trace.request.url}</span>
                            </div>
                            <HeadersBlock headers={trace.request.headers} />
                            <BodyBlock body={trace.request.body} />
                            {Object.keys(trace.request.query ?? {}).length > 0 && (
                                <div className="p-4 border-t border-gray-100 dark:border-slate-800">
                                    <h4 className="text-xs font-medium text-gray-500 dark:text-slate-400 uppercase mb-2">Query Parameters</h4>
                                    <div className="font-mono text-xs space-y-1">
                                        {Object.entries(trace.request.query).map(([k, vs]) => (
                                            <div key={k}>
                                                <span className="text-blue-600 dark:text-blue-400">{k}:</span>{' '}
                                                <span className="text-gray-600 dark:text-slate-300">{vs.join(', ')}</span>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Response */}
                    <div>
                        <h3 className="text-sm font-semibold text-gray-900 dark:text-slate-100 uppercase tracking-wider mb-3 flex items-center">
                            <ArrowDownUp className="w-4 h-4 mr-2 text-green-600 rotate-180" />
                            {trace.proxyMode ? 'Backend Response → Client' : 'Response'}
                        </h3>
                        <div className="bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 overflow-hidden">
                            <div className="p-4 border-b border-gray-100 dark:border-slate-800 flex items-center justify-between">
                                <div className="flex items-center gap-2">
                                    <span className={clsx('px-2 py-1 rounded text-sm font-bold',
                                        trace.response.statusCode >= 200 && trace.response.statusCode < 300
                                            ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
                                            : trace.response.statusCode >= 400
                                                ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                                                : 'bg-gray-100 text-gray-700 dark:bg-slate-800 dark:text-slate-300'
                                    )}>
                                        {trace.response.statusCode}
                                    </span>
                                    {(trace.responseSource === 'proxy' || trace.responseSource === 'ai') && (
                                        <span className="text-xs text-violet-600 dark:text-violet-400 font-medium">
                                            {trace.responseSource === 'proxy' ? 'Recorded & saved for replay' : 'AI-generated & saved for replay'}
                                        </span>
                                    )}
                                </div>
                                <span className="text-sm text-gray-500 dark:text-slate-400">{formatDuration(trace.duration)}</span>
                            </div>
                            <HeadersBlock headers={trace.response.headers} />
                            <BodyBlock body={trace.response.body} />
                        </div>
                    </div>

                    {/* Matched config */}
                    {trace.matchedConfig && trace.responseSource === 'config' && (
                        <div className="text-sm text-gray-500 dark:text-slate-400 bg-white dark:bg-slate-900 rounded-lg border border-gray-200 dark:border-slate-800 px-4 py-3">
                            Matched config: <span className="font-medium text-gray-700 dark:text-slate-200">{trace.matchedConfig}</span>
                            {trace.matchedConfigOrigin && (
                                <span className="ml-2 text-xs uppercase tracking-wide text-gray-400 dark:text-slate-500">
                                    ({trace.matchedConfigOrigin})
                                </span>
                            )}
                        </div>
                    )}

                    {/* Session */}
                    {trace.session && <SessionSection session={trace.session} />}

                    {/* Unified pipeline timeline */}
                    {((trace.pipeline?.length ?? 0) > 0 ||
                      (trace.scripts?.length ?? 0) > 0 ||
                      (trace.validations?.length ?? 0) > 0 ||
                      (trace.collections?.length ?? 0) > 0) && (
                        <PipelineTimeline trace={trace} />
                    )}
                </div>
            </div>
        </div>
    )
}
