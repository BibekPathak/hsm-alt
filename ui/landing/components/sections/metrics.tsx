'use client'

const metrics = [
  { value: '3', label: 'MPC Nodes' },
  { value: '2-of-3', label: 'Threshold' },
  { value: '2', label: 'Supported Chains' },
  { value: '21', label: 'REST APIs' },
  { value: '12k+', label: 'Lines of Code' },
  { value: '24', label: 'Go Packages' },
]

export function Metrics() {
  return (
    <section className="border-y border-gray-100 px-6 py-12">
      <div className="mx-auto max-w-page">
        <div className="grid grid-cols-2 gap-8 sm:grid-cols-3 lg:grid-cols-6">
          {metrics.map((m) => (
            <div key={m.label} className="text-center">
              <div className="text-3xl font-bold tracking-tight sm:text-4xl">
                {m.value}
              </div>
              <div className="mt-1 text-sm text-gray-500">{m.label}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
