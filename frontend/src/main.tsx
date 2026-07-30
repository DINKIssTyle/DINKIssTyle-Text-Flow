import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'

const mode = new URLSearchParams(window.location.search).get('mode')
if (mode === 'capture') {
    document.documentElement.classList.add('capture-mode')
} else if (mode === 'floating') {
    document.documentElement.classList.add('floating-capture-mode')
}

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
