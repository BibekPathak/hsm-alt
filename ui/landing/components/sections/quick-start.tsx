'use client'

import { SectionTitle } from '@/components/section-title'

const steps = [
  {
    cmd: 'docker compose -f configs/docker/docker-compose.yml up -d',
    label: 'Start the cluster',
  },
  {
    cmd: "curl -X POST http://localhost:8080/wallet/create \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"name\":\"My Treasury\"}'",
    label: 'Create a wallet',
  },
  {
    cmd: "curl -X POST http://localhost:8080/intent \\\n  -H 'Content-Type: application/json' \\\n  -d '{\n    \"wallet_id\": \"<wallet_id>\",\n    \"chain\": \"solana\",\n    \"to\": \"<recipient>\",\n    \"value\": \"0.01\"\n  }'",
    label: 'Create an intent',
  },
  {
    cmd: 'curl -X POST http://localhost:8080/intent/<id>/approve \\\n  -H \'Content-Type: application/json\' \\\n  -d \'{"approver":"admin"}\'',
    label: 'Approve the intent',
  },
  {
    cmd: 'curl -X POST http://localhost:8080/intent/<id>/execute',
    label: 'Execute & broadcast',
  },
]

export function QuickStart() {
  return (
    <section id="quick-start" className="bg-gray-50 px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Quick Start"
          title="Go from zero to first transaction in 5 minutes."
          description="No SDKs to install. No dependencies. Just Docker and curl."
        />

        <div className="mt-12 grid gap-4">
          {steps.map((step, i) => (
            <div
              key={i}
              className="overflow-hidden rounded-2xl border border-gray-200 bg-white"
            >
              <div className="flex items-center gap-3 border-b border-gray-100 bg-gray-50 px-5 py-3">
                <span className="flex h-6 w-6 items-center justify-center rounded-full bg-gray-200 text-xs font-medium text-gray-700">
                  {i + 1}
                </span>
                <span className="text-sm font-medium text-gray-700">
                  {step.label}
                </span>
              </div>
              <pre className="overflow-x-auto p-5 text-sm text-gray-900">
                <code>{step.cmd}</code>
              </pre>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
