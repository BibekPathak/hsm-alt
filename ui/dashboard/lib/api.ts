const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function fetchApi<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const url = `${API_URL}${endpoint}`;
  
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Request failed' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

// Types
export interface Wallet {
  id: string;
  name: string;
  created_at: string;
}

export interface Account {
  wallet_id: string;
  chain: string;
  address: string;
  pub_key: string;
  signer_type: string;
  index: number;
}

export interface WalletInfo {
  wallet: Wallet;
  accounts: Account[];
}

export interface WalletSummary {
  wallet_id: string;
  name: string;
  total_balance: string;
  addresses: AddressBalance[];
  intents: IntentStatusCounts;
}

export interface AddressBalance {
  address: string;
  balance: string;
  chain: string;
}

export interface IntentStatusCounts {
  pending: number;
  approved: number;
  executing: number;
  sent: number;
  failed: number;
  rejected: number;
  expired: number;
  draft: number;
}

export interface TransactionIntent {
  id: string;
  wallet_id: string;
  chain: string;
  from: string;
  to: string;
  value: string;
  value_eth: string;
  gas_limit: number;
  status: string;
  created_at: string;
  updated_at: string;
  expires_at: string;
  approved_by: string[];
  required_sigs: number;
  tx_hash: string;
  error: string;
  retry_count: number;
  max_retries: number;
  fee_speed: string;
}

export interface IntentListResponse {
  intents: TransactionIntent[];
  total: number;
  limit: number;
  offset: number;
}

export interface FeeEstimate {
  chain: string;
  type: string;
  base_fee: string;
  priority_fee: string;
  max_fee: string;
  gas_price_legacy: string;
  presets: {
    slow: string;
    standard: string;
    fast: string;
  };
}

// API Functions
export const api = {
  // Wallets
  listWallets: () => fetchApi<WalletInfo[]>('/wallet'),
  
  getWallet: (id: string) => fetchApi<WalletInfo>(`/wallet/${id}`),
  
  createWallet: (name: string) => 
    fetchApi<{ wallet_id: string; name: string; accounts: Account[] }>('/wallet/create', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),
  
  getWalletSummary: (id: string) => fetchApi<WalletSummary>(`/wallet/${id}/summary`),
  
  getWalletBalance: (id: string) => 
    fetchApi<{ address: string; balance: string; chain: string }>(`/wallet/${id}/balance`),
  
  deleteWallet: (id: string) => 
    fetchApi<{ wallet_id: string; deleted: boolean }>(`/wallet/${id}`, {
      method: 'DELETE',
    }),

  // Intents
  listIntents: (params?: {
    wallet_id?: string;
    status?: string;
    chain?: string;
    from?: string;
    to?: string;
    limit?: number;
    offset?: number;
    sort?: string;
    order?: string;
  }) => {
    const searchParams = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== '') {
          searchParams.append(key, String(value));
        }
      });
    }
    const query = searchParams.toString();
    return fetchApi<IntentListResponse>(`/intent${query ? `?${query}` : ''}`);
  },
  
  getIntent: (id: string) => fetchApi<TransactionIntent>(`/intent/${id}`),
  
  createIntent: (data: {
    wallet_id: string;
    to: string;
    value: string;
    chain?: string;
    gas_limit?: number;
    required_sigs?: number;
    fee_speed?: string;
  }) => fetchApi<TransactionIntent>('/intent', {
    method: 'POST',
    body: JSON.stringify(data),
  }),
  
  approveIntent: (id: string, approver: string) => 
    fetchApi<TransactionIntent>(`/intent/${id}/approve`, {
      method: 'POST',
      body: JSON.stringify({ approver }),
    }),
  
  rejectIntent: (id: string, rejecter: string, reason: string) => 
    fetchApi<{ intent_id: string; status: string; reason: string }>(`/intent/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ rejecter, reason }),
    }),
  
  executeIntent: (id: string) => 
    fetchApi<{ tx_hash: string; intent_id: string }>(`/intent/${id}/execute`, {
      method: 'POST',
    }),
  
  retryIntent: (id: string) => 
    fetchApi<TransactionIntent>(`/intent/${id}/retry`, {
      method: 'POST',
    }),

  // Fees
  getFeeEstimate: (chain?: string, speed?: string) => {
    const params = new URLSearchParams();
    if (chain) params.append('chain', chain);
    if (speed) params.append('speed', speed);
    const query = params.toString();
    return fetchApi<FeeEstimate>(`/fee-estimate${query ? `?${query}` : ''}`);
  },
};

export default api;