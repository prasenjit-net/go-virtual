import type { Condition, ResponseConfig, ResponseConfigInput } from '../../types'

export interface ResponseTransferEnvelope {
    type: 'go-virtual-response-config'
    version: 1
    payload: ResponseConfigInput
}

function normalizeCondition(input: any): Condition | null {
    if (!input || typeof input !== 'object') return null
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

function sanitizeInput(input: any): ResponseConfigInput {
    const conditions = Array.isArray(input?.conditions)
        ? input.conditions.map(normalizeCondition).filter((item): item is Condition => item !== null)
        : []

    const headers: Record<string, string> = {}
    if (input?.headers && typeof input.headers === 'object') {
        for (const [key, value] of Object.entries(input.headers)) {
            if (typeof value === 'string') {
                headers[key] = value
            }
        }
    }

    return {
        name: typeof input?.name === 'string' ? input.name.trim() : '',
        description: typeof input?.description === 'string' ? input.description : '',
        tag: typeof input?.tag === 'string' ? input.tag : 'default',
        priority: Number.isFinite(input?.priority) ? Number(input.priority) : 0,
        conditions,
        statusCode: Number.isFinite(input?.statusCode) ? Number(input.statusCode) : 200,
        headers,
        body: typeof input?.body === 'string' ? input.body : '',
        delay: Number.isFinite(input?.delay) ? Number(input.delay) : 0,
        enabled: typeof input?.enabled === 'boolean' ? input.enabled : true,
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
    let parsed: any
    try {
        parsed = JSON.parse(raw)
    } catch {
        throw new Error('Invalid JSON payload')
    }

    const input = parsed?.type === 'go-virtual-response-config'
        ? parsed?.payload
        : parsed

    const normalized = sanitizeInput(input)
    if (!normalized.name) {
        throw new Error('Imported response must include a non-empty name')
    }

    return normalized
}
