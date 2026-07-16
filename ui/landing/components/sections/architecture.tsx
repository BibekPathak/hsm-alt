'use client'

import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { SectionTitle } from '@/components/section-title'

interface Node {
  id: string
  x: number
  y: number
  w: number
  h: number
  label: string
  sub?: string
  edges: string[]
  details: { label: string; value: string }[]
}

const nodes: Node[] = [
  {
    id: 'ui',
    x: 400,
    y: 10,
    w: 160,
    h: 44,
    label: 'UI / CLI',
    edges: ['wallet'],
    details: [
      { label: 'Interface', value: 'Next.js 14 / Go CLI' },
      { label: 'Protocol', value: 'REST API (localhost:8080)' },
    ],
  },
  {
    id: 'wallet',
    x: 370,
    y: 84,
    w: 220,
    h: 44,
    label: 'Wallet Service',
    sub: 'cmd/wallet-service',
    edges: ['policy'],
    details: [
      { label: 'Purpose', value: 'API server for wallet CRUD, intents, signing' },
      { label: 'Package', value: 'cmd/wallet-service' },
      { label: 'Port', value: '8080' },
      { label: 'Stack', value: 'Go, Chi router, gRPC client' },
    ],
  },
  {
    id: 'policy',
    x: 355,
    y: 158,
    w: 250,
    h: 44,
    label: 'Policy Engine',
    sub: 'pkg/zk/policy',
    edges: ['signer'],
    details: [
      { label: 'Purpose', value: 'ZK policy evaluation & proof generation' },
      { label: 'Package', value: 'pkg/zk/policy' },
      { label: 'Latency', value: '~10–50ms verification' },
      { label: 'Proof', value: 'Groth16 on BN128' },
    ],
  },
  {
    id: 'signer',
    x: 355,
    y: 232,
    w: 250,
    h: 44,
    label: 'Signer Resolver',
    sub: 'pkg/signer',
    edges: ['direct', 'mpc-coord'],
    details: [
      { label: 'Purpose', value: 'Route to direct or MPC signer based on account type' },
      { label: 'Interface', value: 'Signer / SolanaTxSigner' },
      { label: 'Types', value: 'ECDSASigner, SolanaSigner, MPCSolanaSigner' },
    ],
  },
  {
    id: 'direct',
    x: 130,
    y: 318,
    w: 180,
    h: 40,
    label: 'Direct Signer',
    sub: 'Ed25519 / ECDSA',
    edges: ['chains'],
    details: [
      { label: 'Algorithm', value: 'Ed25519 for Solana, secp256k1 for Ethereum' },
      { label: 'Key storage', value: 'Encrypted with AES-256-GCM' },
      { label: 'Use case', value: 'Single-party signing (non-MPC accounts)' },
    ],
  },
  {
    id: 'mpc-coord',
    x: 650,
    y: 318,
    w: 180,
    h: 40,
    label: 'MPC Coordinator',
    sub: 'pkg/mpc/node',
    edges: ['node1', 'node2', 'node3'],
    details: [
      { label: 'Purpose', value: 'Coordinate 2-round threshold signing' },
      { label: 'Package', value: 'pkg/mpc/node' },
      { label: 'Protocol', value: 'gRPC between nodes' },
      { label: 'Rounds', value: 'Round 1 (commitments) → Round 2 (partial sigs) → Aggregate' },
    ],
  },
  {
    id: 'node1',
    x: 250,
    y: 404,
    w: 120,
    h: 40,
    label: 'Node 1',
    sub: '8001',
    edges: ['enclave1'],
    details: [
      { label: 'Role', value: 'MPC participant' },
      { label: 'Service', value: 'gRPC server' },
      { label: 'Port', value: '8001' },
      { label: 'State', value: 'Handshake, DKG, Sign, Heartbeat' },
    ],
  },
  {
    id: 'node2',
    x: 400,
    y: 404,
    w: 120,
    h: 40,
    label: 'Node 2',
    sub: '8002',
    edges: ['enclave2'],
    details: [
      { label: 'Role', value: 'MPC participant' },
      { label: 'Service', value: 'gRPC server' },
      { label: 'Port', value: '8002' },
      { label: 'State', value: 'Handshake, DKG, Sign, Heartbeat' },
    ],
  },
  {
    id: 'node3',
    x: 550,
    y: 404,
    w: 120,
    h: 40,
    label: 'Node 3',
    sub: '8003',
    edges: ['enclave3'],
    details: [
      { label: 'Role', value: 'MPC participant' },
      { label: 'Service', value: 'gRPC server' },
      { label: 'Port', value: '8003' },
      { label: 'State', value: 'Handshake, DKG, Sign, Heartbeat' },
    ],
  },
  {
    id: 'enclave1',
    x: 250,
    y: 488,
    w: 120,
    h: 40,
    label: 'Enclave 1',
    sub: '7001',
    edges: ['chains'],
    details: [
      { label: 'Language', value: 'Rust (axum HTTP)' },
      { label: 'Crypto', value: 'FROST-Ed25519' },
      { label: 'Encryption', value: 'AES-256-GCM, Argon2id' },
      { label: 'DKG rounds', value: 'Part1/Part2/Part3' },
      { label: 'Sign rounds', value: 'Round1/Round2/Aggregate' },
    ],
  },
  {
    id: 'enclave2',
    x: 400,
    y: 488,
    w: 120,
    h: 40,
    label: 'Enclave 2',
    sub: '7002',
    edges: ['chains'],
    details: [
      { label: 'Language', value: 'Rust (axum HTTP)' },
      { label: 'Crypto', value: 'FROST-Ed25519' },
      { label: 'Encryption', value: 'AES-256-GCM, Argon2id' },
      { label: 'DKG rounds', value: 'Part1/Part2/Part3' },
      { label: 'Sign rounds', value: 'Round1/Round2/Aggregate' },
    ],
  },
  {
    id: 'enclave3',
    x: 550,
    y: 488,
    w: 120,
    h: 40,
    label: 'Enclave 3',
    sub: '7003',
    edges: ['chains'],
    details: [
      { label: 'Language', value: 'Rust (axum HTTP)' },
      { label: 'Crypto', value: 'FROST-Ed25519' },
      { label: 'Encryption', value: 'AES-256-GCM, Argon2id' },
      { label: 'DKG rounds', value: 'Part1/Part2/Part3' },
      { label: 'Sign rounds', value: 'Round1/Round2/Aggregate' },
    ],
  },
  {
    id: 'chains',
    x: 355,
    y: 570,
    w: 250,
    h: 44,
    label: 'Ethereum / Solana',
    sub: 'blockchain',
    edges: [],
    details: [
      { label: 'Ethereum', value: 'EIP-1559, ERC-20, Sepolia' },
      { label: 'Solana', value: 'Native SOL, SPL Tokens, Devnet' },
      { label: 'Package (ETH)', value: 'pkg/blockchain/ethereum' },
      { label: 'Package (SOL)', value: 'pkg/blockchain/solana' },
    ],
  },
]

function getConnectionPath(a: Node, b: Node): string {
  const ax = a.x + a.w / 2
  const ay = a.y + a.h
  const bx = b.x + b.w / 2
  const by = b.y
  const cy = (ay + by) / 2
  return `M ${ax} ${ay} C ${ax} ${cy}, ${bx} ${cy}, ${bx} ${by}`
}

export function Architecture() {
  const [active, setActive] = useState<string | null>(null)

  const connectedTo = (id: string): Set<string> => {
    const set = new Set<string>([id])
    const node = nodes.find((n) => n.id === id)
    if (node) {
      for (const e of node.edges) set.add(e)
      for (const n of nodes) {
        if (n.edges.includes(id)) set.add(n.id)
      }
    }
    return set
  }

  const connected = active ? connectedTo(active) : new Set(nodes.map((n) => n.id))
  const activeNode = active ? nodes.find((n) => n.id === active) : null

  return (
    <section id="architecture" className="px-6 py-20 lg:py-28">
      <div className="mx-auto max-w-page">
        <SectionTitle
          label="Architecture"
          title="Full system diagram."
          description="Hover any component to see details and connections."
        />

        <div className="relative mt-12 flex flex-col items-center lg:flex-row lg:items-start lg:gap-8">
          <div className="w-full overflow-x-auto lg:flex-1">
            <svg viewBox="0 0 960 650" className="w-full max-w-[720px] mx-auto">
              <defs>
                <filter id="glow">
                  <feGaussianBlur stdDeviation="2" result="blur" />
                  <feMerge>
                    <feMergeNode in="blur" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
                <linearGradient id="lineGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#d1d5db" />
                  <stop offset="100%" stopColor="#9ca3af" />
                </linearGradient>
              </defs>

              {nodes.map((node) =>
                node.edges
                  .map((targetId) => {
                    const target = nodes.find((n) => n.id === targetId)
                    if (!target) return null
                    const isActive = connected.has(node.id) && connected.has(targetId)
                    return (
                      <path
                        key={`${node.id}-${targetId}`}
                        d={getConnectionPath(node, target)}
                        fill="none"
                        stroke={isActive ? '#059669' : '#e5e7eb'}
                        strokeWidth={isActive ? 2 : 1}
                        opacity={isActive ? 1 : 0.3}
                        className="transition-all duration-300"
                      />
                    )
                  })
              )}

              {nodes.map((node) => {
                const isActive = connected.has(node.id)
                const isHovered = active === node.id
                return (
                  <g
                    key={node.id}
                    onMouseEnter={() => setActive(node.id)}
                    onMouseLeave={() => setActive(null)}
                    className="cursor-pointer"
                    style={{ opacity: isActive ? 1 : 0.3 }}
                  >
                    <rect
                      x={node.x}
                      y={node.y}
                      width={node.w}
                      height={node.h}
                      rx={12}
                      ry={12}
                      fill={isHovered ? '#f0fdf4' : '#ffffff'}
                      stroke={isHovered ? '#059669' : '#e5e7eb'}
                      strokeWidth={isHovered ? 2 : 1}
                      filter={isHovered ? 'url(#glow)' : undefined}
                      className="transition-all duration-200"
                    />
                    <text
                      x={node.x + node.w / 2}
                      y={node.sub ? node.y + 18 : node.y + node.h / 2 + 1}
                      textAnchor="middle"
                      className="text-xs font-semibold fill-gray-900"
                    >
                      {node.label}
                    </text>
                    {node.sub && (
                      <text
                        x={node.x + node.w / 2}
                        y={node.y + 32}
                        textAnchor="middle"
                        className="text-[9px] fill-gray-400"
                      >
                        {node.sub}
                      </text>
                    )}
                  </g>
                )
              })}
            </svg>
          </div>

          <AnimatePresence mode="wait">
            {activeNode && (
              <motion.div
                key={activeNode.id}
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: 20 }}
                className="mt-6 w-full rounded-2xl border border-gray-200 bg-gray-50 p-6 lg:mt-0 lg:w-72 lg:shrink-0"
              >
                <h3 className="text-base font-semibold">{activeNode.label}</h3>
                <div className="mt-4 space-y-3">
                  {activeNode.details.map((d) => (
                    <div key={d.label}>
                      <div className="text-xs font-medium text-gray-500">
                        {d.label}
                      </div>
                      <div className="mt-0.5 text-sm text-gray-900">
                        {d.value}
                      </div>
                    </div>
                  ))}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>
    </section>
  )
}
