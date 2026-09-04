import { useMemo, useState } from 'react'
import { AlertCircle, Braces, ChevronDown, ChevronRight, Code2, FileJson, ListPlus, Plus, Type } from 'lucide-react'
import clsx from 'clsx'
import type { LeafValueType } from './templateSnippets'
import { normalizeTemplateExpression, snippetForSource } from './templateSnippets'
import {
    compileTemplateTree,
    createRoot,
    describeBinding,
    parseTemplateTree,
    updateLeafBinding,
    updateLeafType,
    updateNode,
    type LeafBinding,
    type VisualTemplateNode,
} from './templateTree'
import { sourceGroupLabels, type TemplateSourceOption } from './templateSources'

interface VisualTemplateEditorProps {
    body: string
    onBodyChange: (body: string) => void
    sources: TemplateSourceOption[]
    readOnly?: boolean
    heightClass?: string
}

interface LeafInfo {
    kind: 'leaf'
    path: string
    key?: string
    valueType: LeafValueType
    binding: LeafBinding
}

interface BranchInfo {
    kind: 'object' | 'array'
    path: string
    key?: string
    childCount: number
}

type SelectedInfo = LeafInfo | BranchInfo

function collectLeaves(node: VisualTemplateNode): LeafInfo[] {
    if (node.kind === 'leaf') {
        return [{ kind: 'leaf', path: node.path, key: node.key, valueType: node.valueType, binding: node.binding }]
    }
    return node.children.flatMap(collectLeaves)
}

function collectSelectable(node: VisualTemplateNode): SelectedInfo[] {
    if (node.kind === 'leaf') {
        return [{ kind: 'leaf', path: node.path, key: node.key, valueType: node.valueType, binding: node.binding }]
    }
    return [
        { kind: node.kind, path: node.path, key: node.key, childCount: node.children.length },
        ...node.children.flatMap(collectSelectable),
    ]
}

function typeLabel(node: VisualTemplateNode): string {
    if (node.kind === 'object') return `${node.children.length} ${node.children.length === 1 ? 'field' : 'fields'}`
    if (node.kind === 'array') return `${node.children.length} ${node.children.length === 1 ? 'item' : 'items'}`
    return node.valueType
}

function pathSegments(fromPath: string, toPath: string): string[] {
    const prefix = fromPath === '$' ? '$.' : `${fromPath}.`
    return toPath.startsWith(prefix) ? toPath.slice(prefix.length).split('.').filter(Boolean) : []
}

function collectionExpression(source: TemplateSourceOption, relativePath: string[], arrayIndex?: number): string {
    const outputKey = source.outputKey || source.label
    if (typeof arrayIndex === 'number') {
        const args = [String(arrayIndex), ...relativePath.map((segment) => (/^\d+$/.test(segment) ? segment : `\`${segment.replace(/`/g, '')}\``))]
        return `{{index .Collection.${outputKey} ${args.join(' ')}}}`
    }
    if (relativePath.length === 0) {
        return `{{.Collection.${outputKey}}}`
    }
    const args = relativePath.map((segment) => (/^\d+$/.test(segment) ? segment : `\`${segment.replace(/`/g, '')}\``)).join(' ')
    return `{{index .Collection.${outputKey} ${args}}}`
}

function rangeItemExpression(relativePath: string[]): string {
    if (relativePath.length === 0) return '{{$item}}'
    const args = relativePath.map((segment) => (/^\d+$/.test(segment) ? segment : `\`${segment.replace(/`/g, '')}\``)).join(' ')
    return `{{index $item ${args}}}`
}

function bindLeavesToCollection(node: VisualTemplateNode, source: TemplateSourceOption, basePath: string, arrayItemPath?: string, arrayIndex?: number): VisualTemplateNode {
    if (node.kind === 'leaf') {
        const relativePath = pathSegments(arrayItemPath || basePath, node.path)
        return {
            ...node,
            binding: {
                kind: 'template',
                sourceId: source.id,
                nestedPath: relativePath.join('.'),
                expression: collectionExpression(source, relativePath, arrayIndex),
            },
        }
    }
    return {
        ...node,
        children: node.children.map((child) => bindLeavesToCollection(child, source, basePath, arrayItemPath, arrayIndex)),
    }
}

function bindLeavesToRangeItem(node: VisualTemplateNode, source: TemplateSourceOption, itemPath: string): VisualTemplateNode {
    if (node.kind === 'leaf') {
        const relativePath = pathSegments(itemPath, node.path)
        return {
            ...node,
            binding: {
                kind: 'template',
                sourceId: source.id,
                nestedPath: relativePath.join('.'),
                expression: rangeItemExpression(relativePath),
            },
        }
    }
    if (node.kind === 'array') {
        const relativePath = pathSegments(itemPath, node.path)
        const templateChild = node.children[0]
        return {
            ...node,
            binding: {
                kind: 'item',
                itemPath: relativePath.join('.'),
                operation: 'find-many',
            },
            children: templateChild ? [bindLeavesToRangeItem(templateChild, source, templateChild.path)] : [],
        }
    }
    return {
        ...node,
        children: node.children.map((child) => bindLeavesToRangeItem(child, source, itemPath)),
    }
}

function bindNodeToCollection(node: VisualTemplateNode, source: TemplateSourceOption): VisualTemplateNode {
    if (node.kind === 'array' && source.collectionOperation === 'find-many') {
        const templateChild = node.children[0]
        return {
            ...node,
            binding: {
                kind: 'collection',
                sourceId: source.id,
                outputKey: source.outputKey || source.label,
                operation: source.collectionOperation,
            },
            children: templateChild ? [bindLeavesToRangeItem(templateChild, source, templateChild.path)] : [],
        }
    }
    const mapped = bindLeavesToCollection(node, source, node.path)
    if (mapped.kind !== 'leaf') {
        return {
            ...mapped,
            binding: {
                kind: 'collection',
                sourceId: source.id,
                outputKey: source.outputKey || source.label,
                operation: source.collectionOperation,
            },
        }
    }
    return mapped
}

function applyNestedPath(snippet: string, nestedPath: string): string {
    const path = nestedPath.trim().replace(/^\.+/, '')
    if (!path) return snippet
    if (snippet.includes('"field.path"')) return snippet.replace('"field.path"', JSON.stringify(path))
    if (snippet.includes('"key"')) return snippet.replace('"key"', JSON.stringify(path))
    if (snippet.includes('.property')) return snippet.replace('.property', `.${path}`)
    if (snippet.includes('.Collection.') && snippet.endsWith('}}')) return `${snippet.slice(0, -2)}.${path}}}`
    if (snippet.endsWith('}}')) return `${snippet.slice(0, -2)}.${path}}}`
    return snippet
}

function parseLiteral(value: string, valueType: LeafValueType): string | number | boolean | null {
    if (valueType === 'string') return value
    if (valueType === 'number') return Number(value) || 0
    if (valueType === 'boolean') return value === 'true'
    return null
}

function replaceRoot(onBodyChange: (body: string) => void, kind: 'object' | 'array') {
    onBodyChange(compileTemplateTree(createRoot(kind)))
}

export default function VisualTemplateEditor({
    body,
    onBodyChange,
    sources,
    readOnly = false,
    heightClass = 'h-[360px]',
}: VisualTemplateEditorProps) {
    const parsed = useMemo(() => parseTemplateTree(body), [body])
    const [expanded, setExpanded] = useState<Set<string>>(() => new Set(['$']))
    const leaves = useMemo(() => (parsed.root ? collectLeaves(parsed.root) : []), [parsed.root])
    const selectable = useMemo(() => (parsed.root ? collectSelectable(parsed.root) : []), [parsed.root])
    const [selectedPath, setSelectedPath] = useState<string>('')
    const selected = selectable.find((item) => item.path === selectedPath) ?? selectable[0]
    const collectionSources = sources.filter((source) => source.group === 'collection' && !source.disabled && source.outputKey)

    const updateRoot = (root: VisualTemplateNode) => onBodyChange(compileTemplateTree(root))

    const groupedSources = useMemo(() => {
        return sources.reduce<Record<string, TemplateSourceOption[]>>((acc, source) => {
            acc[source.group] = acc[source.group] || []
            acc[source.group].push(source)
            return acc
        }, {})
    }, [sources])

    const renderTemplateControls = (binding: Extract<LeafBinding, { kind: 'template' }>, leaf: LeafInfo) => (
        <div className="space-y-3">
            <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                Source
                <select
                    disabled={readOnly}
                    value={binding.sourceId || ''}
                    onChange={(event) => {
                        const source = sources.find((item) => item.id === event.target.value)
                        if (!source || source.disabled) return
                        updateRoot(updateLeafBinding(parsed.root!, leaf.path, {
                            kind: 'template',
                            sourceId: source.id,
                            expression: snippetForSource(source),
                        }))
                    }}
                    className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                >
                    <option value="">Custom expression</option>
                    {Object.entries(groupedSources).map(([group, options]) => (
                        <optgroup key={group} label={sourceGroupLabels[group as keyof typeof sourceGroupLabels] || group}>
                            {options.map((source) => (
                                <option key={source.id} value={source.id} disabled={source.disabled}>
                                    {source.label}
                                </option>
                            ))}
                        </optgroup>
                    ))}
                </select>
            </label>
            <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                Expression
                <input
                    disabled={readOnly}
                    value={binding.expression}
                    onChange={(event) => updateRoot(updateLeafBinding(parsed.root!, leaf.path, { ...binding, expression: normalizeTemplateExpression(event.target.value) }))}
                    className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 font-mono text-xs dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                />
            </label>
            <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                Nested path
                <div className="mt-1 flex gap-2">
                    <input
                        disabled={readOnly}
                        placeholder="field.path"
                        value={binding.nestedPath || ''}
                        onChange={(event) => {
                            const source = sources.find((item) => item.id === binding.sourceId)
                            const base = source ? snippetForSource(source) : binding.expression
                            updateRoot(updateLeafBinding(parsed.root!, leaf.path, {
                                ...binding,
                                nestedPath: event.target.value,
                                expression: applyNestedPath(base, event.target.value),
                            }))
                        }}
                        className="min-w-0 flex-1 rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                    />
                    <button
                        type="button"
                        disabled={readOnly}
                        onClick={() => updateRoot(updateLeafBinding(parsed.root!, leaf.path, binding))}
                        className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-gray-300 text-gray-600 hover:bg-gray-100 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                        title="Apply nested path"
                    >
                        <Plus className="h-3.5 w-3.5" />
                    </button>
                </div>
            </label>
        </div>
    )

    const renderNode = (node: VisualTemplateNode, depth = 0) => {
        const canExpand = node.kind !== 'leaf'
        const isExpanded = expanded.has(node.path)
        const isSelected = selected?.path === node.path
        const name = node.key ?? 'root'

        return (
            <div key={node.path}>
                <button
                    type="button"
                    className={clsx(
                        'flex h-9 w-full items-center gap-2 border-b border-gray-100 px-2 text-left text-xs dark:border-slate-800',
                        node.kind === 'leaf' ? 'hover:bg-gray-50 dark:hover:bg-slate-900' : 'bg-gray-50/70 dark:bg-slate-900/50',
                        isSelected && 'bg-primary-50 text-primary-700 dark:bg-primary-950/30 dark:text-primary-300',
                    )}
                    style={{ paddingLeft: `${8 + depth * 16}px` }}
                    onClick={() => {
                        setSelectedPath(node.path)
                        if (node.kind === 'leaf') {
                            return
                        } else {
                            setExpanded((prev) => {
                                const next = new Set(prev)
                                if (next.has(node.path)) next.delete(node.path)
                                else next.add(node.path)
                                return next
                            })
                        }
                    }}
                >
                    {canExpand ? (
                        isExpanded ? <ChevronDown className="h-3.5 w-3.5 flex-shrink-0" /> : <ChevronRight className="h-3.5 w-3.5 flex-shrink-0" />
                    ) : (
                        <Type className="h-3.5 w-3.5 flex-shrink-0 text-gray-400" />
                    )}
                    <span className="min-w-0 flex-1 truncate font-medium">{name}</span>
                    <span className="flex-shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-slate-800 dark:text-slate-400">
                        {typeLabel(node)}
                    </span>
                    {node.kind === 'leaf' && (
                        <span className="hidden max-w-[180px] truncate font-mono text-[11px] text-gray-500 dark:text-slate-400 sm:block">
                            {describeBinding(node.binding)}
                        </span>
                    )}
                </button>
                {canExpand && isExpanded && node.children.map((child) => renderNode(child, depth + 1))}
            </div>
        )
    }

    if (!parsed.ok || !parsed.root) {
        return (
            <div className={clsx('flex flex-col border border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/20', heightClass)}>
                <div className="flex items-start gap-2 border-b border-amber-200 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:text-amber-200">
                    <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0" />
                    <div>
                        <div className="font-medium">Visual mode needs valid JSON.</div>
                        <div className="mt-1 text-xs text-amber-700 dark:text-amber-300">{parsed.error}</div>
                    </div>
                </div>
                <div className="flex flex-wrap gap-2 p-3">
                    <button
                        type="button"
                        disabled={readOnly}
                        onClick={() => replaceRoot(onBodyChange, 'object')}
                        className="inline-flex items-center gap-1.5 rounded-md border border-amber-300 bg-white px-2.5 py-1.5 text-xs font-medium text-amber-800 hover:bg-amber-100 disabled:opacity-50 dark:border-amber-800 dark:bg-slate-950 dark:text-amber-200"
                    >
                        <Braces className="h-3.5 w-3.5" /> Start object
                    </button>
                    <button
                        type="button"
                        disabled={readOnly}
                        onClick={() => replaceRoot(onBodyChange, 'array')}
                        className="inline-flex items-center gap-1.5 rounded-md border border-amber-300 bg-white px-2.5 py-1.5 text-xs font-medium text-amber-800 hover:bg-amber-100 disabled:opacity-50 dark:border-amber-800 dark:bg-slate-950 dark:text-amber-200"
                    >
                        <ListPlus className="h-3.5 w-3.5" /> Start array
                    </button>
                </div>
            </div>
        )
    }

    return (
        <div className={clsx('grid min-h-0 grid-cols-1 overflow-hidden border border-gray-200 bg-white dark:border-slate-800 dark:bg-slate-950 lg:grid-cols-[minmax(0,1fr)_320px]', heightClass)}>
            <div className="min-h-0 overflow-auto">
                {renderNode(parsed.root)}
                {leaves.length === 0 && (
                    <div className="flex items-center gap-2 p-4 text-xs text-gray-500 dark:text-slate-400">
                        <FileJson className="h-4 w-4" />
                        Add fields in text mode to configure leaf values visually.
                    </div>
                )}
            </div>
            <div className="min-h-0 overflow-auto border-t border-gray-200 p-3 dark:border-slate-800 lg:border-l lg:border-t-0">
                {selected ? (
                    <div className="space-y-3">
                        <div>
                            <div className="text-xs font-semibold uppercase text-gray-500 dark:text-slate-400">{selected.kind === 'leaf' ? 'Leaf' : selected.kind}</div>
                            <div className="mt-1 truncate text-sm font-medium text-gray-900 dark:text-slate-100">{selected.key ?? selected.path}</div>
                            <div className="mt-0.5 truncate font-mono text-xs text-gray-500 dark:text-slate-400">{selected.path}</div>
                        </div>

                        {selected.kind !== 'leaf' && (
                            <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                                Data mapper
                                <select
                                    disabled={readOnly || collectionSources.length === 0}
                                    value=""
                                    onChange={(event) => {
                                        const source = collectionSources.find((item) => item.id === event.target.value)
                                        if (!source) return
                                        updateRoot(updateNode(parsed.root!, selected.path, (target) => bindNodeToCollection(target, source)))
                                    }}
                                    className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                                >
                                    <option value="">{collectionSources.length ? 'Map children from collection output' : 'No collection outputs available'}</option>
                                    {collectionSources.map((source) => (
                                        <option key={source.id} value={source.id}>
                                            {source.label}{source.collectionOperation ? ` (${source.collectionOperation})` : ''}
                                        </option>
                                    ))}
                                </select>
                                <span className="mt-1 block text-[11px] font-normal text-gray-500 dark:text-slate-400">
                                    Child leaves use matching field names under the selected mapper output.
                                </span>
                            </label>
                        )}

                        {selected.kind === 'leaf' && (
                            <>
                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                            Type
                            <select
                                disabled={readOnly}
                                value={selected.valueType}
                                onChange={(event) => updateRoot(updateLeafType(parsed.root!, selected.path, event.target.value as LeafValueType))}
                                className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                            >
                                <option value="string">String</option>
                                <option value="number">Number</option>
                                <option value="boolean">Boolean</option>
                                <option value="null">Null</option>
                            </select>
                        </label>

                        <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                            Binding
                            <select
                                disabled={readOnly}
                                value={selected.binding.kind === 'template' ? 'template' : 'literal'}
                                onChange={(event) => {
                                    const binding: LeafBinding = event.target.value === 'template'
                                        ? { kind: 'template', expression: '{{.Path.id}}' }
                                        : { kind: 'literal', value: parseLiteral('', selected.valueType) }
                                    updateRoot(updateLeafBinding(parsed.root!, selected.path, binding))
                                }}
                                className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                            >
                                <option value="literal">Literal</option>
                                <option value="template">Template source</option>
                            </select>
                        </label>

                        {selected.binding.kind === 'literal' ? (
                            <label className="block text-xs font-medium text-gray-600 dark:text-slate-300">
                                Value
                                <input
                                    disabled={readOnly || selected.valueType === 'null'}
                                    value={selected.binding.value == null ? '' : String(selected.binding.value)}
                                    onChange={(event) => updateRoot(updateLeafBinding(parsed.root!, selected.path, { kind: 'literal', value: parseLiteral(event.target.value, selected.valueType) }))}
                                    className="mt-1 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                                />
                            </label>
                        ) : selected.binding.kind === 'template' ? (
                            renderTemplateControls(selected.binding, selected)
                        ) : (
                            <div className="rounded-md bg-amber-50 p-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
                                This value can stay in text mode, or switch it to a literal/template binding.
                            </div>
                        )}

                        {selected.valueType !== 'string' && selected.binding.kind === 'template' && (
                            <div className="flex gap-2 rounded-md bg-amber-50 p-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
                                <Code2 className="h-4 w-4 flex-shrink-0" />
                                Non-string template leaves must render valid JSON for this type.
                            </div>
                        )}
                            </>
                        )}
                    </div>
                ) : (
                    <div className="text-xs text-gray-500 dark:text-slate-400">Select a leaf value to configure its source.</div>
                )}
            </div>
        </div>
    )
}
