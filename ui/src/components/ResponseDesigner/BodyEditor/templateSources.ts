import type { CollectionMapping, CollectionOpType, Operation, ScriptBinding, ValidationRule } from '../../../types'

export type TemplateSourceGroup =
    | 'request'
    | 'runtime'
    | 'script'
    | 'validation'
    | 'collection'
    | 'advanced'

export interface TemplateSourceOption {
    id: string
    group: TemplateSourceGroup
    label: string
    detail: string
    snippet: string
    needsPath?: boolean
    disabled?: boolean
    outputKey?: string
    collectionOperation?: CollectionOpType
}

export interface TemplateSourceContext {
    operation?: Operation
    operationScriptBindings?: ScriptBinding[]
    responseScriptBindings?: ScriptBinding[]
    operationCollectionMappings?: CollectionMapping[]
    responseCollectionMappings?: CollectionMapping[]
    validationRules?: ValidationRule[]
    responseConfigId?: string
}

function fieldOptions(group: TemplateSourceGroup, prefix: string, fields: string[] | undefined, detail: string): TemplateSourceOption[] {
    return (fields ?? []).filter(Boolean).map((field) => ({
        id: `${group}:${prefix}:${field}`,
        group,
        label: field,
        detail,
        snippet: `{{.${prefix}.${field}}}`,
    }))
}

function outputOptions(
    group: 'script' | 'collection',
    root: 'Script' | 'Collection',
    outputs: Array<{ outputKey?: string; id: string; name?: string; disabled?: boolean; collectionOperation?: CollectionOpType }>,
): TemplateSourceOption[] {
    return outputs
        .filter((item) => item.outputKey)
        .map((item) => ({
            id: `${group}:${item.id}`,
            group,
            label: item.outputKey || item.id,
            detail: item.name || `${root} output`,
            snippet: `{{.${root}.${item.outputKey}}}`,
            needsPath: true,
            disabled: item.disabled,
            outputKey: item.outputKey,
            collectionOperation: item.collectionOperation,
        }))
}

export function buildTemplateSourceOptions({
    operation,
    operationScriptBindings = [],
    responseScriptBindings = [],
    operationCollectionMappings = [],
    responseCollectionMappings = [],
    validationRules = [],
    responseConfigId,
}: TemplateSourceContext): TemplateSourceOption[] {
    const request: TemplateSourceOption[] = [
        ...fieldOptions('request', 'Path', operation?.declaredPathParams, 'Path parameter'),
        ...fieldOptions('request', 'Query', operation?.declaredQueryParams, 'Query parameter'),
        ...fieldOptions('request', 'Header', operation?.declaredHeaderParams, 'Header parameter'),
        ...fieldOptions('request', 'Body', operation?.declaredBodyFields, 'Request body field'),
        { id: 'request:rawBody', group: 'request', label: 'Raw body', detail: 'Entire request body', snippet: '{{.RawBody}}' },
        { id: 'request:method', group: 'request', label: 'Method', detail: 'HTTP method', snippet: '{{.Method}}' },
        { id: 'request:url', group: 'request', label: 'URL', detail: 'Request URL', snippet: '{{.URL}}' },
        { id: 'request:requestId', group: 'request', label: 'Request ID', detail: 'Stable request UUID', snippet: '{{.RequestID}}' },
        { id: 'request:bodyPath', group: 'request', label: 'Body path', detail: 'JSON body path', snippet: '{{body "field.path"}}', needsPath: true },
    ]

    const runtime: TemplateSourceOption[] = [
        { id: 'runtime:uuid', group: 'runtime', label: 'UUID', detail: 'Random UUID', snippet: '{{random "uuid"}}' },
        { id: 'runtime:int', group: 'runtime', label: 'Integer', detail: 'Random integer', snippet: '{{random "int"}}' },
        { id: 'runtime:email', group: 'runtime', label: 'Email', detail: 'Faker email', snippet: '{{faker "email"}}' },
        { id: 'runtime:name', group: 'runtime', label: 'Name', detail: 'Faker full name', snippet: '{{faker "name"}}' },
        { id: 'runtime:timestamp', group: 'runtime', label: 'Timestamp', detail: 'ISO timestamp', snippet: '{{timestamp "iso"}}' },
        { id: 'runtime:counter', group: 'runtime', label: 'Counter', detail: 'Session counter', snippet: '{{counter "key"}}', needsPath: true },
        { id: 'runtime:store', group: 'runtime', label: 'Store key', detail: 'Read session store', snippet: '{{store "key"}}', needsPath: true },
    ]

    const script = outputOptions('script', 'Script', [
        ...operationScriptBindings.map((binding) => ({ id: binding.id, outputKey: binding.outputKey, name: binding.scriptName })),
        ...responseScriptBindings.map((binding) => ({ id: binding.id, outputKey: binding.outputKey, name: binding.scriptName })),
        ...(!responseConfigId ? [{ id: 'response-script-unavailable', outputKey: 'responseOutput', name: 'Save response to use response-scoped scripts', disabled: true }] : []),
    ])

    const validation: TemplateSourceOption[] = validationRules
        .filter((rule) => rule.name)
        .flatMap((rule) => [
            {
                id: `validation:${rule.id}:status`,
                group: 'validation' as const,
                label: `${rule.name}.status`,
                detail: 'Validation status',
                snippet: `{{.Validation.${rule.name}.status}}`,
            },
            {
                id: `validation:${rule.id}:property`,
                group: 'validation' as const,
                label: rule.name,
                detail: 'Validation output property',
                snippet: `{{.Validation.${rule.name}.property}}`,
                needsPath: true,
            },
        ])

    const collection = outputOptions('collection', 'Collection', [
        ...operationCollectionMappings.map((mapping) => ({ id: mapping.id, outputKey: mapping.outputKey, name: mapping.name || mapping.collectionName, collectionOperation: mapping.operation })),
        ...responseCollectionMappings.map((mapping) => ({ id: mapping.id, outputKey: mapping.outputKey, name: mapping.name || mapping.collectionName, collectionOperation: mapping.operation })),
        ...(!responseConfigId ? [{ id: 'response-collection-unavailable', outputKey: 'responseCollection', name: 'Save response to use response-scoped collections', disabled: true }] : []),
    ])

    return [
        ...request,
        ...runtime,
        ...script,
        ...validation,
        ...collection,
        { id: 'advanced:custom', group: 'advanced', label: 'Custom expression', detail: 'Raw Go template expression', snippet: '{{.Path.id}}', needsPath: true },
    ]
}

export const sourceGroupLabels: Record<TemplateSourceGroup, string> = {
    request: 'Request',
    runtime: 'Runtime',
    script: 'Script',
    validation: 'Validation',
    collection: 'Collection',
    advanced: 'Advanced',
}
