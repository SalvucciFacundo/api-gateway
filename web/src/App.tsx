import { Navigate, Route, Routes } from 'react-router-dom'
import { tokens } from './api'
import { ExplorerPage } from './pages/ExplorerPage'
import { LoginPage } from './pages/LoginPage'
import { RegisterPage } from './pages/RegisterPage'

function Protected() { return tokens.get().access ? <ExplorerPage /> : <Navigate to="/" replace /> }
export function App() { return <Routes><Route path="/" element={<LoginPage />} /><Route path="/register" element={<RegisterPage />} /><Route path="/explorer" element={<Protected />} /><Route path="*" element={<Navigate to={tokens.get().access ? '/explorer' : '/'} replace />} /></Routes> }
