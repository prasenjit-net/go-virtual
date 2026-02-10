import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './components/Dashboard'
import SpecList from './components/SpecManager/SpecList'
import SpecDetail from './components/SpecManager/SpecDetail'
import OperationDetail from './components/OperationDetail'
import TraceViewer from './components/TraceViewer'
import TagManager from './components/TagManager'

function App() {
    return (
        <Routes>
            <Route path="/" element={<Layout />}>
                <Route index element={<Navigate to="/dashboard" replace />} />
                <Route path="dashboard" element={<Dashboard />} />
                <Route path="specs" element={<SpecList />} />
                <Route path="specs/:specId" element={<SpecDetail />} />
                <Route path="operations/:operationId" element={<OperationDetail />} />
                <Route path="traces" element={<TraceViewer />} />
                <Route path="tags" element={<TagManager />} />
            </Route>
        </Routes>
    )
}

export default App
