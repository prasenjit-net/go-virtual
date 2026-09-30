import type { FieldHint } from './MappingHints'

export interface HintPosition { scope?: string; order?: number; stepId?: string; stepType?: string }
const scopeRank: Record<string, number> = { spec: 0, operation: 1, response: 2 }
const typeRank: Record<string, number> = { script: 0, validation: 1, collection: 2 }
export function hintAvailable(h: FieldHint, ctx: HintPosition): boolean {
    if (!h.producerId) return true
    if (h.producerId === ctx.stepId) return false
    const before = scopeRank[h.scope || 'spec'] ?? 0
    const current = scopeRank[ctx.scope || 'response'] ?? 2
    if (before !== current) return before < current
    if (ctx.order === undefined) return current !== 2
    return (h.order ?? 0) < ctx.order || h.order === ctx.order && (typeRank[h.source] ?? 2) < (typeRank[ctx.stepType || 'validation'] ?? 1)
}
export function visibleHints(items: FieldHint[], ctx: HintPosition): FieldHint[] {
    const sorted = items.filter(h => hintAvailable(h, ctx)).sort((a, b) =>
        (scopeRank[b.scope || 'spec'] ?? 0) - (scopeRank[a.scope || 'spec'] ?? 0)
        || (b.order ?? 0) - (a.order ?? 0)
        || Number(b.origin === 'schema') - Number(a.origin === 'schema'))
    const producers = new Map<string, string>()
    return sorted.filter(h => {
        if (!h.producerId) return true
        const namespace = `${h.source}:${h.key.split('.')[0]}`
        const winner = producers.get(namespace)
        if (winner && winner !== h.producerId) return false
        producers.set(namespace, h.producerId)
        return true
    })
}
