import { SuggestedFieldInput } from './MappingHints'
interface MappingDefaultInputProps {
    value: string | undefined
    skipWhenMissing?: boolean
    onChange: (value: string | undefined, skipWhenMissing: boolean) => void
    json?: boolean
    hintSource?: string
    valueKey?: string
}

export function isValidDefaultJSON(value: string | undefined): boolean {
    if (value === undefined) return true
    try { JSON.parse(value); return true } catch { return false }
}

export default function MappingDefaultInput({ value, onChange, skipWhenMissing = false, json = false, hintSource = "", valueKey = "" }: MappingDefaultInputProps) {
    const invalid = json && !isValidDefaultJSON(value)
    return <div className="w-full space-y-1 text-xs text-gray-600 dark:text-slate-300">
        <label className="flex items-center gap-2">
            <span>When source is missing</span>
            <select
                value={skipWhenMissing ? 'skip' : value !== undefined ? 'default' : ''}
                onChange={(event) => onChange(event.target.value === 'default' ? (json ? 'null' : '') : undefined, event.target.value === 'skip')}
                className="px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100"
            >
                <option value="">Existing behavior</option>
                <option value="default">Use default value</option>
                <option value="skip">Skip mapping</option>
            </select>
        </label>
        {skipWhenMissing && <p className="text-gray-500 dark:text-slate-400">Ignore this mapping when its source/key is absent. Existing empty or null values are kept.</p>}
        {value !== undefined && <>
            <label className="flex items-center gap-2">
                <span className="shrink-0">Default value{json ? ' (JSON)' : ''}</span>
                <SuggestedFieldInput hintSource={hintSource} valueKey={valueKey} jsonValue={json} type="text" value={value} aria-invalid={invalid}
                    onChange={(event) => onChange(event.target.value, false)}
                    placeholder={json ? '"text", true, 0, null, {}, []' : 'Default text (empty is allowed)'}
                    className="min-w-0 flex-1 px-2 py-1.5 border border-gray-300 dark:border-slate-700 rounded bg-white dark:bg-slate-950 text-gray-900 dark:text-slate-100 font-mono focus:ring-2 focus:ring-primary-500" />
            </label>
            {invalid && <p role="alert" className="text-red-600 dark:text-red-400">Enter valid JSON. Put text values in double quotes.</p>}
            <p className="text-gray-500 dark:text-slate-400">Used only when the source or key is missing. Existing empty or null values are kept.</p>
        </>}
    </div>
}
