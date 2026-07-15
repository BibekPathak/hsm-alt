'use client'

import { SectionTitle } from '@/components/section-title'

const specs = [
  { label: 'Private Keys', value: 'Never reconstructed' },
  { label: 'Threshold Signing', value: '2-of-3' },
  { label: 'Share Encryption', value: 'AES-256-GCM' },
  { label: 'Key Derivation', value: 'Argon2id' },
  { label: 'Policy Engine', value: 'Groth16 ZK' },
  { label: 'Cryptography', value: 'FROST-Ed25519' },
  { label: 'Replay Protection', value: 'Enabled' },
  { label: 'Attestation', value: 'Simulation (TEE-ready)' },
]

export function Security() {
  return (
    <section id="security" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Security"
          title="Designed for institutional-grade custody."
          description="Every layer is built with security as the default, not an afterthought."
        />

        <div className="mt-16 overflow-hidden rounded-2xl border border-gray-200">
          {specs.map((spec, index) => (
            <div
              key={spec.label}
              className={`flex items-center justify-between px-6 py-4 sm:px-8 ${
                index % 2 === 0 ? 'bg-white' : 'bg-gray-50'
              }`}
            >
              <span className="text-sm font-medium text-gray-900">
                {spec.label}
              </span>
              <span className="text-sm text-gray-600">{spec.value}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
