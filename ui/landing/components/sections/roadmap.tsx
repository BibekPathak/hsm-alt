'use client'

import { SectionTitle } from '@/components/section-title'

const milestones = [
  { year: 'Q1 2025', title: 'Wallet Service', description: 'Multi-chain wallet CRUD and encrypted key storage.' },
  { year: 'Q1 2025', title: 'Ethereum Integration', description: 'Native ETH and ERC-20 transfers with EIP-1559 fee estimation.' },
  { year: 'Q1 2025', title: 'Solana Integration', description: 'Native SOL and SPL token transfers.' },
  { year: 'Q2 2025', title: 'Intent Engine', description: 'Declarative transaction intents with approval workflows.' },
  { year: 'Q2 2025', title: 'ZK Policy Engine', description: 'Groth16-based zero-knowledge policy verification gate.' },
  { year: 'Q2 2025', title: 'MPC Infrastructure', description: '2-of-3 FROST-Ed25519 signing with DKG, encrypted share storage.' },
  { year: 'Q3 2025', title: 'TEE Integration', description: 'Hardware-backed attestation for enclave operations.' },
  { year: 'Q3 2025', title: 'Threshold ECDSA', description: 'Ethereum threshold signing via FROST-secp256k1.' },
]

export function Roadmap() {
  return (
    <section id="roadmap" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Roadmap"
          title="What we've built and what's coming."
        />

        <div className="mt-16">
          {milestones.map((m, index) => (
            <div key={m.title} className="relative flex gap-6 pb-8 last:pb-0">
              <div className="flex flex-col items-center">
                <div className="z-10 flex h-8 w-8 items-center justify-center rounded-full border-2 border-gray-200 bg-white">
                  <div className="h-2 w-2 rounded-full bg-gray-300" />
                </div>
                {index < milestones.length - 1 && (
                  <div className="h-full w-px bg-gray-200" />
                )}
              </div>
              <div className="flex-1 pb-4">
                <span className="text-sm font-medium text-gray-500">
                  {m.year}
                </span>
                <h3 className="mt-1 text-base font-semibold">{m.title}</h3>
                <p className="mt-1 text-sm leading-relaxed text-gray-600">
                  {m.description}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
