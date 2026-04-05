'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'next/navigation';
import { ArrowLeft, Loader2, CheckCircle, XCircle, Play, RotateCcw, ExternalLink } from 'lucide-react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { formatAddress, formatDate, getStatusColor, cn } from '@/lib/utils';

export default function IntentDetailPage() {
  const params = useParams();
  const intentId = params.id as string;
  const queryClient = useQueryClient();

  const { data: intent, isLoading, error } = useQuery({
    queryKey: ['intent', intentId],
    queryFn: () => api.getIntent(intentId),
  });

  const approveMutation = useMutation({
    mutationFn: (approver: string) => api.approveIntent(intentId, approver),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['intent', intentId] }),
  });

  const rejectMutation = useMutation({
    mutationFn: (data: { rejecter: string; reason: string }) => api.rejectIntent(intentId, data.rejecter, data.reason),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['intent', intentId] }),
  });

  const executeMutation = useMutation({
    mutationFn: () => api.executeIntent(intentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['intent', intentId] }),
  });

  const retryMutation = useMutation({
    mutationFn: () => api.retryIntent(intentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['intent', intentId] }),
  });

  if (isLoading) {
    return <div className="flex items-center justify-center p-12"><Loader2 className="h-8 w-8 animate-spin text-muted-foreground" /></div>;
  }

  if (error || !intent) {
    return <div className="text-center p-12"><h2 className="text-xl font-semibold">Intent not found</h2><Link href="/intents" className="text-primary hover:underline mt-2 inline-block">Back to Intents</Link></div>;
  }

  const canApprove = intent.status === 'draft' || intent.status === 'pending';
  const canExecute = intent.status === 'approved';
  const canRetry = intent.status === 'failed' && intent.retry_count < intent.max_retries;
  const canReject = intent.status === 'draft' || intent.status === 'pending';

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link href="/intents" className="p-2 hover:bg-muted rounded-lg"><ArrowLeft className="h-5 w-5" /></Link>
        <div>
          <h1 className="text-3xl font-bold">Intent Details</h1>
          <p className="text-muted-foreground font-mono text-sm">{intent.id}</p>
        </div>
        <span className={cn('ml-auto px-3 py-1 rounded-full text-sm font-medium', getStatusColor(intent.status))}>{intent.status}</span>
      </div>

      <div className="flex gap-2 flex-wrap">
        {canApprove && <button onClick={() => approveMutation.mutate('operator')} disabled={approveMutation.isPending} className="inline-flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg font-medium hover:bg-green-700 disabled:opacity-50"><CheckCircle className="h-4 w-4" /> Approve</button>}
        {canReject && <button onClick={() => rejectMutation.mutate({ rejecter: 'operator', reason: 'Rejected by operator' })} disabled={rejectMutation.isPending} className="inline-flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg font-medium hover:bg-red-700 disabled:opacity-50"><XCircle className="h-4 w-4" /> Reject</button>}
        {canExecute && <button onClick={() => executeMutation.mutate()} disabled={executeMutation.isPending} className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50"><Play className="h-4 w-4" /> Execute</button>}
        {canRetry && <button onClick={() => retryMutation.mutate()} disabled={retryMutation.isPending} className="inline-flex items-center gap-2 px-4 py-2 bg-orange-600 text-white rounded-lg font-medium hover:bg-orange-700 disabled:opacity-50"><RotateCcw className="h-4 w-4" /> Retry</button>}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="rounded-lg border bg-card p-6">
          <h2 className="font-semibold mb-4">Transaction</h2>
          <dl className="space-y-3">
            <div className="flex justify-between"><dt className="text-muted-foreground">From</dt><dd className="font-mono text-sm">{formatAddress(intent.from, 8)}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">To</dt><dd className="font-mono text-sm">{formatAddress(intent.to, 8)}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Value</dt><dd className="font-semibold">{intent.value} ETH</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Gas Limit</dt><dd>{intent.gas_limit.toLocaleString()}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Fee Speed</dt><dd className="capitalize">{intent.fee_speed || 'standard'}</dd></div>
          </dl>
        </div>
        <div className="rounded-lg border bg-card p-6">
          <h2 className="font-semibold mb-4">Status</h2>
          <dl className="space-y-3">
            <div className="flex justify-between"><dt className="text-muted-foreground">Status</dt><dd><span className={cn('px-2 py-1 rounded-full text-xs font-medium', getStatusColor(intent.status))}>{intent.status}</span></dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Required Sigs</dt><dd>{intent.required_sigs}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Approved By</dt><dd>{intent.approved_by?.join(', ') || 'None'}</dd></div>
            <div className="flex justify-between"><dt className="text-muted-foreground">Retries</dt><dd>{intent.retry_count} / {intent.max_retries}</dd></div>
            {intent.tx_hash && <div className="flex justify-between"><dt className="text-muted-foreground">Tx Hash</dt><dd><a href={`https://sepolia.etherscan.io/tx/${intent.tx_hash}`} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1 text-primary hover:underline">{formatAddress(intent.tx_hash, 10)}<ExternalLink className="h-3 w-3"/></a></dd></div>}
            {intent.error && <div><dt className="text-muted-foreground mb-1">Error</dt><dd className="text-sm text-red-500">{intent.error}</dd></div>}
          </dl>
        </div>
      </div>
    </div>
  );
}