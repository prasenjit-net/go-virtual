import type { TemplateSourceOption } from './templateSources'

export type LeafValueType = 'string' | 'number' | 'boolean' | 'null'

export function normalizeTemplateExpression(value: string): string {
    const trimmed = value.trim()
    if (!trimmed) return ''
    if (trimmed.startsWith('{{') && trimmed.endsWith('}}')) {
        return trimmed
    }
    return `{{${trimmed}}}`
}

export function snippetForSource(option: TemplateSourceOption): string {
    return option.snippet
}

export function renderTemplateLeafValue(expression: string, valueType: LeafValueType): string {
    const normalized = normalizeTemplateExpression(expression)
    if (!normalized) {
        return JSON.stringify('')
    }
    if (valueType === 'string') {
        return JSON.stringify(normalized)
    }
    return normalized
}

