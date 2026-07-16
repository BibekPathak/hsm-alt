'use client'

import { useState } from 'react'
import { SectionTitle } from '@/components/section-title'

const tabs = [
  { id: 'dashboard', label: 'Dashboard', video: '/demo-dashboard.mp4' },
  { id: 'mpc', label: 'MPC Signing', video: '/demo-mpc.mp4' },
  { id: 'zk', label: 'ZK Verification', video: '/demo-zk.mp4' },
]

export function Demo() {
  const [active, setActive] = useState('dashboard')

  return (
    <section id="demo" className="bg-gray-50 px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Demo"
          title="See it in action."
          description="Real screen captures of the platform running locally."
        />

        <div className="mt-10">
          <div className="flex items-center gap-1 rounded-xl bg-gray-100 p-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActive(tab.id)}
                className={`flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all ${
                  active === tab.id
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="mt-6 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
            {tabs.map(
              (tab) =>
                active === tab.id && (
                  <video
                    key={tab.id}
                    className="w-full"
                    autoPlay
                    muted
                    loop
                    playsInline
                    poster="/dashboard-preview.png"
                  >
                    <source src={tab.video} type="video/mp4" />
                  </video>
                )
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
