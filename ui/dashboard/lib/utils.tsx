import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatAddress(address: string, chars: number = 6): string {
  if (!address) return '';
  if (address.length <= chars * 2) return address;
  return `${address.slice(0, chars)}...${address.slice(-chars)}`;
}

export function formatBalance(balance: string, decimals: number = 6): string {
  const num = parseFloat(balance);
  if (isNaN(num)) return '0';
  return num.toFixed(decimals);
}

export function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
  return formatDate(dateString);
}

export const statusColors: Record<string, string> = {
  draft: 'bg-gray-100 text-gray-800',
  pending: 'bg-yellow-100 text-yellow-800',
  approved: 'bg-green-100 text-green-800',
  executing: 'bg-blue-100 text-blue-800',
  sent: 'bg-green-100 text-green-800',
  failed: 'bg-red-100 text-red-800',
  rejected: 'bg-red-100 text-red-800',
  expired: 'bg-orange-100 text-orange-800',
  permanent_fail: 'bg-red-100 text-red-800',
};

export function getStatusColor(status: string): string {
  return statusColors[status] || 'bg-gray-100 text-gray-800';
}

export const chainColors: Record<string, string> = {
  ethereum: 'bg-blue-500',
  solana: 'bg-purple-500',
};

export function getChainColor(chain: string): string {
  return chainColors[chain] || 'bg-gray-500';
}

export function ChainBadge({ chain }: { chain: string }) {
  return (
    <span className={`text-xs px-2 py-0.5 rounded text-white ${getChainColor(chain)}`}>
      {chain.toUpperCase()}
    </span>
  );
}