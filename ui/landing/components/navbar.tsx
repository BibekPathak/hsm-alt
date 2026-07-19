'use client'

import { useState } from 'react'
import Link from 'next/link'
import { LayoutDashboard, Menu, X } from 'lucide-react'
import { cn } from '@/lib/utils'

const DASHBOARD_URL = process.env.NEXT_PUBLIC_DASHBOARD_URL || 'http://localhost:3001'

const navLinks = [
  { href: '#features', label: 'Features' },
  { href: '#how-it-works', label: 'How It Works' },
  { href: '#architecture', label: 'Architecture' },
  { href: '#demo', label: 'Demo' },
  { href: '#security', label: 'Security' },
  { href: '#roadmap', label: 'Roadmap' },
]

export function Navbar() {
  const [mobileOpen, setMobileOpen] = useState(false)

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-100">
      <nav className="mx-auto flex max-w-page items-center justify-between px-6 py-4 lg:px-8">
        <Link href="/" className="flex items-center gap-2">
          <div className="h-7 w-7 rounded-lg bg-black" />
          <span className="text-sm font-semibold tracking-tight">HSM</span>
        </Link>

          <div className="hidden items-center gap-8 md:flex">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-sm text-gray-600 transition-colors hover:text-black"
              >
                {link.label}
              </Link>
            ))}
            <a
              href={DASHBOARD_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-sm text-gray-600 transition-colors hover:text-black"
            >
              <LayoutDashboard className="h-3.5 w-3.5" />
              Dashboard
            </a>
            <Link
              href="#demo"
              className="rounded-xl bg-black px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90"
            >
              Live Demo
            </Link>
          </div>

        <button
          className="md:hidden"
          onClick={() => setMobileOpen(!mobileOpen)}
          aria-label="Toggle menu"
        >
          {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      </nav>

      {mobileOpen && (
        <div className="border-t border-gray-100 bg-white px-6 py-4 md:hidden">
          <div className="flex flex-col gap-4">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="text-sm text-gray-600 transition-colors hover:text-black"
                onClick={() => setMobileOpen(false)}
              >
                {link.label}
              </Link>
            ))}
            <a
              href={DASHBOARD_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center justify-center gap-1.5 rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-900 transition-colors hover:bg-gray-50"
              onClick={() => setMobileOpen(false)}
            >
              <LayoutDashboard className="h-3.5 w-3.5" />
              Dashboard
            </a>
            <Link
              href="#demo"
              className="inline-flex items-center justify-center rounded-xl bg-black px-4 py-2 text-sm font-medium text-white"
              onClick={() => setMobileOpen(false)}
            >
              Live Demo
            </Link>
          </div>
        </div>
      )}
    </header>
  )
}
