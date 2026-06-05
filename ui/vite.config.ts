import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [
        react(),
        VitePWA({
            registerType: 'autoUpdate',
            scope: '/_ui/',
            base: '/_ui/',
            includeAssets: ['favicon.svg', 'icon-192.svg', 'icon-512.svg'],
            manifest: {
                name: 'go-virtual',
                short_name: 'go-virtual',
                description: 'API mock and virtualisation server',
                start_url: '/_ui/',
                scope: '/_ui/',
                display: 'standalone',
                background_color: '#ffffff',
                theme_color: '#4f46e5',
                icons: [
                    {
                        src: '/_ui/icon-192.svg',
                        sizes: '192x192',
                        type: 'image/svg+xml',
                        purpose: 'any',
                    },
                    {
                        src: '/_ui/icon-512.svg',
                        sizes: '512x512',
                        type: 'image/svg+xml',
                        purpose: 'any maskable',
                    },
                ],
            },
            workbox: {
                // Precache all build assets (fingerprinted, safe to cache forever)
                globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
                navigateFallback: '/_ui/index.html',
                navigateFallbackDenylist: [
                    // Never intercept API calls
                    /^\/_api\//,
                ],
                runtimeCaching: [
                    {
                        // API calls — always network, never cache
                        urlPattern: /^\/_api\//,
                        handler: 'NetworkOnly',
                    },
                    {
                        // App shell — network first so deployments propagate quickly
                        urlPattern: /^\/_ui\/$/,
                        handler: 'NetworkFirst',
                        options: {
                            cacheName: 'app-shell',
                            networkTimeoutSeconds: 3,
                        },
                    },
                ],
            },
        }),
    ],
    base: '/_ui/',
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        },
    },
    server: {
        port: 5173,
        proxy: {
            '/_api': {
                target: 'http://localhost:8080',
                changeOrigin: true,
            },
        },
    },
    build: {
        outDir: 'dist',
        emptyOutDir: true,
        chunkSizeWarningLimit: 800,
        rollupOptions: {
            output: {
                manualChunks: {
                    react: ['react', 'react-dom', 'react-router-dom'],
                    tanstack: ['@tanstack/react-query'],
                    monaco: ['@monaco-editor/react'],
                    ui: ['lucide-react', '@dnd-kit/core', '@dnd-kit/sortable', '@dnd-kit/utilities'],
                },
            },
        },
    },
})
