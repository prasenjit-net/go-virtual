import { renderTemplateLeafValue, type LeafValueType } from './templateSnippets'

export type BranchBinding =
    | { kind: 'collection'; sourceId?: string; outputKey: string; operation?: string }
    | { kind: 'item'; itemPath: string; operation: 'find-many' }

export type LeafBinding =
    | { kind: 'literal'; value: string | number | boolean | null }
    | { kind: 'template'; expression: string; sourceId?: string; nestedPath?: string }
    | { kind: 'unsupported'; raw: string }

export type VisualTemplateNode =
    | { kind: 'object'; path: string; key?: string; children: VisualTemplateNode[]; binding?: BranchBinding }
    | { kind: 'array'; path: string; key?: string; children: VisualTemplateNode[]; binding?: BranchBinding }
    | { kind: 'leaf'; path: string; key?: string; valueType: LeafValueType; binding: LeafBinding }

export interface TemplateTreeParseResult {
    ok: boolean
    root?: VisualTemplateNode
    error?: string
}

const templatePattern = /^\s*\{\{[\s\S]*\}\}\s*$/
const templatePlaceholderPrefix = '__GO_VIRTUAL_TEMPLATE_'
const rangePlaceholderPrefix = '__GO_VIRTUAL_RANGE_'

interface MaskedTemplateBody {
    body: string
    templates: Map<string, string>
    ranges: Map<string, BranchBinding>
}

interface MaskState {
    templateIndex: number
    rangeIndex: number
    templates: Map<string, string>
    ranges: Map<string, BranchBinding>
}

function childPath(parent: string, key: string | number): string {
    return parent === '$' ? `$.${key}` : `${parent}.${key}`
}

function valueType(value: unknown): LeafValueType {
    if (value === null) return 'null'
    if (typeof value === 'number') return 'number'
    if (typeof value === 'boolean') return 'boolean'
    return 'string'
}

function isTemplatePlaceholder(value: string): boolean {
    return value.startsWith(templatePlaceholderPrefix) && value.endsWith('__')
}

function isRangePlaceholder(value: string): boolean {
    return value.startsWith(rangePlaceholderPrefix) && value.endsWith('__')
}

function leafBinding(value: string | number | boolean | null, templates: Map<string, string>): LeafBinding {
    if (typeof value === 'string' && isTemplatePlaceholder(value)) {
        return { kind: 'template', expression: templates.get(value) || '{{.Path.id}}' }
    }
    if (typeof value === 'string' && templatePattern.test(value)) {
        return { kind: 'template', expression: value.trim() }
    }
    return { kind: 'literal', value }
}

function maskRawTemplateActionsWithState(body: string, state: MaskState): string {
    let masked = ''
    let inString = false
    let escaped = false

    for (let i = 0; i < body.length; i += 1) {
        const char = body[i]
        const next = body[i + 1]

        if (inString) {
            masked += char
            if (escaped) {
                escaped = false
            } else if (char === '\\') {
                escaped = true
            } else if (char === '"') {
                inString = false
            }
            continue
        }

        if (char === '"') {
            inString = true
            masked += char
            continue
        }

        if (char === '{' && next === '{') {
            const end = body.indexOf('}}', i + 2)
            if (end === -1) {
                masked += char
                continue
            }
            const expression = body.slice(i, end + 2)
            const placeholder = `${templatePlaceholderPrefix}${state.templateIndex}__`
            state.templates.set(placeholder, expression)
            masked += JSON.stringify(placeholder)
            state.templateIndex += 1
            i = end + 1
            continue
        }

        masked += char
    }

    return masked
}

function newMaskState(): MaskState {
    return {
        templateIndex: 0,
        rangeIndex: 0,
        templates: new Map(),
        ranges: new Map(),
    }
}

function maskRawTemplateActions(body: string): MaskedTemplateBody {
    const state = newMaskState()
    const masked = maskRawTemplateActionsWithState(body, state)
    return { body: masked, templates: state.templates, ranges: state.ranges }
}

function readTemplateAction(body: string, start: number): { action: string; end: number } | null {
    if (body[start] !== '{' || body[start + 1] !== '{') return null
    const end = body.indexOf('}}', start + 2)
    if (end === -1) return null
    return { action: body.slice(start, end + 2), end: end + 2 }
}

function actionInner(action: string): string {
    return action.slice(2, -2).trim()
}

function parseIndexPathArgs(args: string): string {
    const parts: string[] = []
    const pattern = /`([^`]*)`|"([^"]*)"|(\d+)/g
    let match: RegExpExecArray | null
    while ((match = pattern.exec(args)) !== null) {
        parts.push(match[1] ?? match[2] ?? match[3])
    }
    return parts.join('.')
}

function parseRangeStart(action: string): BranchBinding | null {
    const inner = actionInner(action)
    const collectionMatch = inner.match(/^range\s+\$i\s*,\s*\$item\s*:=\s*\.Collection\.([A-Za-z_][\w]*)$/)
    if (collectionMatch) {
        return { kind: 'collection', outputKey: collectionMatch[1], operation: 'find-many' }
    }
    const itemMatch = inner.match(/^range\s+\$i\s*,\s*\$item\s*:=\s*index\s+\$item\s+(.+)$/)
    if (itemMatch) {
        return { kind: 'item', itemPath: parseIndexPathArgs(itemMatch[1]), operation: 'find-many' }
    }
    return null
}

function isControlStart(action: string): boolean {
    return /^(range|if|with)\b/.test(actionInner(action))
}

function isControlEnd(action: string): boolean {
    return actionInner(action) === 'end'
}

function skipWhitespace(body: string, index: number): number {
    let i = index
    while (i < body.length && /\s/.test(body[i])) i += 1
    return i
}

function findMatchingRangeEnd(body: string, index: number): { start: number; end: number } | null {
    let depth = 1
    let cursor = index

    while (cursor < body.length) {
        const start = body.indexOf('{{', cursor)
        if (start === -1) return null
        const action = readTemplateAction(body, start)
        if (!action) return null

        if (isControlStart(action.action)) {
            depth += 1
        } else if (isControlEnd(action.action)) {
            depth -= 1
            if (depth === 0) {
                return { start, end: action.end }
            }
        }
        cursor = action.end
    }

    return null
}

function stripRangeCommaGuard(body: string): string {
    return body.replace(/^\s*\{\{if\s+\$i\s*\}\}\s*,\s*\{\{end\s*\}\}/, '')
}

function maskGeneratedRangeArraysWithState(body: string, state: MaskState): string {
    let out = ''
    let cursor = 0

    while (cursor < body.length) {
        const bracket = body.indexOf('[', cursor)
        if (bracket === -1) {
            out += body.slice(cursor)
            break
        }

        out += body.slice(cursor, bracket)
        const rangeActionStart = skipWhitespace(body, bracket + 1)
        const rangeAction = readTemplateAction(body, rangeActionStart)
        const rangeStart = rangeAction ? parseRangeStart(rangeAction.action) : null

        if (!rangeAction || !rangeStart) {
            out += body[bracket]
            cursor = bracket + 1
            continue
        }

        const rangeEnd = findMatchingRangeEnd(body, rangeAction.end)
        if (!rangeEnd) {
            out += body[bracket]
            cursor = bracket + 1
            continue
        }

        const closeBracket = skipWhitespace(body, rangeEnd.end)
        if (body[closeBracket] !== ']') {
            out += body[bracket]
            cursor = bracket + 1
            continue
        }

        const arrayPlaceholder = `${rangePlaceholderPrefix}${state.rangeIndex}__`
        state.ranges.set(arrayPlaceholder, rangeStart)
        state.rangeIndex += 1

        const itemTemplate = stripRangeCommaGuard(body.slice(rangeAction.end, rangeEnd.start))
        const nestedRanges = maskGeneratedRangeArraysWithState(itemTemplate, state)
        const maskedItem = maskRawTemplateActionsWithState(nestedRanges, state)

        out += `[${JSON.stringify(arrayPlaceholder)},${maskedItem}]`
        cursor = closeBracket + 1
    }

    return out
}

function maskGeneratedRangeArrays(body: string): MaskedTemplateBody {
    const state = newMaskState()
    const masked = maskGeneratedRangeArraysWithState(body, state)

    return { body: masked, templates: state.templates, ranges: state.ranges }
}

function nodeFromValue(value: unknown, path: string, key: string | undefined, templates: Map<string, string>, ranges: Map<string, BranchBinding>): VisualTemplateNode {
    if (Array.isArray(value)) {
        const maybePlaceholder = value[0]
        const rangeBinding = typeof maybePlaceholder === 'string' && isRangePlaceholder(maybePlaceholder) ? ranges.get(maybePlaceholder) : undefined
        const values = rangeBinding ? value.slice(1) : value
        return {
            kind: 'array',
            path,
            key,
            binding: rangeBinding,
            children: values.map((item, index) => {
                return nodeFromValue(item, childPath(path, index), String(index), templates, ranges)
            }),
        }
    }
    if (value && typeof value === 'object') {
        return {
            kind: 'object',
            path,
            key,
            children: Object.entries(value as Record<string, unknown>).map(([childKey, child]) =>
                nodeFromValue(child, childPath(path, childKey), childKey, templates, ranges),
            ),
        }
    }
    const primitive = value as string | number | boolean | null
    const fromRawTemplate = typeof primitive === 'string' && isTemplatePlaceholder(primitive)
    return {
        kind: 'leaf',
        path,
        key,
        valueType: fromRawTemplate ? 'number' : valueType(primitive),
        binding: leafBinding(primitive, templates),
    }
}

export function parseTemplateTree(body: string): TemplateTreeParseResult {
    const trimmed = body.trim()
    if (!trimmed) {
        return { ok: true, root: { kind: 'object', path: '$', children: [] } }
    }
    try {
        return { ok: true, root: nodeFromValue(JSON.parse(trimmed), '$', undefined, new Map(), new Map()) }
    } catch {
        try {
            const masked = maskRawTemplateActions(trimmed)
            return { ok: true, root: nodeFromValue(JSON.parse(masked.body), '$', undefined, masked.templates, masked.ranges) }
        } catch {
            try {
                const masked = maskGeneratedRangeArrays(trimmed)
                return { ok: true, root: nodeFromValue(JSON.parse(masked.body), '$', undefined, masked.templates, masked.ranges) }
            } catch (error) {
                return {
                    ok: false,
                    error: error instanceof Error ? error.message : 'Response body is not valid JSON.',
                }
            }
        }
    }
}

function indexExpression(root: string, nestedPath: string | undefined): string {
    const segments = (nestedPath || '').split('.').filter(Boolean)
    if (segments.length === 0) return root
    const args = segments.map((segment) => (/^\d+$/.test(segment) ? segment : `\`${segment.replace(/`/g, '')}\``)).join(' ')
    return `index ${root} ${args}`
}

function expressionForRangeItem(expression: string, nestedPath: string | undefined): string {
    if (expression.includes('$item')) return expression
    const indexCollectionMatch = expression.match(/^\{\{index\s+\.Collection\.[A-Za-z_][\w]*(\s+[\s\S]+)\}\}$/)
    if (indexCollectionMatch) {
        return `{{index $item${indexCollectionMatch[1]}}}`
    }
    if (expression.startsWith('{{.Collection.')) {
        return `{{${indexExpression('$item', nestedPath)}}}`
    }
    return expression
}

function compileNode(node: VisualTemplateNode, depth: number, rangeItem = false): string {
    const pad = '  '.repeat(depth)
    const childPad = '  '.repeat(depth + 1)

    if (node.kind === 'object') {
        if (node.children.length === 0) return '{}'
        const lines = node.children.map((child) => `${childPad}${JSON.stringify(child.key || '')}: ${compileNode(child, depth + 1, rangeItem)}`)
        return `{\n${lines.join(',\n')}\n${pad}}`
    }

    if (node.kind === 'array') {
        if (node.children.length === 0) return '[]'
        if (node.binding?.kind === 'collection' && node.binding.operation === 'find-many') {
            const item = node.children[0]
            if (!item) return '[]'
            return `[\n${childPad}{{range $i, $item := .Collection.${node.binding.outputKey}}}{{if $i}},{{end}}\n${childPad}${compileNode(item, depth + 1, true)}\n${childPad}{{end}}\n${pad}]`
        }
        if (node.binding?.kind === 'item' && node.binding.operation === 'find-many' && rangeItem) {
            const item = node.children[0]
            if (!item) return '[]'
            return `[\n${childPad}{{range $i, $item := ${indexExpression('$item', node.binding.itemPath)}}}{{if $i}},{{end}}\n${childPad}${compileNode(item, depth + 1, true)}\n${childPad}{{end}}\n${pad}]`
        }
        const lines = node.children.map((child) => `${childPad}${compileNode(child, depth + 1, rangeItem)}`)
        return `[\n${lines.join(',\n')}\n${pad}]`
    }

    if (node.binding.kind === 'template') {
        return renderTemplateLeafValue(rangeItem ? expressionForRangeItem(node.binding.expression, node.binding.nestedPath) : node.binding.expression, node.valueType)
    }
    if (node.binding.kind === 'unsupported') {
        return JSON.stringify(node.binding.raw)
    }
    return JSON.stringify(node.binding.value)
}

export function compileTemplateTree(root: VisualTemplateNode): string {
    return `${compileNode(root, 0)}\n`
}

export function updateLeafBinding(
    node: VisualTemplateNode,
    path: string,
    binding: LeafBinding,
    valueType?: LeafValueType,
): VisualTemplateNode {
    if (node.path === path && node.kind === 'leaf') {
        return { ...node, binding, valueType: valueType ?? node.valueType }
    }
    if (node.kind === 'leaf') return node
    return {
        ...node,
        children: node.children.map((child) => updateLeafBinding(child, path, binding, valueType)),
    }
}

export function updateLeafType(node: VisualTemplateNode, path: string, valueType: LeafValueType): VisualTemplateNode {
    if (node.path === path && node.kind === 'leaf') {
        const binding = node.binding.kind === 'literal' ? coerceLiteralBinding(node.binding, valueType) : node.binding
        return { ...node, valueType, binding }
    }
    if (node.kind === 'leaf') return node
    return { ...node, children: node.children.map((child) => updateLeafType(child, path, valueType)) }
}

export function updateNode(node: VisualTemplateNode, path: string, updater: (target: VisualTemplateNode) => VisualTemplateNode): VisualTemplateNode {
    if (node.path === path) {
        return updater(node)
    }
    if (node.kind === 'leaf') return node
    return { ...node, children: node.children.map((child) => updateNode(child, path, updater)) }
}

function coerceLiteralBinding(binding: LeafBinding, valueType: LeafValueType): LeafBinding {
    if (binding.kind !== 'literal') return binding
    const current = binding.value
    if (valueType === 'string') return { kind: 'literal', value: current == null ? '' : String(current) }
    if (valueType === 'number') return { kind: 'literal', value: Number(current) || 0 }
    if (valueType === 'boolean') return { kind: 'literal', value: current === true || current === 'true' }
    return { kind: 'literal', value: null }
}

export function createRoot(kind: 'object' | 'array'): VisualTemplateNode {
    return kind === 'object' ? { kind: 'object', path: '$', children: [] } : { kind: 'array', path: '$', children: [] }
}

export function describeBinding(binding: LeafBinding): string {
    if (binding.kind === 'template') return binding.expression
    if (binding.kind === 'unsupported') return binding.raw
    return JSON.stringify(binding.value)
}
