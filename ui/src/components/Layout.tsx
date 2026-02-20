import { useEffect, useMemo, useState } from 'react'
import { Outlet, NavLink } from 'react-router-dom'
import {
    LayoutDashboard,
    FileCode2,
    Activity,
    Sun,
    Moon,
    Monitor,
    Tags
} from 'lucide-react'
import { LogoFull } from './Logo'
import clsx from 'clsx'

const navItems = [
    { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { to: '/specs', icon: FileCode2, label: 'API Specs' },
    { to: '/traces', icon: Activity, label: 'Traces' },
    { to: '/tags', icon: Tags, label: 'Tags' },
]

type ThemeMode = 'light' | 'dark' | 'system'

const themeStorageKey = 'go-virtual-theme'

const getInitialTheme = (): ThemeMode => {
    if (typeof window === 'undefined') return 'system'
    const stored = window.localStorage.getItem(themeStorageKey)
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored
    return 'system'
}

const applyThemeMode = (mode: ThemeMode) => {
    if (typeof window === 'undefined') return
    const root = document.documentElement
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    const useDark = mode === 'dark' || (mode === 'system' && prefersDark)
    root.classList.toggle('dark', useDark)
    root.style.colorScheme = useDark ? 'dark' : 'light'
}

export default function Layout() {
    const [themeMode, setThemeMode] = useState<ThemeMode>(getInitialTheme)

    useEffect(() => {
        applyThemeMode(themeMode)
        if (typeof window !== 'undefined') {
            window.localStorage.setItem(themeStorageKey, themeMode)
        }
    }, [themeMode])

    useEffect(() => {
        if (typeof window === 'undefined') return
        const media = window.matchMedia('(prefers-color-scheme: dark)')
        const handler = () => {
            if (themeMode === 'system') {
                applyThemeMode('system')
            }
        }
        if (media.addEventListener) {
            media.addEventListener('change', handler)
        } else {
            media.addListener(handler)
        }
        return () => {
            if (media.removeEventListener) {
                media.removeEventListener('change', handler)
            } else {
                media.removeListener(handler)
            }
        }
    }, [themeMode])

    const themeOptions = useMemo(() => (
        [
            { value: 'light' as const, label: 'Light', icon: Sun },
            { value: 'system' as const, label: 'System', icon: Monitor },
            { value: 'dark' as const, label: 'Dark', icon: Moon },
        ]
    ), [])

    return (
        <div className="h-screen bg-gray-50 text-gray-900 dark:bg-slate-950 dark:text-slate-100 flex overflow-hidden">
            {/* Sidebar */}
            <aside className="w-64 h-screen sticky top-0 bg-white dark:bg-slate-900 border-r border-gray-200 dark:border-slate-800 flex flex-col">
                {/* Logo */}
                <div className="h-16 flex items-center px-5 border-b border-gray-200 dark:border-slate-800">
                    <LogoFull iconSize={36} />
                </div>

                {/* Navigation */}
                <nav className="flex-1 px-4 py-6 overflow-y-auto">
                    <ul className="space-y-1">
                        {navItems.map((item) => (
                            <li key={item.to}>
                                <NavLink
                                    to={item.to}
                                    className={({ isActive }) =>
                                        clsx(
                                            'flex items-center px-4 py-2.5 rounded-lg text-sm font-medium transition-colors',
                                            isActive
                                                ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-200'
                                                : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100'
                                        )
                                    }
                                >
                                    <item.icon className="w-5 h-5 mr-3" />
                                    {item.label}
                                </NavLink>
                            </li>
                        ))}
                    </ul>
                </nav>

                <div className="mt-auto">
                    {/* Theme Toggle */}
                    <div className="px-4 pb-4">
                        <div className="text-xs font-semibold text-gray-500 dark:text-slate-400 mb-2">
                            Theme
                        </div>
                        <div className="grid grid-cols-3 gap-1 bg-gray-100 dark:bg-slate-800 p-1 rounded-lg">
                            {themeOptions.map((option) => (
                                <button
                                    key={option.value}
                                    type="button"
                                    onClick={() => setThemeMode(option.value)}
                                    aria-pressed={themeMode === option.value}
                                    className={clsx(
                                        'flex items-center justify-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors',
                                        themeMode === option.value
                                            ? 'bg-white text-gray-900 shadow-sm dark:bg-slate-700 dark:text-slate-100'
                                            : 'text-gray-500 hover:text-gray-900 dark:text-slate-400 dark:hover:text-slate-100'
                                    )}
                                >
                                    <option.icon className="w-3.5 h-3.5" />
                                    {option.label}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Footer */}
                    <div className="p-4 border-t border-gray-200 dark:border-slate-800">
                        <div className="text-xs text-gray-500 dark:text-slate-400">
                            <p>Go-Virtual v0.1.0</p>
                            <p>OpenAPI 3 Proxy Service</p>
                        </div>
                    </div>
                </div>
            </aside>

            {/* Main Content */}
            <main className="flex-1 overflow-y-auto">
                <Outlet />
            </main>
        </div>
    )
}
