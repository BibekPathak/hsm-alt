'use client'

import { ArrowRight, Github, BookOpen } from 'lucide-react'

export function CTA() {
  return (
    <section className="px-6 py-24 lg:py-32">
      <div className="mx-auto max-w-page text-center">
        <h2 className="text-3xl font-bold tracking-tight sm:text-4xl lg:text-5xl">
          Ready to build secure
          <br />
          treasury infrastructure?
        </h2>
        <p className="mx-auto mt-4 max-w-lg text-base leading-relaxed text-gray-600">
          Open-source, modular, and designed for institutional custody.
        </p>

        <div className="mt-8 flex items-center justify-center gap-4">
          <a
            href="#"
            className="inline-flex items-center gap-2 rounded-2xl bg-black px-6 py-3 text-sm font-medium text-white transition-opacity hover:opacity-90"
          >
            Book Demo
            <ArrowRight className="h-4 w-4" />
          </a>
          <a
            href="https://github.com/BibekPathak/hsm-alt"
            className="inline-flex items-center gap-2 rounded-2xl border border-gray-200 bg-white px-6 py-3 text-sm font-medium text-gray-900 transition-colors hover:bg-gray-50"
          >
            <Github className="h-4 w-4" />
            GitHub
          </a>
          <a
            href="#"
            className="inline-flex items-center gap-2 rounded-2xl border border-gray-200 bg-white px-6 py-3 text-sm font-medium text-gray-900 transition-colors hover:bg-gray-50"
          >
            <BookOpen className="h-4 w-4" />
            Documentation
          </a>
        </div>
      </div>
    </section>
  )
}
