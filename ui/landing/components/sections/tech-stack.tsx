'use client'

import { SectionTitle } from '@/components/section-title'

const techs = [
  { name: 'Go', color: 'bg-cyan-50 text-cyan-700 border-cyan-200' },
  { name: 'Rust', color: 'bg-orange-50 text-orange-700 border-orange-200' },
  { name: 'Ethereum', color: 'bg-blue-50 text-blue-700 border-blue-200' },
  { name: 'Solana', color: 'bg-purple-50 text-purple-700 border-purple-200' },
  { name: 'Circom', color: 'bg-amber-50 text-amber-700 border-amber-200' },
  { name: 'Docker', color: 'bg-sky-50 text-sky-700 border-sky-200' },
  { name: 'Kubernetes', color: 'bg-indigo-50 text-indigo-700 border-indigo-200' },
]

export function TechStack() {
  return (
    <section className="border-y border-gray-100 px-6 py-16">
      <div className="mx-auto max-w-page">
        <SectionTitle
          align="center"
          label="Built With"
          title=""
        />

        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          {techs.map((tech) => (
            <span
              key={tech.name}
              className={`inline-flex items-center rounded-xl border px-4 py-2 text-sm font-medium ${tech.color}`}
            >
              {tech.name}
            </span>
          ))}
        </div>
      </div>
    </section>
  )
}
