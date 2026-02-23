import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './components/Dashboard'
import SpecList from './components/SpecManager/SpecList'
import SpecDetail from './components/SpecManager/SpecDetail'
import OperationDetail from './components/OperationDetail'
import ResponseConfigPage from './components/ResponseDesigner/ResponseConfigPage'
import TraceViewer from './components/TraceViewer'
import TagManager from './components/TagManager'
import ScriptList from './components/ScriptManager/ScriptList'
import ScriptEditor from './components/ScriptManager/ScriptEditor'

function App() {
    return (
        <Routes>
            <Route path="/" element={<Layout />}>
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route path="dashboard" element={<Dashboard />} />
                <Route path="specs" element={<SpecList />} />
                <Route path="specs/:specId" element={<SpecDetail />} />
                <Route path="operations/:operationId" element={<OperationDetail />} />
                <Route path="operations/:operationId/responses/new" element={<ResponseConfigPage />} />
                <Route path="responses/:responseId/edit" element={<ResponseConfigPage />} />
                <Route path="traces" element={<TraceViewer />} />
                <Route path="tags" element={<TagManager />} />
                <Route path="scripts" element={<ScriptList />} />
                <Route path="scripts/new" element={<ScriptEditor />} />
                <Route path="scripts/:scriptId/edit" element={<ScriptEditor />} />
            </Route>
        </Routes>
    )
}

export default App

