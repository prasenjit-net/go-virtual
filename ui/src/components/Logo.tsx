/**
 * go-virtual Logo components
 *
 * LogoIcon  – square SVG icon (the V-shape network proxy mark)
 * LogoFull  – icon + "go-virtual" wordmark side-by-side
 */

interface LogoIconProps {
    /** Rendered size in pixels (width = height). Default: 36 */
    size?: number
    className?: string
}

export function LogoIcon({ size = 36, className }: LogoIconProps) {
    return (
        <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 100 100"
            width={size}
            height={size}
            className={className}
            role="img"
            aria-label="go-virtual logo"
        >
            <defs>
                <linearGradient id="gvl-bg" x1="0" y1="0" x2="100" y2="100" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#0c1828" />
                    <stop offset="100%" stopColor="#172038" />
                </linearGradient>
                <linearGradient id="gvl-la" x1="18" y1="22" x2="50" y2="74" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#38bdf8" />
                    <stop offset="100%" stopColor="#60a5fa" />
                </linearGradient>
                <linearGradient id="gvl-ra" x1="82" y1="22" x2="50" y2="74" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#a78bfa" />
                    <stop offset="100%" stopColor="#818cf8" />
                </linearGradient>
                <linearGradient id="gvl-cn" x1="39" y1="63" x2="61" y2="85" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#38bdf8" />
                    <stop offset="100%" stopColor="#818cf8" />
                </linearGradient>
                <radialGradient id="gvl-glow" cx="50" cy="74" r="22" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stopColor="#818cf8" stopOpacity="0.35" />
                    <stop offset="100%" stopColor="#38bdf8" stopOpacity="0" />
                </radialGradient>
            </defs>

            {/* Background rounded square */}
            <rect width="100" height="100" rx="22" fill="url(#gvl-bg)" />

            {/* Dashed top line — the "direct" API route being intercepted */}
            <line x1="26" y1="22" x2="74" y2="22"
                stroke="#475569" strokeWidth="1.5" strokeDasharray="3.5,3" opacity="0.5" />

            {/* V-shape left arm — sky blue (client / request side) */}
            <line x1="18" y1="22" x2="50" y2="74"
                stroke="url(#gvl-la)" strokeWidth="6" strokeLinecap="round" />
            {/* V-shape right arm — violet (server / response side) */}
            <line x1="82" y1="22" x2="50" y2="74"
                stroke="url(#gvl-ra)" strokeWidth="6" strokeLinecap="round" />

            {/* Top-left endpoint node (client) */}
            <circle cx="18" cy="22" r="8.5" fill="#0c1828" stroke="#38bdf8" strokeWidth="2.5" />
            <circle cx="18" cy="22" r="3.2" fill="#38bdf8" />

            {/* Top-right endpoint node (server) */}
            <circle cx="82" cy="22" r="8.5" fill="#0c1828" stroke="#a78bfa" strokeWidth="2.5" />
            <circle cx="82" cy="22" r="3.2" fill="#a78bfa" />

            {/* Centre node — glow halo */}
            <circle cx="50" cy="74" r="22" fill="url(#gvl-glow)" />
            {/* Centre node — the virtual proxy nexus */}
            <circle cx="50" cy="74" r="11.5" fill="url(#gvl-cn)" />
            {/* Three dots: active-processing indicator */}
            <circle cx="43.5" cy="74" r="1.8" fill="white" opacity="0.7" />
            <circle cx="50"   cy="74" r="1.8" fill="white" opacity="0.95" />
            <circle cx="56.5" cy="74" r="1.8" fill="white" opacity="0.7" />
        </svg>
    )
}

interface LogoFullProps {
    /** Icon size in pixels. Default: 36 */
    iconSize?: number
    className?: string
}

/** Icon + "go-virtual" wordmark, horizontally laid out. */
export function LogoFull({ iconSize = 36, className }: LogoFullProps) {
    return (
        <div className={`flex items-center gap-2.5 ${className ?? ''}`}>
            <LogoIcon size={iconSize} />
            <span
                className="font-bold tracking-tight leading-none select-none"
                style={{ fontSize: iconSize * 0.55 }}
            >
                <span className="text-sky-400">go</span>
                <span className="text-slate-400">-</span>
                <span className="text-indigo-400">virtual</span>
            </span>
        </div>
    )
}
