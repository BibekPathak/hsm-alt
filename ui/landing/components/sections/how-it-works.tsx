'use client'

import { motion } from 'framer-motion'
import { SectionTitle } from '@/components/section-title'

const stages = [
  { title: 'Create Intent', label: 'Define chain, amount, token, and gas' },
  { title: 'Wallet Service', label: 'Route, validate, and load the account' },
  { title: 'Policy Engine', label: 'Evaluate ZK policies and generate proof' },
  { title: 'Proof Verification', label: 'Groth16 verification gates execution' },
  { title: 'Signer Resolver', label: 'Select direct or MPC signer' },
  { title: 'MPC Coordinator', label: 'Orchestrate 2-round threshold signing' },
]

const nodes = ['Node 1', 'Node 2', 'Node 3']

export function HowItWorks() {
  return (
    <section id="how-it-works" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="How It Works"
          title="From intent to on-chain settlement."
          description="A fully orchestrated pipeline combining approval workflows, ZK proofs, and threshold cryptography."
        />

        <div className="relative mt-16 flex flex-col items-center">
          {stages.map((stage, i) => (
            <div key={stage.title} className="flex w-full flex-col items-center">
              <motion.div
                initial={{ opacity: 0, y: 20 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1 }}
                className="w-full max-w-lg rounded-2xl border border-gray-200 bg-white p-5 text-center shadow-sm"
              >
                <span className="text-xs font-medium text-gray-400">
                  Step {String(i + 1).padStart(2, '0')}
                </span>
                <h3 className="mt-1 text-base font-semibold">{stage.title}</h3>
                <p className="mt-0.5 text-sm text-gray-500">{stage.label}</p>
              </motion.div>

              <motion.div
                initial={{ scaleY: 0 }}
                whileInView={{ scaleY: 1 }}
                viewport={{ once: true }}
                transition={{ delay: i * 0.1 + 0.2, duration: 0.4 }}
                className="h-8 w-px origin-top bg-gray-300"
              />
            </div>
          ))}

          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: stages.length * 0.1 }}
            className="w-full max-w-lg rounded-2xl border border-gray-200 bg-gray-50 p-5 text-center shadow-sm"
          >
            <span className="text-xs font-medium text-gray-400">
              Broadcast
            </span>
            <div className="mt-3 flex items-center justify-center gap-3">
              {nodes.map((n) => (
                <span
                  key={n}
                  className="rounded-xl border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700"
                >
                  {n}
                </span>
              ))}
            </div>
            <p className="mt-2 text-sm text-gray-500">
              2-of-3 threshold signature → broadcast to blockchain
            </p>
          </motion.div>
        </div>
      </div>
    </section>
  )
}
