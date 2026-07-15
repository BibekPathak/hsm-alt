import { Hero } from '@/components/sections/hero'
import { Problem } from '@/components/sections/problem'
import { TechStack } from '@/components/sections/tech-stack'
import { HowItWorks } from '@/components/sections/how-it-works'
import { Features } from '@/components/sections/features'
import { Security } from '@/components/sections/security'
import { Roadmap } from '@/components/sections/roadmap'
import { Engineering } from '@/components/sections/engineering'
import { CTA } from '@/components/sections/cta'

export default function Home() {
  return (
    <>
      <Hero />
      <Problem />
      <TechStack />
      <HowItWorks />
      <Features />
      <Security />
      <Roadmap />
      <Engineering />
      <CTA />
    </>
  )
}
