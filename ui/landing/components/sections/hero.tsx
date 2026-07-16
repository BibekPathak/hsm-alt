'use client'

import { ArrowRight, Github, Play } from 'lucide-react'
import Balancer from 'react-wrap-balancer'
import Image from 'next/image'

export function Hero() {
  return (
    <section className="relative overflow-hidden px-6 pt-32 pb-20 lg:px-8 lg:pt-40 lg:pb-28">
      <div className="mx-auto max-w-page">
        <div className="mx-auto max-w-3xl text-center">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-gray-200 bg-gray-50 px-4 py-1.5 text-sm text-gray-600">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            Open-source treasury infrastructure
          </div>

          <h1 className="text-4xl font-bold tracking-tight sm:text-5xl md:text-6xl lg:text-7xl">
            <Balancer>
              Hardware-Backed
              <br />
              Multi-Chain
              <br />
              Treasury Infrastructure
            </Balancer>
          </h1>

          <p className="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-gray-600 sm:text-lg lg:text-xl">
            <Balancer>
              Secure digital assets with MPC threshold signing, ZK policy
              verification, and intent-based workflows. Open source from day
              one.
            </Balancer>
          </p>

          <div className="mt-8 flex items-center justify-center gap-4">
            <a
              href="#demo"
              className="inline-flex items-center gap-2 rounded-2xl bg-black px-6 py-3 text-sm font-medium text-white transition-opacity hover:opacity-90"
            >
              <Play className="h-4 w-4" />
              View Live Demo
            </a>
            <a
              href="https://github.com/BibekPathak/hsm-alt"
              className="inline-flex items-center gap-2 rounded-2xl border border-gray-200 bg-white px-6 py-3 text-sm font-medium text-gray-900 transition-colors hover:bg-gray-50"
            >
              <Github className="h-4 w-4" />
              GitHub
            </a>
          </div>

          <div className="mt-8 flex items-center justify-center gap-2 text-sm text-gray-500">
            <span className="flex items-center gap-1">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              12,000+ lines
            </span>
            <span className="text-gray-300">·</span>
            <span className="flex items-center gap-1">
              Go + Rust
            </span>
            <span className="text-gray-300">·</span>
            <span className="flex items-center gap-1">
              FROST + Groth16
            </span>
          </div>
        </div>

        <div className="mx-auto mt-16 max-w-4xl">
          <div className="overflow-hidden rounded-2xl border border-gray-200 shadow-sm">
            <Image
              src="/dashboard-preview.png"
              alt="HSM Treasury Dashboard"
              width={1200}
              height={675}
              className="w-full"
              priority
            />
          </div>
        </div>
      </div>
    </section>
  )
}
