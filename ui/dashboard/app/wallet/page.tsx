'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { 
  Wallet, 
  Plus, 
  Trash2, 
  Loader2,
  Copy,
  CheckCircle
} from 'lucide-react';
import Link from 'next/link';
import { api, WalletInfo } from '@/lib/api';
import { formatAddress, formatDate, cn, ChainBadge } from '@/lib/utils';

export default function WalletsPage() {
  const queryClient = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [walletName, setWalletName] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const { data: wallets, isLoading } = useQuery({
    queryKey: ['wallets'],
    queryFn: api.listWallets,
  });

  const createMutation = useMutation({
    mutationFn: (name: string) => api.createWallet(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
      setCreating(false);
      setWalletName('');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.deleteWallet(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
    },
  });

  const copyAddress = (address: string, id: string) => {
    navigator.clipboard.writeText(address);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Wallets</h1>
          <p className="text-muted-foreground">Manage your treasury wallets</p>
        </div>
        <button
          onClick={() => setCreating(true)}
          className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90"
        >
          <Plus className="h-4 w-4" />
          Create Wallet
        </button>
      </div>

      {/* Create Wallet Modal */}
      {creating && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-card rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4">Create New Wallet</h2>
            <input
              type="text"
              placeholder="Wallet name"
              value={walletName}
              onChange={(e) => setWalletName(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg mb-4"
              autoFocus
            />
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => {
                  setCreating(false);
                  setWalletName('');
                }}
                className="px-4 py-2 text-muted-foreground hover:text-foreground"
              >
                Cancel
              </button>
              <button
                onClick={() => createMutation.mutate(walletName || 'New Wallet')}
                disabled={createMutation.isPending}
                className="px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50"
              >
                {createMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  'Create'
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Wallet List */}
      {isLoading ? (
        <div className="flex items-center justify-center p-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : wallets?.length === 0 ? (
        <div className="text-center p-12 border rounded-lg">
          <Wallet className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
          <h3 className="text-lg font-semibold mb-2">No wallets yet</h3>
          <p className="text-muted-foreground mb-4">Create your first wallet to get started</p>
          <button
            onClick={() => setCreating(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium"
          >
            <Plus className="h-4 w-4" />
            Create Wallet
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {wallets?.map((walletData) => (
            <WalletCard
              key={walletData.wallet.id}
              wallet={walletData}
              onDelete={() => deleteMutation.mutate(walletData.wallet.id)}
              deleting={deleteMutation.isPending}
              onCopy={copyAddress}
              copiedId={copiedId}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function WalletCard({ 
  wallet, 
  onDelete, 
  deleting,
  onCopy,
  copiedId,
}: { 
  wallet: WalletInfo;
  onDelete: () => void;
  deleting: boolean;
  onCopy: (address: string, id: string) => void;
  copiedId: string | null;
}) {
  return (
    <div className="rounded-lg border bg-card overflow-hidden">
      <div className="p-4 border-b bg-muted/30">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold truncate">{wallet.wallet.name}</h3>
          <button
            onClick={onDelete}
            disabled={deleting}
            className="p-1 text-muted-foreground hover:text-destructive"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
        <p className="text-xs text-muted-foreground">
          Created {formatDate(wallet.wallet.created_at)}
        </p>
      </div>
      
      <div className="p-4 space-y-3">
        {wallet.accounts.map((account) => (
          <div key={account.address} className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <ChainBadge chain={account.chain} />
              </div>
              <div className="flex items-center gap-2">
                <code className="text-sm font-mono">{formatAddress(account.address, 10)}</code>
                <button
                  onClick={() => onCopy(account.address, account.address)}
                  className="p-1 hover:text-primary"
                >
                  {copiedId === account.address ? (
                    <CheckCircle className="h-3 w-3 text-green-500" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </button>
              </div>
            </div>
            <span className="text-xs px-2 py-1 bg-muted rounded">{account.signer_type}</span>
          </div>
        ))}
      </div>
      
      <Link
        href={`/wallet/${wallet.wallet.id}`}
        className="block p-3 text-center border-t text-sm text-primary hover:bg-muted/50"
      >
        View Details →
      </Link>
    </div>
  );
}