import { Hero } from '@/components/sections/hero'
import { Problem } from '@/components/sections/problem'
import { Metrics } from '@/components/sections/metrics'
import { HowItWorks } from '@/components/sections/how-it-works'
import { Features } from '@/components/sections/features'
import { Architecture } from '@/components/sections/architecture'
import { Demo } from '@/components/sections/demo'
import { Security } from '@/components/sections/security'
import { ThreatModel } from '@/components/sections/threat-model'
import { Roadmap } from '@/components/sections/roadmap'
import { QuickStart } from '@/components/sections/quick-start'
import { Engineering } from '@/components/sections/engineering'
import { CTA } from '@/components/sections/cta'

export default function Home() {
  return (
    <>
      <Hero />
      <Problem />
      <Metrics />
      <HowItWorks />
      <Features />
      <Architecture />
      <Demo />
      <Security />
      <ThreatModel />
      <Roadmap />
      <QuickStart />
      <Engineering />
      <CTA />
    </>
  )
}
