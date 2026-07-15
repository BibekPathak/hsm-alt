'use client'

import {
  FileText,
  CheckCircle,
  Coins,
  Globe,
  Shield,
  Eye,
  History,
  LayoutDashboard,
  Container,
  Box,
} from 'lucide-react'
import { SectionTitle } from '@/components/section-title'

const features = [
  {
    icon: FileText,
    title: 'Intent Engine',
    description: 'Declarative transaction definitions with retry logic and expiration.',
  },
  {
    icon: CheckCircle,
    title: 'Approval Workflow',
    description: 'Multi-signer approval flows with configurable thresholds.',
  },
  {
    icon: Coins,
    title: 'Ethereum',
    description: 'EIP-1559 transactions and ERC-20 token transfers on mainnet and testnets.',
  },
  {
    icon: Globe,
    title: 'Solana',
    description: 'Native SOL and SPL token transfers with associated token accounts.',
  },
  {
    icon: Shield,
    title: 'MPC Signing',
    description: '2-of-3 threshold signatures using FROST-Ed25519. Keys never reconstructed.',
  },
  {
    icon: Eye,
    title: 'ZK Compliance',
    description: 'Groth16 proofs verify policy compliance without revealing sensitive data.',
  },
  {
    icon: History,
    title: 'Audit Trail',
    description: 'Every intent, approval, and transaction logged with timestamps.',
  },
  {
    icon: LayoutDashboard,
    title: 'Dashboard',
    description: 'Real-time treasury overview with wallet balances and intent tracking.',
  },
  {
    icon: Container,
    title: 'Docker',
    description: 'Containerized deployment with docker-compose for local clusters.',
  },
  {
    icon: Box,
    title: 'Kubernetes',
    description: 'Production-grade deployment with StatefulSets and service discovery.',
  },
]

export function Features() {
  return (
    <section id="features" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          title="Everything you need to run a treasury."
          description="Modular components that compose into a complete custody platform."
        />

        <div className="mt-16 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((feature) => {
            const Icon = feature.icon
            return (
              <div
                key={feature.title}
                className="rounded-2xl border border-gray-200 bg-white p-6 transition-shadow hover:shadow-card-hover"
              >
                <div className="mb-3 flex h-9 w-9 items-center justify-center rounded-xl bg-gray-100">
                  <Icon className="h-4 w-4 text-gray-700" />
                </div>
                <h3 className="text-sm font-semibold">{feature.title}</h3>
                <p className="mt-1.5 text-sm leading-relaxed text-gray-600">
                  {feature.description}
                </p>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
