'use client'

import { SectionTitle } from '@/components/section-title'

const rows = [
  {
    threat: 'Single node failure',
    mitigation:
      '2-of-3 threshold signatures allow signing to continue while one node is unavailable.',
  },
  {
    threat: 'Node compromise',
    mitigation:
      'A single key share cannot reconstruct the private key. The signature threshold prevents any single node from signing.',
  },
  {
    threat: 'Malicious partial signature',
    mitigation:
      'FROST validates commitments before aggregation. Invalid shares are rejected before the final signature is produced.',
  },
  {
    threat: 'Replay attack',
    mitigation:
      'Intent IDs, nonces, and chain-specific transaction hashes prevent replay across chains or sessions.',
  },
  {
    threat: 'RPC outage',
    mitigation:
      'Configurable RPC providers with exponential backoff and retry logic ensure availability.',
  },
  {
    threat: 'Expired Solana blockhash',
    mitigation:
      'Automatic blockhash refresh and transaction rebuild within the intent lifecycle.',
  },
  {
    threat: 'Double execution',
    mitigation:
      'Intent state machine and per-wallet execution locks provide idempotency guarantees.',
  },
  {
    threat: 'Policy bypass',
    mitigation:
      'Groth16 proof verification is a mandatory gate before any transaction execution.',
  },
  {
    threat: 'Database compromise',
    mitigation:
      'Private keys are never stored. Only encrypted metadata and public keys exist in the database.',
  },
  {
    threat: 'Wallet theft',
    mitigation:
      'Key material remains distributed across MPC nodes. No single system holds a complete private key.',
  },
]

export function ThreatModel() {
  return (
    <section id="threat-model" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Threat Model"
          title="Designed for adversarial environments."
          description="Every component is evaluated against realistic failure scenarios."
        />

        <div className="mt-12 overflow-hidden rounded-2xl border border-gray-200">
          {rows.map((row, i) => (
            <div
              key={row.threat}
              className={`grid gap-4 px-6 py-4 sm:grid-cols-2 sm:px-8 ${
                i % 2 === 0 ? 'bg-white' : 'bg-gray-50'
              }`}
            >
              <div>
                <span className="text-sm font-semibold text-gray-900">
                  {row.threat}
                </span>
              </div>
              <div>
                <span className="text-sm leading-relaxed text-gray-600">
                  {row.mitigation}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
