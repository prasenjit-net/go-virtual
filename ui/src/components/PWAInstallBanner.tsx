import { useEffect, useState } from 'react'
import { Download, X } from 'lucide-react'

interface BeforeInstallPromptEvent extends Event {
    prompt: () => Promise<void>
    userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

export default function PWAInstallBanner() {
    const [installEvent, setInstallEvent] = useState<BeforeInstallPromptEvent | null>(null)
    const [dismissed, setDismissed] = useState(
        () => localStorage.getItem('pwa-install-dismissed') === '1'
    )

    useEffect(() => {
        const handler = (e: Event) => {
            e.preventDefault()
            setInstallEvent(e as BeforeInstallPromptEvent)
        }
        window.addEventListener('beforeinstallprompt', handler)
        return () => window.removeEventListener('beforeinstallprompt', handler)
    }, [])

    if (!installEvent || dismissed) return null

    const handleInstall = async () => {
        await installEvent.prompt()
        const { outcome } = await installEvent.userChoice
        if (outcome === 'accepted') setInstallEvent(null)
    }

    const handleDismiss = () => {
        setDismissed(true)
        localStorage.setItem('pwa-install-dismissed', '1')
    }

    return (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 px-4 py-3 bg-indigo-600 text-white rounded-xl shadow-lg text-sm">
            <Download className="w-4 h-4 shrink-0" />
            <span>Install go-virtual as an app for faster access</span>
            <button
                onClick={handleInstall}
                className="px-3 py-1 bg-white text-indigo-600 rounded-lg font-medium hover:bg-indigo-50 transition-colors"
            >
                Install
            </button>
            <button onClick={handleDismiss} className="p-1 hover:bg-indigo-500 rounded transition-colors">
                <X className="w-4 h-4" />
            </button>
        </div>
    )
}
