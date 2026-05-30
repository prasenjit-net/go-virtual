import { useState, useRef, useEffect } from 'react'
import { Sparkles, X, Loader2, AlertCircle, User, Bot, ChevronDown, ChevronUp, CheckCircle } from 'lucide-react'
import { aiApi } from '../../services/api'

export type AiChatMessage = { role: 'user' | 'assistant'; content: string }

interface Props {
    /** Script currently in the editor — sent as context on every turn. */
    currentSource: string
    /** Conversation history maintained by the parent. */
    history: AiChatMessage[]
    onHistoryChange: (updated: AiChatMessage[]) => void
    /** Optional: provide operation context to the AI. */
    operationId?: string
    /** Called after each successful generation with the new source. */
    onGenerated: (source: string) => void
    onClose: () => void
}

export default function AIScriptModal({
    currentSource,
    history,
    onHistoryChange,
    operationId,
    onGenerated,
    onClose,
}: Props) {
    const [prompt, setPrompt] = useState('')
    const [isGenerating, setIsGenerating] = useState(false)
    const [error, setError] = useState<string | null>(null)
    // Track which assistant turns have their script expanded
    const [expanded, setExpanded] = useState<Record<number, boolean>>({})
    const bottomRef = useRef<HTMLDivElement>(null)

    // Auto-scroll to bottom on new messages
    useEffect(() => {
        bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }, [history])

    const handleGenerate = async () => {
        const trimmed = prompt.trim()
        if (!trimmed || isGenerating) return

        setIsGenerating(true)
        setError(null)

        // Optimistically append the user message so the UI feels instant.
        const userMsg: AiChatMessage = { role: 'user', content: trimmed }
        const updatedHistory = [...history, userMsg]
        onHistoryChange(updatedHistory)
        setPrompt('')

        try {
            const result = await aiApi.generateScript(trimmed, {
                operationId,
                currentSource,
                // Send only the prior history (before this user message) as context.
                history,
            })

            const assistantMsg: AiChatMessage = { role: 'assistant', content: result.source }
            onHistoryChange([...updatedHistory, assistantMsg])
            onGenerated(result.source)
        } catch (err: any) {
            // Roll back the optimistic user message on failure.
            onHistoryChange(history)
            setPrompt(trimmed)
            setError(err?.message ?? 'Failed to generate script')
        } finally {
            setIsGenerating(false)
        }
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            handleGenerate()
        }
    }

    const toggleExpand = (idx: number) => {
        setExpanded(prev => ({ ...prev, [idx]: !prev[idx] }))
    }

    const isEmpty = history.length === 0

    return (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/50">
            <div className="bg-white dark:bg-slate-900 rounded-t-2xl sm:rounded-xl shadow-2xl w-full sm:max-w-2xl mx-0 sm:mx-4 flex flex-col"
                 style={{ maxHeight: 'min(90vh, 680px)' }}>

                {/* Header */}
                <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-slate-800 shrink-0">
                    <div className="flex items-center gap-2">
                        <Sparkles className="w-5 h-5 text-purple-500" />
                        <h2 className="text-lg font-semibold text-gray-900 dark:text-slate-100">AI Script Assistant</h2>
                        {history.length > 0 && (
                            <span className="text-xs text-purple-600 dark:text-purple-400 bg-purple-100 dark:bg-purple-900/30 px-2 py-0.5 rounded-full">
                                {Math.ceil(history.length / 2)} turn{Math.ceil(history.length / 2) !== 1 ? 's' : ''}
                            </span>
                        )}
                    </div>
                    <div className="flex items-center gap-2">
                        {history.length > 0 && (
                            <button
                                onClick={onClose}
                                className="px-3 py-1.5 text-sm font-medium text-white bg-purple-600 rounded-lg hover:bg-purple-700 transition-colors flex items-center gap-1.5"
                            >
                                <CheckCircle className="w-4 h-4" />
                                Done
                            </button>
                        )}
                        <button
                            onClick={onClose}
                            className="text-gray-400 hover:text-gray-600 dark:hover:text-slate-300 transition-colors"
                            aria-label="Close"
                        >
                            <X className="w-5 h-5" />
                        </button>
                    </div>
                </div>

                {/* Conversation thread */}
                <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4 min-h-0">
                    {isEmpty && (
                        <div className="flex flex-col items-center justify-center py-8 text-center text-gray-400 dark:text-slate-500 space-y-3">
                            <Sparkles className="w-10 h-10 text-purple-300 dark:text-purple-700" />
                            <div>
                                <p className="font-medium text-gray-600 dark:text-slate-400">Describe what the script should do</p>
                                <p className="text-sm mt-1">You can refine it iteratively — the AI remembers the conversation</p>
                            </div>
                            <div className="text-xs bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-700 rounded-lg px-4 py-3 text-left text-purple-800 dark:text-purple-300 w-full max-w-sm">
                                <p className="font-medium mb-1">Available in every script:</p>
                                <ul className="space-y-0.5 text-purple-700 dark:text-purple-400">
                                    <li><code className="font-mono">req["path/query/header/body"]</code></li>
                                    <li><code className="font-mono">store.get/set/has/delete/keys()</code></li>
                                    <li><code className="font-mono">log(...)</code> — trace logging</li>
                                </ul>
                            </div>
                        </div>
                    )}

                    {history.map((msg, idx) => (
                        <div key={idx} className={`flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : 'flex-row'}`}>
                            {/* Avatar */}
                            <div className={`w-7 h-7 rounded-full flex items-center justify-center shrink-0 mt-0.5
                                ${msg.role === 'user'
                                    ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400'
                                    : 'bg-purple-100 dark:bg-purple-900/40 text-purple-600 dark:text-purple-400'}`}>
                                {msg.role === 'user'
                                    ? <User className="w-3.5 h-3.5" />
                                    : <Bot className="w-3.5 h-3.5" />}
                            </div>

                            {/* Bubble */}
                            <div className={`max-w-[85%] rounded-xl px-4 py-2.5 text-sm
                                ${msg.role === 'user'
                                    ? 'bg-blue-600 text-white'
                                    : 'bg-gray-100 dark:bg-slate-800 text-gray-800 dark:text-slate-200'}`}>
                                {msg.role === 'user' ? (
                                    <p className="whitespace-pre-wrap">{msg.content}</p>
                                ) : (
                                    <div>
                                        <div className="flex items-center gap-2 mb-1">
                                            <CheckCircle className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
                                            <span className="text-emerald-700 dark:text-emerald-400 font-medium text-xs">Script generated</span>
                                            <button
                                                onClick={() => toggleExpand(idx)}
                                                className="ml-auto text-gray-400 hover:text-gray-600 dark:hover:text-slate-300 transition-colors"
                                            >
                                                {expanded[idx]
                                                    ? <ChevronUp className="w-3.5 h-3.5" />
                                                    : <ChevronDown className="w-3.5 h-3.5" />}
                                            </button>
                                        </div>
                                        {expanded[idx] && (
                                            <pre className="mt-2 text-xs font-mono bg-gray-800 dark:bg-slate-950 text-gray-100 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap max-h-48 overflow-y-auto">
                                                {msg.content}
                                            </pre>
                                        )}
                                        {!expanded[idx] && (
                                            <p className="text-xs text-gray-500 dark:text-slate-400 font-mono truncate">
                                                {msg.content.split('\n').find(l => l.trim() && !l.startsWith('#')) ?? msg.content.split('\n')[0]}
                                            </p>
                                        )}
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}

                    {isGenerating && (
                        <div className="flex gap-3">
                            <div className="w-7 h-7 rounded-full bg-purple-100 dark:bg-purple-900/40 text-purple-600 dark:text-purple-400 flex items-center justify-center shrink-0">
                                <Bot className="w-3.5 h-3.5" />
                            </div>
                            <div className="bg-gray-100 dark:bg-slate-800 rounded-xl px-4 py-3 flex items-center gap-2 text-sm text-gray-500 dark:text-slate-400">
                                <Loader2 className="w-4 h-4 animate-spin text-purple-500" />
                                Generating script…
                            </div>
                        </div>
                    )}

                    {error && (
                        <div className="flex items-start gap-2 text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg px-4 py-3 text-sm">
                            <AlertCircle className="w-4 h-4 mt-0.5 shrink-0" />
                            <span>{error}</span>
                        </div>
                    )}

                    <div ref={bottomRef} />
                </div>

                {/* Input area */}
                <div className="px-6 py-4 border-t border-gray-200 dark:border-slate-800 shrink-0">
                    <div className="flex gap-2 items-end">
                        <textarea
                            className="flex-1 rounded-lg border border-gray-300 dark:border-slate-600 px-3 py-2 text-sm
                                       bg-white dark:bg-slate-800 text-gray-900 dark:text-slate-100
                                       focus:outline-none focus:ring-2 focus:ring-purple-500 focus:border-transparent
                                       resize-none placeholder:text-gray-400 dark:placeholder:text-slate-500"
                            rows={2}
                            placeholder={isEmpty
                                ? 'Describe what the script should do… (⌘+Enter to generate)'
                                : 'Ask for a change or refinement… (⌘+Enter)'}
                            value={prompt}
                            onChange={e => setPrompt(e.target.value)}
                            onKeyDown={handleKeyDown}
                            disabled={isGenerating}
                            autoFocus
                        />
                        <button
                            onClick={handleGenerate}
                            disabled={isGenerating || !prompt.trim()}
                            className="flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white
                                       bg-purple-600 rounded-lg hover:bg-purple-700 transition-colors
                                       disabled:opacity-50 disabled:cursor-not-allowed shrink-0 h-[60px]"
                        >
                            {isGenerating
                                ? <Loader2 className="w-4 h-4 animate-spin" />
                                : <Sparkles className="w-4 h-4" />}
                            {isGenerating ? 'Wait…' : 'Send'}
                        </button>
                    </div>
                    {history.length > 0 && (
                        <p className="text-xs text-gray-400 dark:text-slate-500 mt-1.5">
                            Context is maintained across turns. Saving the script will clear this conversation.
                        </p>
                    )}
                </div>
            </div>
        </div>
    )
}
