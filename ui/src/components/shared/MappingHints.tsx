import { visibleHints } from './mappingHintsLogic'
import { createContext, useContext, useId, useMemo, useState, type InputHTMLAttributes, type ReactNode } from 'react'
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query'

export interface FieldHint {
    source: string; key: string; type: string; description?: string; origin: string
    required?: boolean; conditional?: boolean; values?: string[]; producerId?: string
    scope?: string; order?: number; collection?: string
}
interface Catalog { items: FieldHint[]; truncated: boolean; warnings?: string[] }
interface HintSettings {
    operationId?: string; specId?: string; scope?: string; order?: number; stepId?: string; stepType?: string
    statusCode?: number; templateRef?: string; responseId?: string; collectionName?: string; writeTarget?: boolean; traceId?: string; sessionId?: string
    extra?: FieldHint[]; collections?: { name: string; source: string; prefix?: string; array?: boolean }[]
}
interface HintContext extends HintSettings { items: FieldHint[]; loading?: boolean; error?: boolean; controls?: ReactNode }
const Context = createContext<HintContext>({ items: [] })
async function fetchHints(ctx: HintSettings, collectionName?: string): Promise<Catalog> {
    if (!ctx.operationId && !ctx.specId) return { items: [], truncated: false }
    const path = ctx.specId ? `specs/${encodeURIComponent(ctx.specId)}` : `operations/${encodeURIComponent(ctx.operationId!)}`
    const params = new URLSearchParams()
    if (ctx.specId && ctx.operationId) params.set("operationId", ctx.operationId)
    if (collectionName) params.set('collectionName', collectionName)
    if (ctx.traceId) params.set('traceId', ctx.traceId)
    if (ctx.sessionId) params.set('sessionId', ctx.sessionId)
    if (ctx.statusCode !== undefined) params.set('statusCode', String(ctx.statusCode))
    if (ctx.templateRef) params.set('templateRef', ctx.templateRef)
    if (ctx.responseId) params.set('responseId', ctx.responseId)
    const response = await fetch(`/_api/${path}/mapping-hints?${params}`)
    if (!response.ok) throw new Error('Suggestions unavailable; custom input still works.')
    return response.json()
}
export function MappingHintsProvider({ children, controls = false, ...settings }: HintSettings & { children: ReactNode; controls?: boolean }) {
    const parent = useContext(Context)
    const [traceId, setTraceId] = useState('')
    const [sessionId, setSessionId] = useState('')
    const [operationId, setOperationId] = useState('')
    const queryClient = useQueryClient()
    const ctx = { ...parent, ...Object.fromEntries(Object.entries(settings).filter(([, value]) => value !== undefined)), ...(traceId ? { traceId } : {}), ...(sessionId ? { sessionId } : {}), ...(operationId ? { operationId } : {}) }
    const operations = useQuery<{ id: string; method: string; path: string }[]>({
        queryKey: ['hintOperations', ctx.specId], enabled: !!ctx.specId,
        queryFn: async () => { const response = await fetch(`/_api/specs/${encodeURIComponent(ctx.specId!)}/operations`); if (!response.ok) throw new Error('Operations unavailable'); return response.json() },
    })
    const enabled = !!(ctx.operationId || ctx.specId)
    const query = useQuery({ queryKey: ['mappingHints', ctx.operationId, ctx.specId, ctx.collectionName, ctx.traceId, ctx.sessionId, ctx.statusCode, ctx.templateRef, ctx.responseId], queryFn: () => fetchHints(ctx, ctx.collectionName), enabled, staleTime: 0 })
    const collectionQueries = useQueries({ queries: (ctx.collections || []).map(c => ({
        queryKey: ['mappingHints', ctx.operationId, ctx.specId, c.name, ctx.traceId, ctx.sessionId, ctx.statusCode, ctx.templateRef, ctx.responseId],
        queryFn: () => fetchHints(ctx, c.name), enabled: enabled && !!c.name, staleTime: 0,
    })) })
    const items = [...(query.data?.items || []).map(h => h.source === 'target' ? { ...h, key: h.key.replace(/(^|\.)0(?=\.|$)/g, '').replace(/^\./, '') } : h), ...(ctx.extra || [])]
    collectionQueries.forEach((q, i) => {
        const c = ctx.collections![i]
        if (c.prefix) items.push({ source: c.source, key: c.prefix, type: c.array ? 'array' : 'object', origin: 'configured' })
        for (const hint of q.data?.items || []) if (hint.source === 'document') {
            items.push({ ...hint, source: c.source, key: [c.prefix, c.array ? '0' : '', hint.key].filter(Boolean).join('.') })
        }
    })
    const availableItems = items.filter(h => h.source !== 'mapper' || !ctx.collections || ctx.collections.some(c => c.source === 'mapper' && (h.key === c.prefix || h.key.startsWith(c.prefix + '.'))))
    const value = { ...ctx, items: availableItems, loading: query.isFetching, error: query.isError }
    const contextControls = <details className="mb-3 text-xs text-gray-500 dark:text-slate-400">
            <summary className="cursor-pointer">Field suggestions · schema and previous executions</summary>
            <div className="flex flex-wrap gap-2 mt-2">
                {ctx.specId && <label>Operation context <select aria-label="Suggestion operation" value={operationId} onChange={e => setOperationId(e.target.value)} className="block rounded border p-1 bg-white dark:bg-slate-950 dark:border-slate-700"><option value="">Common fields across operations</option>{operations.data?.map(op => <option key={op.id} value={op.id}>{op.method} {op.path}</option>)}</select></label>}
                <label>Trace context <input aria-label="Suggestion trace ID" placeholder="Trace ID (blank: recent traces)" value={traceId} onChange={e => setTraceId(e.target.value)} className="block rounded border p-1 bg-white dark:bg-slate-950 dark:border-slate-700" /></label>
                <label>Session context <input aria-label="Suggestion session ID" placeholder="Session ID (optional)" value={sessionId} onChange={e => setSessionId(e.target.value)} className="block rounded border p-1 bg-white dark:bg-slate-950 dark:border-slate-700" /></label>
                <button type="button" onClick={() => queryClient.invalidateQueries({ queryKey: ['mappingHints'] })} className="underline">Refresh suggestions</button>
            </div>
            {query.isError && <p role="status">Suggestions unavailable. You can still enter fields manually.</p>}
            {query.data?.truncated && <p>Suggestions are sampled and limited; custom fields remain available.</p>}
            {query.data?.warnings?.map(w => <p key={w}>{w}</p>)}
        </details>
    return <Context.Provider value={{ ...value, controls: contextControls }}>
        {controls && contextControls}
        {children}
    </Context.Provider>
}

export function shapeHints(value: unknown, source: string, prefix = '', depth = 0): FieldHint[] {
    if (depth > 8) return []
    const type = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value
    const result: FieldHint[] = prefix ? [{ source, key: prefix, type, origin: 'example' }] : []
    if (Array.isArray(value)) {
        if (value.length) result.push(...shapeHints(value[0], source, prefix, depth + 1).filter(h => h.key !== prefix))
    } else if (value && typeof value === 'object') {
        for (const [key, child] of Object.entries(value)) if (!/[.\\#*?\[\]|!@:]/.test(key)) result.push(...shapeHints(child, source, [prefix, key].filter(Boolean).join('.'), depth + 1))
    }
    return result.slice(0, 500)
}

interface SuggestedProps extends InputHTMLAttributes<HTMLInputElement> {
    hintSource: string; valueKey?: string; jsonValue?: boolean; suggestions?: { value: string; description?: string }[]; topLevel?: boolean
}
export function SuggestedFieldInput({ hintSource, valueKey, jsonValue = false, suggestions = [], topLevel = false, className, ...props }: SuggestedProps) {
    const ctx = useContext(Context)
    const id = useId()
    const candidates = useMemo(() => {
        const seen = new Map<string, { value: string; description: string }>()
        for (const h of visibleHints(ctx.items, ctx)) {
            if (h.source !== hintSource || topLevel && (h.key.includes('.') || h.key === '_id')) continue
            if (valueKey !== undefined && h.key !== valueKey) continue
            const values = valueKey === undefined ? [h.key] : (h.values || []).map(v => {
                if (jsonValue) return v
                try { const parsed = JSON.parse(v); return typeof parsed === 'string' ? parsed : String(parsed) } catch { return v }
            })
            for (const v of values) if (!seen.has(v)) seen.set(v, { value: v, description: `${h.type} · ${h.origin}${h.required ? ' · required' : ''}${h.conditional ? ' · may be absent' : ''}${h.description ? ` · ${h.description}` : ''}` })
        }
        for (const s of suggestions) if (!seen.has(s.value)) seen.set(s.value, { value: s.value, description: s.description || '' })
        const search = String(props.value || '').toLowerCase()
        return [...seen.values()].filter(h => h.value.toLowerCase().includes(search)).sort((a, b) => Number(b.value.toLowerCase().startsWith(search)) - Number(a.value.toLowerCase().startsWith(search)) || a.value.localeCompare(b.value)).slice(0, 50)
    }, [ctx, hintSource, valueKey, jsonValue, suggestions, props.value, topLevel])
    const selected = candidates.find(c => c.value === String(props.value ?? ''))
    return <span className="inline-flex flex-col min-w-0 flex-1">
        <input {...props} autoComplete="off" className={className} list={props.disabled ? undefined : id} aria-describedby={`${id}-help`} />
        <datalist id={id}>{candidates.map(c => <option key={c.value} value={c.value}>{c.description}</option>)}</datalist>
        <span id={`${id}-help`} className="text-[10px] text-gray-500 dark:text-slate-400 truncate" title={selected?.description}>{selected?.description || (ctx.error ? 'Suggestions unavailable; custom input allowed' : '')}</span>
    </span>
}

export function MappingHintControls() { return <>{useContext(Context).controls}</> }
