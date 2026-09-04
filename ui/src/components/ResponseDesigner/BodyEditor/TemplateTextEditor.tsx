import { useCallback } from 'react'
import Editor from '@monaco-editor/react'
import type * as Monaco from 'monaco-editor'

interface TemplateTextEditorProps {
    body: string
    onBodyChange: (body: string) => void
    height: string
    readOnly?: boolean
    isDark: boolean
    lineNumbers?: 'on' | 'off'
    minimap?: boolean
    folding?: boolean
    onMount?: (editor: Monaco.editor.IStandaloneCodeEditor, monaco: typeof Monaco) => void
}

export default function TemplateTextEditor({
    body,
    onBodyChange,
    height,
    readOnly = false,
    isDark,
    lineNumbers = 'off',
    minimap = false,
    folding = false,
    onMount,
}: TemplateTextEditorProps) {
    const registerTemplateLanguage = useCallback((monaco: typeof Monaco) => {
        const exists = monaco.languages.getLanguages().some((lang) => lang.id === 'go-template')
        if (exists) return

        monaco.languages.register({ id: 'go-template' })
        monaco.languages.setMonarchTokensProvider('go-template', {
            tokenizer: {
                root: [
                    [/\{\{/, { token: 'keyword', next: '@template' }],
                    [/[^{}]+/, ''],
                ],
                template: [
                    [/\}\}/, { token: 'keyword', next: '@root' }],
                    [/[^}]+/, 'keyword'],
                ],
            },
        })
    }, [])

    return (
        <Editor
            height={height}
            defaultLanguage="go-template"
            language="go-template"
            value={body}
            onChange={(value) => onBodyChange(value || '')}
            onMount={(editor, monaco) => {
                registerTemplateLanguage(monaco)
                const model = editor.getModel()
                if (model) {
                    monaco.editor.setModelLanguage(model, 'go-template')
                }
                onMount?.(editor, monaco)
            }}
            options={{
                minimap: { enabled: minimap },
                fontSize: lineNumbers === 'on' ? 14 : 13,
                lineNumbers,
                folding,
                scrollBeyondLastLine: false,
                readOnly,
                theme: isDark ? 'vs-dark' : 'light',
                wordWrap: 'on',
                automaticLayout: true,
                padding: lineNumbers === 'on' ? { top: 12 } : undefined,
            }}
        />
    )
}

