'use client'

import Link from 'next/link'

const footerLinks = [
  {
    title: 'Platform',
    links: [
      { label: 'Features', href: '#features' },
      { label: 'How It Works', href: '#how-it-works' },
      { label: 'Security', href: '#security' },
      { label: 'Roadmap', href: '#roadmap' },
    ],
  },
  {
    title: 'Community',
    links: [
      { label: 'GitHub', href: 'https://github.com/BibekPathak/hsm-alt' },
      { label: 'Documentation', href: '#' },
      { label: 'Twitter / X', href: '#' },
    ],
  },
  {
    title: 'Legal',
    links: [
      { label: 'Privacy Policy', href: '#' },
      { label: 'Terms of Service', href: '#' },
    ],
  },
]

export function Footer() {
  return (
    <footer className="border-t border-gray-100 px-6 py-12">
      <div className="mx-auto max-w-page">
        <div className="flex flex-col gap-8 lg:flex-row lg:justify-between">
          <div className="max-w-xs">
            <div className="flex items-center gap-2">
              <div className="h-6 w-6 rounded-lg bg-black" />
              <span className="text-sm font-semibold">HSM</span>
            </div>
            <p className="mt-3 text-sm leading-relaxed text-gray-600">
              Hardware-backed MPC treasury infrastructure. Open-source from day one.
            </p>
          </div>

          <div className="grid grid-cols-2 gap-8 sm:grid-cols-3">
            {footerLinks.map((group) => (
              <div key={group.title}>
                <h4 className="text-sm font-semibold">{group.title}</h4>
                <ul className="mt-3 space-y-2">
                  {group.links.map((link) => (
                    <li key={link.label}>
                      <Link
                        href={link.href}
                        className="text-sm text-gray-600 transition-colors hover:text-black"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>

        <div className="mt-12 border-t border-gray-100 pt-6 text-center text-sm text-gray-500">
          &copy; {new Date().getFullYear()} HSM. All rights reserved.
        </div>
      </div>
    </footer>
  )
}
