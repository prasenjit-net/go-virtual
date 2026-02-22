/**
 * go-virtual Logo components
 *
 * Concept: "OpenAPI Shadow Mock"
 *  • Main document  = the real OpenAPI spec (solid indigo outline + content lines)
 *  • Shadow document = the virtual / mock copy (dashed violet, offset right+down)
 *  • Check badge    = test passing / mock validated (emerald green)
 *
 * Transparent background → works on both light and dark sidebars / backgrounds.
 *
 * LogoIcon  – square SVG icon only
 * LogoFull  – icon + configurable title wordmark (falls back to "go-virtual")
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
            {/*
              Shadow document — the virtual/mock copy.
              Offset right+down, dashed violet stroke, transparent fill.
              indigo-500 at 65% opacity is readable on both white and dark bg.
            */}
            <rect x="28" y="22" width="50" height="64" rx="10"
                fill="#818cf8" fillOpacity="0.10"
                stroke="#6366f1" strokeWidth="2"
                strokeDasharray="5,3.5" strokeOpacity="0.65" />

            {/*
              Main document — the OpenAPI spec.
              indigo-600 (#4f46e5) has 4.9:1 contrast ratio on white → AA compliant.
              Vivid enough to read well on dark backgrounds too.
            */}
            <rect x="12" y="8" width="52" height="66" rx="10"
                fill="#4f46e5" fillOpacity="0.07"
                stroke="#4f46e5" strokeWidth="2.5" />

            {/* Header / title bar — rounded top corners only */}
            <path d="M 12 22 L 12 18 Q 12 8 22 8 L 54 8 Q 64 8 64 18 L 64 22 Z"
                fill="#4f46e5" fillOpacity="0.18" />

            {/* ── OpenAPI spec content lines ── */}
            {/* operationId / title line (bolder) — indigo-700 for max contrast on light */}
            <line x1="21" y1="34" x2="55" y2="34"
                stroke="#4338ca" strokeWidth="2.8" strokeLinecap="round" />
            {/* schema / property lines */}
            <line x1="21" y1="43" x2="50" y2="43"
                stroke="#4f46e5" strokeWidth="2" strokeLinecap="round" opacity="0.65" />
            <line x1="21" y1="51" x2="54" y2="51"
                stroke="#4f46e5" strokeWidth="2" strokeLinecap="round" opacity="0.65" />
            <line x1="21" y1="59" x2="40" y2="59"
                stroke="#4f46e5" strokeWidth="2" strokeLinecap="round" opacity="0.45" />

            {/*
              Test / validation badge — overlapping bottom-right corner.
              emerald-600 (#059669) reads clearly on both light and dark backgrounds.
            */}
            <circle cx="72" cy="78" r="14" fill="#059669" />
            <circle cx="72" cy="78" r="14" fill="white" fillOpacity="0.10" />
            {/* Checkmark */}
            <polyline points="64.5,78 70.5,84.5 80.5,71"
                fill="none" stroke="white" strokeWidth="3"
                strokeLinecap="round" strokeLinejoin="round" />
        </svg>
    )
}

interface LogoFullProps {
    /** Icon size in pixels. Default: 36 */
    iconSize?: number
    /**
     * Application title from branding config.
     * When "go-virtual" (or empty) the title renders with its two-tone colour scheme.
     * A custom title renders in a single adaptive indigo colour.
     */
    title?: string
    className?: string
}

/** Icon + configurable title wordmark, horizontally laid out. */
export function LogoFull({ iconSize = 36, title, className }: LogoFullProps) {
    const displayTitle = title?.trim() || 'go-virtual'
    const isDefault = displayTitle === 'go-virtual'

    return (
        <div className={`flex items-center gap-2.5 ${className ?? ''}`}>
            <LogoIcon size={iconSize} />
            <span
                className="font-bold tracking-tight leading-none select-none"
                style={{ fontSize: iconSize * 0.53 }}
            >
                {isDefault ? (
                    <>
                        {/*
                          Two-tone wordmark: darker shades on light, lighter on dark.
                          indigo-700 / violet-700 → 6:1+ on white ✓
                          indigo-400 / violet-400 → vivid on dark ✓
                        */}
                        <span className="text-indigo-700 dark:text-indigo-400">go</span>
                        <span className="text-slate-400 dark:text-slate-500">-</span>
                        <span className="text-violet-700 dark:text-violet-400">virtual</span>
                    </>
                ) : (
                    <span className="text-indigo-700 dark:text-indigo-300">{displayTitle}</span>
                )}
            </span>
        </div>
    )
}
