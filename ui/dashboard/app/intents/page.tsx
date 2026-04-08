'use client';

import { Suspense, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { 
  FileText, 
  Loader2, 
  ChevronLeft, 
  ChevronRight,
  Search
} from 'lucide-react';
import { api } from '@/lib/api';
import { formatAddress, formatTimeAgo, getStatusColor, cn } from '@/lib/utils';

function IntentsContent() {
  const searchParams = useSearchParams();
  const walletId = searchParams.get('wallet_id');
  
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState<string>('');
  const limit = 20;
  const offset = (page - 1) * limit;

  const { data, isLoading } = useQuery({
    queryKey: ['intents', { wallet_id: walletId, status: statusFilter || undefined, limit, offset }],
    queryFn: () => api.listIntents({ 
      wallet_id: walletId || undefined, 
      status: statusFilter || undefined, 
      limit, 
      offset,
      order: 'desc' 
    }),
  });

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  const statuses = ['', 'draft', 'pending', 'approved', 'sent', 'failed', 'rejected', 'expired'];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Transaction Intents</h1>
        <p className="text-muted-foreground">Track and manage all treasury transactions</p>
      </div>

      <div className="flex items-center gap-4"> 
        <div className="flex items-center gap-2"> 
          <Search className="h-4 w-4 text-muted-foreground" />
          <select
            value={statusFilter}
            onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
            className="px-3 py-2 border rounded-lg"
          >
            {statuses.map((status) => (
              <option key={status} value={status}>
                {status === '' ? 'All Statuses' : status.charAt(0).toUpperCase() + status.slice(1)}
              </option>
            ))}
          </select>
        </div>
        {walletId && (
          <Link href="/intents" className="text-sm text-primary hover:underline">
            Clear wallet filter
          </Link>
        )}
      </div>

      {/* Table */}
      <div className="rounded-lg border bg-card overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center p-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : data?.intents.length === 0 ? (
          <div className="p-12 text-center text-muted-foreground">
            <FileText className="h-12 w-12 mx-auto mb-4" />
            <h3 className="text-lg font-semibold mb-2">No intents found</h3>
            <p className="text-sm">Try adjusting your filters or create a new transaction</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="border-b bg-muted/50">
                <tr>
                  <th className="px-4 py-3 text-left text-sm font-medium">Intent ID</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">Status</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">From</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">To</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">Value</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {data?.intents.map((intent) => (
                  <tr key={intent.id} className="hover:bg-muted/50">
                    <td className="px-4 py-3">
                      <Link href={`/intents/${intent.id}`} className="font-mono text-sm hover:text-primary">
                        {intent.id.slice(0, 8)}...
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      <span className={cn('px-2 py-1 rounded-full text-xs font-medium', getStatusColor(intent.status))}>
                        {intent.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-sm font-mono">{formatAddress(intent.from, 8)}</td>
                    <td className="px-4 py-3 text-sm font-mono">{formatAddress(intent.to, 8)}</td>
                    <td className="px-4 py-3 text-sm font-medium">{intent.value} ETH</td>
                    <td className="px-4 py-3 text-sm text-muted-foreground">{formatTimeAgo(intent.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t">
            <p className="text-sm text-muted-foreground">
              Showing {offset + 1} to {Math.min(offset + limit, data?.total || 0)} of {data?.total} intents
            </p>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setPage(p => Math.max(1, p - 1))}
                disabled={page === 1}
                className="p-2 rounded-lg hover:bg-muted disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4" />
              </button>
              <span className="text-sm">Page {page} of {totalPages}</span>
              <button
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
                className="p-2 rounded-lg hover:bg-muted disabled:opacity-50"
              >
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function IntentsPage() {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center p-12">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    }>
      <IntentsContent />
    </Suspense>
  );
}