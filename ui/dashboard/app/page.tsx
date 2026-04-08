'use client';

import { useQuery } from '@tanstack/react-query';
import { 
  Wallet, 
  FileText, 
  TrendingUp, 
  AlertCircle,
  Loader2,
  Plus
} from 'lucide-react';
import Link from 'next/link';
import { api, WalletSummary, IntentListResponse } from '@/lib/api';
import { formatBalance, formatTimeAgo, getStatusColor, cn } from '@/lib/utils';

export default function DashboardPage() {
  // Fetch all wallets
  const { data: wallets, isLoading: loadingWallets } = useQuery({
    queryKey: ['wallets'],
    queryFn: api.listWallets,
  });

  // Fetch recent intents
  const { data: intentsData, isLoading: loadingIntents } = useQuery({
    queryKey: ['intents', { limit: 5 }],
    queryFn: () => api.listIntents({ limit: 5, order: 'desc' }),
  });

  // Calculate totals
  const totalBalance = wallets?.reduce((sum, w) => {
    return sum + parseFloat(w.accounts[0]?.address ? '0' : '0');
  }, 0) || 0;

  const pendingCount = intentsData?.intents?.filter(i => 
    i.status === 'draft' || i.status === 'pending' || i.status === 'approved'
  ).length || 0;

  const sentCount = intentsData?.intents?.filter(i => i.status === 'sent').length || 0;
  const failedCount = intentsData?.intents?.filter(i => i.status === 'failed').length || 0;

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Dashboard</h1>
          <p className="text-muted-foreground">Treasury overview and recent activity</p>
        </div>
        <Link
          href="/wallet"
          className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90"
        >
          <Plus className="h-4 w-4" />
          Create Wallet
        </Link>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <StatCard
          title="Total Wallets"
          value={wallets?.length || 0}
          icon={Wallet}
          loading={loadingWallets}
        />
        <StatCard
          title="Pending Actions"
          value={pendingCount}
          icon={AlertCircle}
          loading={loadingIntents}
          variant={pendingCount > 0 ? 'warning' : 'default'}
        />
        <StatCard
          title="Transactions Sent"
          value={sentCount}
          icon={TrendingUp}
          loading={loadingIntents}
          variant="success"
        />
        <StatCard
          title="Failed Transactions"
          value={failedCount}
          icon={AlertCircle}
          loading={loadingIntents}
          variant="destructive"
        />
      </div>

      {/* Recent Activity */}
      <div className="rounded-lg border bg-card">
        <div className="flex items-center justify-between p-4 border-b">
          <h2 className="text-lg font-semibold">Recent Intents</h2>
          <Link href="/intents" className="text-sm text-primary hover:underline">
            View all
          </Link>
        </div>
        
        {loadingIntents ? (
          <div className="flex items-center justify-center p-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : intentsData?.intents?.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            No intents yet. Create your first transaction intent.
          </div>
        ) : (
          <div className="divide-y">
            {intentsData?.intents?.map((intent) => (
              <Link
                key={intent.id}
                href={`/intents/${intent.id}`}
                className="flex items-center justify-between p-4 hover:bg-muted/50 transition-colors"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm">{intent.id.slice(0, 8)}...</span>
                    <span className={cn('px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(intent.status))}>
                      {intent.status}
                    </span>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    {intent.value} ETH → {intent.to.slice(0, 10)}...
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-sm text-muted-foreground">{formatTimeAgo(intent.created_at)}</p>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Wallets Grid */}
      <div className="rounded-lg border bg-card">
        <div className="flex items-center justify-between p-4 border-b">
          <h2 className="text-lg font-semibold">Wallets</h2>
          <Link href="/wallet" className="text-sm text-primary hover:underline">
            View all
          </Link>
        </div>
        
        {loadingWallets ? (
          <div className="flex items-center justify-center p-8">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : wallets?.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            No wallets yet. Create your first wallet to get started.
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 p-4">
            {wallets?.slice(0, 6).map((wallet) => (
              <Link
                key={wallet.wallet.id}
                href={`/wallet/${wallet.wallet.id}`}
                className="p-4 rounded-lg border hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-semibold">{wallet.wallet.name}</h3>
                  <Wallet className="h-4 w-4 text-muted-foreground" />
                </div>
                <p className="text-sm text-muted-foreground font-mono">
                  {wallet.accounts[0]?.address?.slice(0, 14)}...
                </p>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({ 
  title, 
  value, 
  icon: Icon, 
  loading, 
  variant = 'default' 
}: { 
  title: string; 
  value: number; 
  icon: any; 
  loading?: boolean;
  variant?: 'default' | 'success' | 'warning' | 'destructive';
}) {
  const variantClasses = {
    default: 'bg-card',
    success: 'bg-green-50 dark:bg-green-950',
    warning: 'bg-yellow-50 dark:bg-yellow-950',
    destructive: 'bg-red-50 dark:bg-red-950',
  };

  const iconVariantClasses = {
    default: 'text-primary',
    success: 'text-green-600 dark:text-green-400',
    warning: 'text-yellow-600 dark:text-yellow-400',
    destructive: 'text-red-600 dark:text-red-400',
  };

  return (
    <div className={cn('rounded-lg border p-4', variantClasses[variant])}>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-muted-foreground">{title}</p>
          {loading ? (
            <Loader2 className="h-6 w-6 animate-spin mt-1" />
          ) : (
            <p className="text-2xl font-bold">{value}</p>
          )}
        </div>
        <Icon className={cn('h-5 w-5', iconVariantClasses[variant])} />
      </div>
    </div>
  );
}