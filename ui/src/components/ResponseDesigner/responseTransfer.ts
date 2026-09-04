import type { CollectionResponseConfig, Condition, ResponseConfig, ResponseConfigInput } from '../../types'

export interface ResponseTransferEnvelope {
    type: 'go-virtual-response-config'
    version: 1
    payload: ResponseConfigInput
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return !!value && typeof value === 'object'
}

function normalizeCondition(input: unknown): Condition | null {
    if (!isRecord(input)) return null
    if (typeof input.source !== 'string' || typeof input.operator !== 'string') return null

    return {
        source: input.source,
        key: typeof input.key === 'string' ? input.key : '',
        operator: input.operator,
        value: typeof input.value === 'string' ? input.value : '',
        format: typeof input.format === 'string' ? input.format : undefined,
        negate: typeof input.negate === 'boolean' ? input.negate : undefined,
    } as Condition
}

function sanitizeInput(input: unknown): ResponseConfigInput {
    const source = isRecord(input) ? input : {}

    const conditions = Array.isArray(source.conditions)
        ? source.conditions.map(normalizeCondition).filter((item): item is Condition => item !== null)
        : []

    const headers: Record<string, string> = {}
    if (isRecord(source.headers)) {
        for (const [key, value] of Object.entries(source.headers)) {
            if (typeof value === 'string') {
                headers[key] = value
            }
        }
    }

    return {
        name: typeof source.name === 'string' ? source.name.trim() : '',
        description: typeof source.description === 'string' ? source.description : '',
        tag: typeof source.tag === 'string' ? source.tag : 'default',
        priority: Number.isFinite(source.priority) ? Number(source.priority) : 0,
        conditions,
        statusCode: Number.isFinite(source.statusCode) ? Number(source.statusCode) : 200,
        headers,
        body: typeof source.body === 'string' ? source.body : '',
        delay: Number.isFinite(source.delay) ? Number(source.delay) : 0,
        enabled: typeof source.enabled === 'boolean' ? source.enabled : true,
        kind: source.kind === 'collection' ? 'collection' : undefined,
        collectionResponse: isRecord(source.collectionResponse)
            ? (source.collectionResponse as unknown as CollectionResponseConfig)
            : undefined,
    }
}

export function buildResponseTransferPayload(config: ResponseConfig): ResponseTransferEnvelope {
    return {
        type: 'go-virtual-response-config',
        version: 1,
        payload: sanitizeInput(config),
    }
}

export function serializeResponseForClipboard(config: ResponseConfig): string {
    return JSON.stringify(buildResponseTransferPayload(config), null, 2)
}

export function parseResponseImportPayload(raw: string): ResponseConfigInput {
    let parsed: unknown
    try {
        parsed = JSON.parse(raw)
    } catch {
        throw new Error('Invalid JSON payload')
    }

    const input = isRecord(parsed) && parsed.type === 'go-virtual-response-config'
        ? parsed.payload
        : parsed

    const normalized = sanitizeInput(input)
    if (!normalized.name) {
        throw new Error('Imported response must include a non-empty name')
    }

    return normalized
}
