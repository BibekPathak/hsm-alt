'use client'

import { SectionTitle } from '@/components/section-title'

const steps = [
  {
    number: '01',
    title: 'Create Intent',
    description:
      'Define the transaction: chain, amount, recipient, token type, and gas parameters.',
  },
  {
    number: '02',
    title: 'Approval Workflow',
    description:
      'Configured approvers review and approve the intent. Multi-signature support built in.',
  },
  {
    number: '03',
    title: 'ZK Policy Verification',
    description:
      'Prove compliance off-chain with zero-knowledge proofs. Transfer limits, allowlists, and more.',
  },
  {
    number: '04',
    title: 'Threshold Signing',
    description:
      '2-of-3 MPC signing across distributed enclaves. The private key is never reconstructed.',
  },
  {
    number: '05',
    title: 'Broadcast & Confirm',
    description:
      'Transaction is broadcast to the blockchain and confirmed. Full audit trail logged.',
  },
]

export function HowItWorks() {
  return (
    <section id="how-it-works" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="How It Works"
          title="From intent to on-chain settlement."
          description="A fully orchestrated pipeline that combines approval workflows, zero-knowledge proofs, and threshold cryptography."
        />

        <div className="mt-16 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          {steps.map((step, index) => (
            <div key={step.number} className="relative">
              <div className="rounded-2xl border border-gray-200 bg-white p-6 transition-shadow hover:shadow-card-hover">
                <span className="text-4xl font-bold tracking-tight text-gray-200">
                  {step.number}
                </span>
                <h3 className="mt-3 text-base font-semibold">{step.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-gray-600">
                  {step.description}
                </p>
              </div>
              {index < steps.length - 1 && (
                <div className="hidden h-px w-full bg-gray-200 lg:block" />
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
