import './globals.css';
import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { Providers } from '@/components/providers';
import { Sidebar } from '@/components/sidebar';
import { cn } from '@/lib/utils';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'HSM Treasury Dashboard',
  description: 'Distributed Key Custody & Treasury Management',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={cn(inter.className, 'flex min-h-screen')}>
        <Providers>
          <Sidebar />
          <main className="flex-1 p-8 overflow-auto">
            {children}
          </main>
        </Providers>
      </body>
    </html>
  );
}