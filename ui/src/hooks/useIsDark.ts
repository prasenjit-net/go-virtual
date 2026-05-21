import { useState, useEffect } from 'react'

/** Reactively tracks whether the document root has the 'dark' class. */
export function useIsDark(): boolean {
    const [isDark, setIsDark] = useState(() => document.documentElement.classList.contains('dark'))
    useEffect(() => {
        const observer = new MutationObserver(() => {
            setIsDark(document.documentElement.classList.contains('dark'))
        })
        observer.observe(document.documentElement, { attributeFilter: ['class'] })
        return () => observer.disconnect()
    }, [])
    return isDark
}
