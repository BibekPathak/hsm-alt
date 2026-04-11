'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'next/navigation';
import { 
  Wallet, 
  Copy, 
  CheckCircle,
  Loader2,
  Send,
  ArrowLeft
} from 'lucide-react';
import Link from 'next/link';
import { useState } from 'react';
import { api, WalletSummary, TransactionIntent } from '@/lib/api';
import { formatAddress, formatBalance, getStatusColor, ChainBadge, cn } from '@/lib/utils';

export default function WalletDetailPage() {
  const params = useParams();
  const walletId = params.id as string;
  const queryClient = useQueryClient();
  const [copiedAddress, setCopiedAddress] = useState<string | null>(null);
  const [showSendModal, setShowSendModal] = useState(false);

  const { data: summary, isLoading } = useQuery({
    queryKey: ['wallet-summary', walletId],
    queryFn: () => api.getWalletSummary(walletId),
  });

  const { data: intentsData } = useQuery({
    queryKey: ['intents', walletId],
    queryFn: () => api.listIntents({ wallet_id: walletId, limit: 10 }),
  });

  const createIntentMutation = useMutation({
    mutationFn: (data: { 
      to: string; 
      value: string; 
      fee_speed?: string;
      chain?: string;
      token_address?: string;
      token_decimals?: number;
      token_symbol?: string;
      token_type?: string;
    }) => 
      api.createIntent({ ...data, wallet_id: walletId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['intents', walletId] });
      queryClient.invalidateQueries({ queryKey: ['wallet-summary', walletId] });
      setShowSendModal(false);
    },
  });

  const copyAddress = (address: string) => {
    navigator.clipboard.writeText(address);
    setCopiedAddress(address);
    setTimeout(() => setCopiedAddress(null), 2000);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center p-12">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!summary) {
    return (
      <div className="text-center p-12">
        <h2 className="text-xl font-semibold">Wallet not found</h2>
        <Link href="/wallet" className="text-primary hover:underline mt-2 inline-block">
          Back to Wallets
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link href="/wallet" className="p-2 hover:bg-muted rounded-lg">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-3xl font-bold">{summary.name}</h1>
          <p className="text-muted-foreground">Wallet Details</p>
        </div>
      </div>

      {/* Balance Card */}
      <div className="rounded-lg border bg-card p-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-muted-foreground">Total Balance</p>
            <p className="text-4xl font-bold">{summary.total_balance}</p>
          </div>
          <button
            onClick={() => setShowSendModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90"
          >
            <Send className="h-4 w-4" />
            Send
          </button>
        </div>
        
        {/* Per-chain balances */}
        <div className="mt-4 pt-4 border-t grid grid-cols-2 gap-4">
          {summary.addresses?.map((addr) => (
            <div key={addr.address} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <ChainBadge chain={addr.chain} />
                <span className="text-sm text-muted-foreground">{addr.balance}</span>
              </div>
            </div>
          ))}
        </div>
        
        {/* Fee estimates */}
        {summary.fees && (
          <div className="mt-4 pt-4 border-t">
            <p className="text-sm text-muted-foreground mb-2">Estimated Fees</p>
            <div className="grid grid-cols-3 gap-4 text-sm">
              <div>
                <span className="text-muted-foreground">Native: </span>
                <span className="font-mono">{summary.fees.native}</span>
              </div>
              <div>
                <span className="text-muted-foreground">ERC-20: </span>
                <span className="font-mono">{summary.fees.erc20}</span>
              </div>
              <div>
                <span className="text-muted-foreground">SPL: </span>
                <span className="font-mono">{summary.fees.spl}</span>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Addresses */}
      <div className="rounded-lg border bg-card">
        <div className="p-4 border-b">
          <h2 className="font-semibold">Addresses</h2>
        </div>
        <div className="divide-y">
          {summary.addresses.map((addr) => (
            <div key={addr.address} className="p-4 flex items-center justify-between">
              <div>
                <p className="text-xs text-muted-foreground uppercase">{addr.chain}</p>
                <div className="flex items-center gap-2">
                  <code className="font-mono">{addr.address}</code>
                  <button
                    onClick={() => copyAddress(addr.address)}
                    className="p-1 hover:text-primary"
                  >
                    {copiedAddress === addr.address ? (
                      <CheckCircle className="h-4 w-4 text-green-500" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </button>
                </div>
              </div>
              <div className="text-right">
                <p className="font-semibold">{addr.balance} ETH</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Intent Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Pending', count: summary.intents.draft + summary.intents.pending + summary.intents.approved, color: 'bg-yellow-50' },
          { label: 'Sent', count: summary.intents.sent, color: 'bg-green-50' },
          { label: 'Failed', count: summary.intents.failed, color: 'bg-red-50' },
          { label: 'Expired', count: summary.intents.expired, color: 'bg-orange-50' },
        ].map((stat) => (
          <div key={stat.label} className={cn('rounded-lg border p-4', stat.color)}>
            <p className="text-sm text-muted-foreground">{stat.label}</p>
            <p className="text-2xl font-bold">{stat.count}</p>
          </div>
        ))}
      </div>

      {/* Recent Intents */}
      <div className="rounded-lg border bg-card">
        <div className="flex items-center justify-between p-4 border-b">
          <h2 className="font-semibold">Recent Intents</h2>
          <Link href={`/intents?wallet_id=${walletId}`} className="text-sm text-primary hover:underline">
            View all
          </Link>
        </div>
        <div className="divide-y">
          {intentsData?.intents?.slice(0, 5).map((intent) => (
            <Link
              key={intent.id}
              href={`/intents/${intent.id}`}
              className="flex items-center justify-between p-4 hover:bg-muted/50"
            >
              <div className="flex items-center gap-3">
                <span className={cn('px-2 py-1 rounded-full text-xs font-medium', getStatusColor(intent.status))}>
                  {intent.status}
                </span>
                <span className="font-mono text-sm">{intent.value} ETH</span>
              </div>
              <span className="text-sm text-muted-foreground">{intent.to.slice(0, 10)}...</span>
            </Link>
          ))}
          {(!intentsData?.intents || intentsData.intents.length === 0) && (
            <div className="p-8 text-center text-muted-foreground">
              No intents yet
            </div>
          )}
        </div>
      </div>

      {/* Send Modal */}
      {showSendModal && (
        <SendModal
          onClose={() => setShowSendModal(false)}
          onSubmit={(data) => createIntentMutation.mutate(data)}
          loading={createIntentMutation.isPending}
          error={createIntentMutation.error?.message}
        />
      )}
    </div>
  );
}

function SendModal({ 
  onClose, 
  onSubmit, 
  loading, 
  error 
}: { 
  onClose: () => void; 
  onSubmit: (data: { 
    to: string; 
    value: string; 
    fee_speed?: string;
    chain?: string;
    token_address?: string;
    token_decimals?: number;
    token_symbol?: string;
    token_type?: string;
  }) => void;
  loading: boolean;
  error?: string;
}) {
  const [to, setTo] = useState('');
  const [value, setValue] = useState('');
  const [feeSpeed, setFeeSpeed] = useState('standard');
  const [chain, setChain] = useState('ethereum');
  const [tokenType, setTokenType] = useState('native');
  const [tokenAddress, setTokenAddress] = useState('');
  const [tokenDecimals, setTokenDecimals] = useState(18);
  const [tokenSymbol, setTokenSymbol] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const data: any = { to, value, fee_speed: feeSpeed, chain };
    
    if (tokenType !== 'native') {
      data.token_type = tokenType;
      data.token_address = tokenAddress;
      data.token_decimals = tokenDecimals;
      data.token_symbol = tokenSymbol;
    }
    
    onSubmit(data);
  };

  const getTokenSymbol = () => {
    if (tokenType === 'native') {
      return chain === 'solana' ? 'SOL' : 'ETH';
    }
    return tokenSymbol || 'TOKEN';
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-card rounded-lg p-6 w-full max-w-md">
        <h2 className="text-xl font-semibold mb-4">Send Transaction</h2>
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Chain</label>
            <select
              value={chain}
              onChange={(e) => setChain(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg"
            >
              <option value="ethereum">Ethereum</option>
              <option value="solana">Solana</option>
            </select>
          </div>
          
          <div>
            <label className="block text-sm font-medium mb-1">Token Type</label>
            <select
              value={tokenType}
              onChange={(e) => setTokenType(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg"
            >
              <option value="native">Native ({chain === 'solana' ? 'SOL' : 'ETH'})</option>
              <option value="erc20">ERC-20 Token</option>
              <option value="spl">SPL Token</option>
            </select>
          </div>
          
          {tokenType !== 'native' && (
            <>
              <div>
                <label className="block text-sm font-medium mb-1">
                  {tokenType === 'erc20' ? 'Token Contract Address' : 'Token Mint Address'}
                </label>
                <input
                  type="text"
                  value={tokenAddress}
                  onChange={(e) => setTokenAddress(e.target.value)}
                  placeholder={tokenType === 'erc20' ? '0x...' : 'Token mint...'}
                  className="w-full px-3 py-2 border rounded-lg"
                  required={tokenType !== 'native'}
                />
              </div>
              
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-sm font-medium mb-1">Decimals</label>
                  <input
                    type="number"
                    value={tokenDecimals}
                    onChange={(e) => setTokenDecimals(parseInt(e.target.value) || 18)}
                    placeholder="18"
                    className="w-full px-3 py-2 border rounded-lg"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium mb-1">Symbol</label>
                  <input
                    type="text"
                    value={tokenSymbol}
                    onChange={(e) => setTokenSymbol(e.target.value.toUpperCase())}
                    placeholder="USDC"
                    className="w-full px-3 py-2 border rounded-lg"
                  />
                </div>
              </div>
            </>
          )}
          
          <div>
            <label className="block text-sm font-medium mb-1">
              Amount ({getTokenSymbol()})
            </label>
            <input
              type="text"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="0.0"
              className="w-full px-3 py-2 border rounded-lg"
              required
            />
          </div>

          {tokenType === 'native' && (
            <div>
              <label className="block text-sm font-medium mb-1">Fee Speed</label>
              <select
                value={feeSpeed}
                onChange={(e) => setFeeSpeed(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg"
              >
                <option value="slow">Slow (1.1x base fee)</option>
                <option value="standard">Standard (1.3x base fee)</option>
                <option value="fast">Fast (1.6x base fee)</option>
              </select>
            </div>
          )}

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}

          <div className="flex gap-2 justify-end pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-muted-foreground hover:text-foreground"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-lg font-medium hover:bg-primary/90 disabled:opacity-50"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Create Intent'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}