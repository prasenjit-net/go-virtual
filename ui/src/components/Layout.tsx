import { useEffect, useMemo, useState } from 'react'
import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
    LayoutDashboard,
    FileCode2,
    Activity,
    Sun,
    Moon,
    Monitor,
    Tags,
    Code2,
    Database,
    Users,
    BookOpen,
    Archive,
    Bot,
    Menu,
} from 'lucide-react'
import { LogoFull, LogoIcon } from './Logo'
import { brandingApi } from '../services/api'
import type { Branding } from '../types'
import clsx from 'clsx'

const navItems = [
    { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { to: '/specs', icon: FileCode2, label: 'API Specs' },
    { to: '/traces', icon: Activity, label: 'Traces' },
    { to: '/scripts', icon: Code2, label: 'Scripts' },
    { to: '/ai-scenarios', icon: Bot, label: 'AI Scenarios' },
    { to: '/store', icon: Database, label: 'Store' },
    { to: '/sessions', icon: Users, label: 'Sessions' },
    { to: '/archives', icon: Archive, label: 'Archives' },
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
    const { pathname } = useLocation()
    const [themeMode, setThemeMode] = useState<ThemeMode>(getInitialTheme)
    const [sidebarOpen, setSidebarOpen] = useState(false)
    const isSpecDesigner = /^\/specs\/[^/]+\/designer(?:\/|$)/.test(pathname)

    // Fetch branding config — stale forever (only changes on server restart)
    const { data: branding } = useQuery<Branding>({
        queryKey: ['branding'],
        queryFn: brandingApi.get,
        staleTime: Infinity,
        retry: false,
    })

    // Keep document.title in sync with branding
    useEffect(() => {
        const title = branding?.appTitle?.trim() || 'go-virtual'
        document.title = title === 'go-virtual'
            ? 'go-virtual — API Mock & Virtualization'
            : title
    }, [branding?.appTitle])

    useEffect(() => {
        applyThemeMode(themeMode)
        if (typeof window !== 'undefined') {
            window.localStorage.setItem(themeStorageKey, themeMode)
        }
    }, [themeMode])

    useEffect(() => {
        if (isSpecDesigner) setSidebarOpen(false)
    }, [isSpecDesigner])

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
            {/* Mobile top bar */}
            <div className="flex lg:hidden fixed top-0 left-0 right-0 z-40 h-14 items-center justify-between px-4 bg-white dark:bg-slate-900 border-b border-gray-200 dark:border-slate-800">
                <button
                    onClick={() => setSidebarOpen(true)}
                    className="p-2 rounded-lg text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-800"
                >
                    <Menu className="w-5 h-5" />
                </button>
                <LogoFull iconSize={28} title={branding?.appTitle} />
                <div className="flex items-center gap-0.5">
                    {themeOptions.map((option) => (
                        <button
                            key={option.value}
                            type="button"
                            onClick={() => setThemeMode(option.value)}
                            aria-pressed={themeMode === option.value}
                            className={clsx(
                                'p-1.5 rounded-md transition-colors',
                                themeMode === option.value
                                    ? 'bg-gray-100 text-gray-900 dark:bg-slate-700 dark:text-slate-100'
                                    : 'text-gray-500 hover:text-gray-900 dark:text-slate-400 dark:hover:text-slate-100'
                            )}
                        >
                            <option.icon className="w-4 h-4" />
                        </button>
                    ))}
                </div>
            </div>

            {/* Backdrop */}
            {sidebarOpen && (
                <div
                    className="fixed inset-0 bg-black/50 z-20 lg:hidden"
                    onClick={() => setSidebarOpen(false)}
                />
            )}

            {/* Sidebar */}
            <aside className={clsx(
                "fixed lg:relative inset-y-0 left-0 z-30 w-64 h-full bg-white dark:bg-slate-900 border-r border-gray-200 dark:border-slate-800 flex flex-col transition-[transform,width] duration-300",
                isSpecDesigner ? "lg:w-20" : "lg:w-64",
                sidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
            )}>
                {/* Logo (desktop only) */}
                <div className={clsx('hidden lg:flex h-16 items-center border-b border-gray-200 dark:border-slate-800', isSpecDesigner ? 'justify-center px-2' : 'px-5')}>
                    {isSpecDesigner ? <LogoIcon size={34} /> : <LogoFull iconSize={36} title={branding?.appTitle} />}
                </div>
                {/* Spacer for mobile top bar */}
                <div className="h-14 lg:hidden" />

                {/* Navigation */}
                <nav className={clsx('flex-1 py-6 overflow-y-auto', isSpecDesigner ? 'px-4 lg:px-2' : 'px-4')}>
                    <ul className="space-y-1">
                        {navItems.map((item) => (
                            <li key={item.to}>
                                <NavLink
                                    to={item.to}
                                    onClick={() => setSidebarOpen(false)}
                                    title={item.label}
                                    className={({ isActive }) =>
                                        clsx(
                                            'flex items-center px-4 py-2.5 rounded-lg text-sm font-medium transition-colors',
                                            isSpecDesigner && 'lg:justify-center lg:px-2',
                                            isActive
                                                ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-200'
                                                : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100'
                                        )
                                    }
                                >
                                    <item.icon className={clsx('w-5 h-5 shrink-0', isSpecDesigner ? 'lg:mr-0' : 'mr-3')} />
                                    <span className={isSpecDesigner ? 'lg:sr-only' : undefined}>{item.label}</span>
                                </NavLink>
                            </li>
                        ))}
                        <li>
                            <a
                                href="/_docs/"
                                target="_blank"
                                rel="noreferrer"
                                onClick={() => setSidebarOpen(false)}
                                title="Docs"
                                className={clsx('flex items-center px-4 py-2.5 rounded-lg text-sm font-medium transition-colors text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100', isSpecDesigner && 'lg:justify-center lg:px-2')}
                            >
                                <BookOpen className={clsx('w-5 h-5 shrink-0', isSpecDesigner ? 'lg:mr-0' : 'mr-3')} />
                                <span className={isSpecDesigner ? 'lg:sr-only' : undefined}>Docs</span>
                            </a>
                        </li>
                    </ul>
                </nav>

                <div className="mt-auto">
                    {/* Theme Toggle */}
                    <div className={clsx('px-4 pb-4', isSpecDesigner && 'lg:px-2')}>
                        <div className={clsx('text-xs font-semibold text-gray-500 dark:text-slate-400 mb-2', isSpecDesigner && 'lg:sr-only')}>
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
                                    <span className={isSpecDesigner ? 'lg:sr-only' : undefined}>{option.label}</span>
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Footer */}
                    <div className={clsx('p-4 border-t border-gray-200 dark:border-slate-800', isSpecDesigner && 'lg:hidden')}>
                        <div className="text-xs text-gray-500 dark:text-slate-400">
                            <p className="font-medium">{branding?.appTitle?.trim() || 'go-virtual'}</p>
                            <p>{branding?.appSubtitle?.trim() || 'API Mock & Virtualization'}</p>
                        </div>
                    </div>
                </div>
            </aside>

            {/* Main Content — top padding on mobile for the top bar */}
            <main className="flex-1 overflow-y-auto min-w-0 pt-14 lg:pt-0">
                <Outlet />
            </main>
        </div>
    )
}
