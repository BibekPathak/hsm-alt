'use client'

import { SectionTitle } from '@/components/section-title'

const past = [
  'Wallet Service',
  'Ethereum Integration',
  'Solana Integration',
  'Intent Engine',
  'ZK Policy Engine',
  'MPC Infrastructure',
]

const next = [
  'TEE Integration',
  'Threshold ECDSA',
  'Hardware HSM',
  'Policy Framework',
  'Audit API',
  'FROST-secp256k1',
]

export function Roadmap() {
  return (
    <section id="roadmap" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Roadmap"
          title="Past and future."
          description="What we've shipped and what's coming next."
        />

        <div className="mt-12 grid gap-8 lg:grid-cols-2">
          <div>
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-emerald-600">
              Past
            </h3>
            <div className="space-y-2">
              {past.map((item) => (
                <div
                  key={item}
                  className="flex items-center gap-3 rounded-2xl border border-gray-200 bg-white px-5 py-3 text-sm text-gray-900"
                >
                  <span className="flex h-5 w-5 items-center justify-center rounded-full bg-emerald-50 text-xs text-emerald-600">
                    ✓
                  </span>
                  {item}
                </div>
              ))}
            </div>
          </div>

          <div>
            <h3 className="mb-4 text-sm font-semibold uppercase tracking-wider text-gray-400">
              Next
            </h3>
            <div className="space-y-2">
              {next.map((item) => (
                <div
                  key={item}
                  className="flex items-center gap-3 rounded-2xl border border-dashed border-gray-200 bg-white px-5 py-3 text-sm text-gray-500"
                >
                  <span className="flex h-5 w-5 items-center justify-center rounded-full bg-gray-100 text-xs text-gray-400">
                    +
                  </span>
                  {item}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
