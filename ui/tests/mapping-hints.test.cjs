const assert = require('node:assert/strict')
const fs = require('node:fs')
const ts = require('typescript')
// Load local TS/TSX in this isolated Node test process without adding a framework.
for (const ext of ['.ts', '.tsx']) require.extensions[ext] = (module, filename) => {
    const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true } })
    module._compile(compiled.outputText, filename)
}
const { hintAvailable, visibleHints } = require('../src/components/shared/mappingHintsLogic.ts')
const hint = (overrides = {}) => ({ source: 'script', key: 'user.name', type: 'string', origin: 'configured', producerId: 'earlier', scope: 'operation', order: 1, ...overrides })
assert.equal(hintAvailable(hint(), { scope: 'operation', order: 2 }), true)
assert.equal(hintAvailable(hint({ order: 3 }), { scope: 'operation', order: 2 }), false)
assert.equal(hintAvailable(hint(), { scope: 'operation', order: 2, stepId: 'earlier' }), false)
assert.equal(hintAvailable(hint({ scope: 'response' }), { scope: 'response' }), false)
assert.equal(hintAvailable(hint({ scope: 'operation' }), { scope: 'spec', order: 99 }), false)
assert.equal(hintAvailable(hint({ order: 2 }), { scope: 'operation', order: 2, stepType: 'validation' }), true)
assert.equal(hintAvailable(hint({ order: 2, source: 'collection' }), { scope: 'operation', order: 2, stepType: 'validation' }), false)
const selected = visibleHints([hint(), hint({ producerId: 'later', key: 'user.id', order: 2 })], { scope: 'response' })
assert.deepEqual(selected.map(h => h.key), ['user.id'], 'earlier overwritten namespace fields must not leak')
const React = require('react')
const { renderToStaticMarkup } = require('react-dom/server')
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query')
const { SuggestedFieldInput, MappingHintsProvider, shapeHints } = require('../src/components/shared/MappingHints.tsx')
const client = new QueryClient()
const html = renderToStaticMarkup(React.createElement(QueryClientProvider, { client },
    React.createElement(MappingHintsProvider, { extra: [{ source: 'body', key: 'active', type: 'boolean', origin: 'schema', values: ['false', 'true'] }] },
        React.createElement(SuggestedFieldInput, { hintSource: 'body', value: '', onChange() {} }),
        React.createElement(SuggestedFieldInput, { hintSource: 'body', valueKey: 'active', value: '', onChange() {}, suggestions: [{ value: 'today', description: 'Current date' }] }),
    )))
const ids = [...html.matchAll(/<datalist id="([^"]+)"/g)].map(m => m[1])
assert.equal(new Set(ids).size, 2, 'each input needs a unique datalist')
for (const value of ['active', 'false', 'true', 'today']) assert.ok(html.includes(`value="${value}"`), `missing suggestion ${value}`)
assert.ok(shapeHints({ addresses: [{ city: 'Boston' }] }, 'target').some(h => h.key === 'addresses.city'))
const topLevelHTML = renderToStaticMarkup(React.createElement(QueryClientProvider, { client }, React.createElement(MappingHintsProvider, { extra: [{ source: 'document', key: '_id', type: 'string', origin: 'collection' }, { source: 'document', key: 'name', type: 'string', origin: 'collection' }] }, React.createElement(SuggestedFieldInput, { hintSource: 'document', topLevel: true, value: '', onChange() {} }))))
assert.ok(topLevelHTML.includes('value="name"'))
assert.ok(!topLevelHTML.includes('value="_id"'), 'immutable collection ID should not be suggested as a write target')
client.clear()
console.log('Mapping hints: availability, shadowing, unique datalists, values, and response paths passed.')
