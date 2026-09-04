import { Suspense, lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import PWAInstallBanner from './components/PWAInstallBanner'

// Every route below is its own lazy chunk: Vite/Rollup splits each dynamic
// import into a separate file loaded on demand, instead of bundling every
// page (Monaco-backed editors, recharts, dnd-kit-heavy lists, …) into the
// single eager entry chunk. See vite.config.ts for the vendor-library
// manualChunks that complement this.
const Dashboard = lazy(() => import('./components/Dashboard'))
const SpecList = lazy(() => import('./components/SpecManager/SpecList'))
const SpecDetail = lazy(() => import('./components/SpecManager/SpecDetail'))
const AIScenariosPage = lazy(() => import('./components/SpecManager/AIScenariosPage'))
const OperationDetail = lazy(() => import('./components/OperationDetail'))
const OperationRecordedResponsesPage = lazy(() => import('./components/OperationRecordedResponsesPage'))
const ResponseConfigPage = lazy(() => import('./components/ResponseDesigner/ResponseConfigPage'))
const TraceViewer = lazy(() => import('./components/TraceViewer'))
const TraceDetailPage = lazy(() => import('./components/Traces/TraceDetailPage'))
const TagManager = lazy(() => import('./components/TagManager'))
const ScriptList = lazy(() => import('./components/ScriptManager/ScriptList'))
const ScriptEditor = lazy(() => import('./components/ScriptManager/ScriptEditor'))
const StoreManager = lazy(() => import('./components/StoreManager/StoreManager'))
const SessionList = lazy(() => import('./components/SessionManager/SessionList'))
const SessionDetail = lazy(() => import('./components/SessionManager/SessionDetail'))
const ArchiveManager = lazy(() => import('./components/ArchiveManager/ArchiveManager'))

function RouteFallback() {
    return (
        <div className="p-8">
            <div className="animate-pulse space-y-4">
                <div className="h-8 bg-gray-200 dark:bg-slate-800 rounded w-48"></div>
                <div className="h-32 bg-gray-200 dark:bg-slate-800 rounded-xl"></div>
            </div>
        </div>
    )
}

function App() {
    return (
        <>
        <PWAInstallBanner />
        <Suspense fallback={<RouteFallback />}>
        <Routes>
            <Route path="/" element={<Layout />}>
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route path="dashboard" element={<Dashboard />} />
                <Route path="specs" element={<SpecList />} />
                <Route path="specs/:specId" element={<SpecDetail />} />
                <Route path="specs/:specId/ai-scenarios" element={<Navigate to="/ai-scenarios" replace />} />
                <Route path="ai-scenarios" element={<AIScenariosPage />} />
                <Route path="operations/:operationId" element={<OperationDetail />} />
                <Route path="operations/:operationId/recorded-responses" element={<OperationRecordedResponsesPage />} />
                <Route path="operations/:operationId/responses/new" element={<ResponseConfigPage />} />
                <Route path="responses/:responseId/edit" element={<ResponseConfigPage />} />
                <Route path="traces" element={<TraceViewer />} />
                <Route path="traces/:traceId" element={<TraceDetailPage />} />
                <Route path="tags" element={<TagManager />} />
                <Route path="scripts" element={<ScriptList />} />
                <Route path="scripts/new" element={<ScriptEditor />} />
                <Route path="scripts/:scriptId" element={<Navigate to="edit" replace />} />
                <Route path="scripts/:scriptId/edit" element={<ScriptEditor />} />
                <Route path="store" element={<StoreManager />} />
                <Route path="sessions" element={<SessionList />} />
                <Route path="sessions/:sessionId" element={<SessionDetail />} />
                <Route path="archives" element={<ArchiveManager />} />
            </Route>
        </Routes>
        </Suspense>
        </>
    )
}

export default App
