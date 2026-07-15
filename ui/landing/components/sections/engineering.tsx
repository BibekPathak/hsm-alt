'use client'

import { SectionTitle } from '@/components/section-title'

const tags = [
  '12,000+ lines',
  'Go',
  'Rust',
  'MPC',
  'FROST',
  'Groth16',
  'Ethereum',
  'Solana',
  'Docker',
  'Kubernetes',
  '3-node distributed signing',
  'Intent orchestration',
  'AES-256-GCM',
  'Argon2id',
  'gRPC',
  'REST API',
  'Next.js',
  'Circom',
]

export function Engineering() {
  return (
    <section className="bg-gray-50 px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Engineering"
          title="Built for infrastructure engineers."
          description="Full-stack, open-source, and designed for production."
        />

        <div className="mt-12 flex flex-wrap justify-center gap-2">
          {tags.map((tag) => (
            <span
              key={tag}
              className="inline-flex items-center rounded-xl border border-gray-200 bg-white px-3.5 py-1.5 text-sm text-gray-700"
            >
              {tag}
            </span>
          ))}
        </div>
      </div>
    </section>
  )
}
