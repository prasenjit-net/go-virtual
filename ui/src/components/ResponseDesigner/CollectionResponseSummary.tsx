import type { CollectionFilter, CollectionResponseConfig } from '../../types'

function MappingDetails({ title, rules }: { title: string; rules: CollectionFilter[] }) {
    return <div>
        <h5 className="text-sm font-medium text-gray-700 dark:text-slate-300 mb-2">{title}</h5>
        <div className="overflow-x-auto rounded border border-gray-200 dark:border-slate-700">
            <table className="w-full text-left text-sm">
                <thead className="bg-gray-100 dark:bg-slate-800 text-gray-600 dark:text-slate-300">
                    <tr>
                        <th className="px-3 py-2 font-medium">Collection field</th>
                        <th className="px-3 py-2 font-medium">Source / key</th>
                        <th className="px-3 py-2 font-medium">When missing</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 dark:divide-slate-700 text-gray-900 dark:text-slate-100">
                    {rules.map((rule, index) => <tr key={`${rule.targetPath}-${index}`}>
                        <td className="px-3 py-2 font-mono align-top break-all">{rule.targetPath}</td>
                        <td className="px-3 py-2 align-top">
                            <span className="text-gray-500 dark:text-slate-400">{rule.value.source}</span>
                            <code className="block whitespace-pre-wrap break-all">{rule.value.source === 'literal' ? JSON.stringify(rule.value.value) : rule.value.key}</code>
                        </td>
                        <td className="px-3 py-2 align-top whitespace-pre-wrap break-all">
                            {rule.value.source === 'literal' ? '—' : rule.value.skipWhenMissing ? 'Skip mapping'
                                : Object.prototype.hasOwnProperty.call(rule.value, 'defaultValue') ? `Default: ${JSON.stringify(rule.value.defaultValue)}` : 'Existing behavior'}
                        </td>
                    </tr>)}
                </tbody>
            </table>
        </div>
    </div>
}

export default function CollectionResponseSummary({ response }: { response: CollectionResponseConfig }) {
    const { primary } = response
    const modes = { 'find-one': 'Find One', 'find-many': 'Find Many', insert: 'Insert', update: 'Update', upsert: 'Upsert' }
    const conditionOnly = primary.mode === 'insert' || primary.mode === 'upsert'
    const writesData = primary.mode === 'insert' || primary.mode === 'update' || primary.mode === 'upsert'
    const filters = primary.filterRules || []

    return <section className="space-y-3">
        <h4 className="text-sm font-medium text-gray-700 dark:text-slate-300">Main mapper</h4>
        <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
            <div><dt className="text-gray-500 dark:text-slate-400">Collection</dt><dd className="font-mono text-gray-900 dark:text-slate-100 break-all">{primary.collectionName}</dd></div>
            <div><dt className="text-gray-500 dark:text-slate-400">Operation</dt><dd className="text-gray-900 dark:text-slate-100">{primary.mode ? modes[primary.mode] : 'Automatic (from response shape)'}</dd></div>
        </dl>
        <p className="text-sm text-gray-500 dark:text-slate-400">
            {conditionOnly ? 'Selection uses response conditions only. The main operation runs after selection.'
                : response.matchOnEmpty ? 'Selection also allows an empty query result.' : 'Selection requires query results in addition to response conditions.'}
        </p>
        {primary.mode === 'insert'
            ? <p className="text-sm text-gray-500 dark:text-slate-400">Insert has no query filters.</p>
            : filters.length > 0
                ? <MappingDetails title={primary.mode === 'upsert' ? 'Query filters (used after selection)' : 'Query filters (all must match)'} rules={filters} />
                : <p className="text-sm text-gray-500 dark:text-slate-400">No query filters — all collection documents are eligible.</p>}
        {writesData && (primary.dataRules?.length
            ? <MappingDetails title="Data fields" rules={primary.dataRules} />
            : <p className="text-sm text-gray-500 dark:text-slate-400">No data fields configured.</p>)}
    </section>
}
