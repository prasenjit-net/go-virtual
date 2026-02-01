import { Outlet, NavLink } from 'react-router-dom'
import {
    LayoutDashboard,
    FileCode2,
    Activity,
    Server
} from 'lucide-react'
import clsx from 'clsx'

const navItems = [
    { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { to: '/specs', icon: FileCode2, label: 'API Specs' },
    { to: '/traces', icon: Activity, label: 'Traces' },
]

export default function Layout() {
    return (
        <div className="min-h-screen bg-gray-50 flex">
            {/* Sidebar */}
            <aside className="w-64 bg-white border-r border-gray-200 flex flex-col">
                {/* Logo */}
                <div className="h-16 flex items-center px-6 border-b border-gray-200">
                    <Server className="w-8 h-8 text-primary-600" />
                    <span className="ml-3 text-xl font-bold text-gray-900">Go-Virtual</span>
                </div>

                {/* Navigation */}
                <nav className="flex-1 px-4 py-6">
                    <ul className="space-y-1">
                        {navItems.map((item) => (
                            <li key={item.to}>
                                <NavLink
                                    to={item.to}
                                    className={({ isActive }) =>
                                        clsx(
                                            'flex items-center px-4 py-2.5 rounded-lg text-sm font-medium transition-colors',
                                            isActive
                                                ? 'bg-primary-50 text-primary-700'
                                                : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
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

                {/* Footer */}
                <div className="p-4 border-t border-gray-200">
                    <div className="text-xs text-gray-500">
                        <p>Go-Virtual v0.1.0</p>
                        <p>OpenAPI 3 Proxy Service</p>
                    </div>
                </div>
            </aside>

            {/* Main Content */}
            <main className="flex-1 overflow-auto">
                <Outlet />
            </main>
        </div>
    )
}
