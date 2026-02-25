import { createRoot } from 'react-dom/client'
import { loader } from '@monaco-editor/react'
import * as monaco from 'monaco-editor'
import App from './App.tsx'
import './index.css'

// Use local Monaco Editor instead of CDN (required by CSP)
loader.config({ monaco })

createRoot(document.getElementById('root')!).render(<App />)
