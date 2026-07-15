'use client'

import { Key, ShieldOff, ClipboardX, Building2 } from 'lucide-react'
import { SectionTitle } from '@/components/section-title'

const problems = [
  {
    icon: Key,
    title: 'Private Keys',
    description: 'Single point of failure. Lose one key, lose everything.',
  },
  {
    icon: ShieldOff,
    title: 'Approvals',
    description: 'Manual multi-sig processes that slow down every transaction.',
  },
  {
    icon: ClipboardX,
    title: 'Compliance',
    description: 'Hard-coded rules that require redeployment to change.',
  },
  {
    icon: Building2,
    title: 'Treasury Ops',
    description: 'No orchestration layer for cross-chain treasury management.',
  },
]

export function Problem() {
  return (
    <section id="problem" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="The Problem"
          title="Traditional custody wasn't built for multi-chain operations."
          description="Legacy key management breaks when you need to move fast across multiple chains with compliance."
        />

        <div className="mt-16 grid gap-4 sm:grid-cols-2 lg:gap-6">
          {problems.map((problem) => {
            const Icon = problem.icon
            return (
              <div
                key={problem.title}
                className="rounded-2xl border border-gray-200 bg-white p-8 transition-shadow hover:shadow-card-hover"
              >
                <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-gray-100">
                  <Icon className="h-5 w-5 text-gray-700" />
                </div>
                <h3 className="text-lg font-semibold">{problem.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-gray-600">
                  {problem.description}
                </p>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
