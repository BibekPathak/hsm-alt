'use client';

import { useQuery } from '@tanstack/react-query';
import { Loader2, Zap, Clock, Rocket } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';

export default function FeesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['fee-estimate'],
    queryFn: () => api.getFeeEstimate('ethereum', 'standard'),
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Gas Fees</h1>
        <p className="text-muted-foreground">Current network fee estimates for Ethereum</p>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center p-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <div className="p-8 text-center border rounded-lg">
          Failed to load fee data. The RPC endpoint may be rate-limited.
        </div>
      ) : data && (
        <>
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <FeeCard title="Base Fee" value={data.base_fee} unit="gwei" icon={Zap} />
            <FeeCard title="Priority Fee" value={data.priority_fee} unit="gwei" icon={Clock} />
            <FeeCard title="Max Fee" value={data.max_fee} unit="gwei" icon={Rocket} color="blue" />
            <FeeCard title="Legacy Gas Price" value={data.gas_price_legacy} unit="gwei" />
          </div>

          <div className="rounded-lg border bg-card p-6">
            <h2 className="font-semibold mb-4">Speed Presets</h2>
            <div className="grid grid-cols-3 gap-4">
              <PresetCard name="Slow" description="Lower cost, longer confirmation" value={data.presets.slow} icon={Clock} color="gray" />
              <PresetCard name="Standard" description="Balanced cost and speed" value={data.presets.standard} icon={Zap} color="blue" />
              <PresetCard name="Fast" description="Higher cost, fastest confirmation" value={data.presets.fast} icon={Rocket} color="green" />
            </div>
          </div>

          <div className="rounded-lg border bg-card p-6">
            <h2 className="font-semibold mb-4">About EIP-1559</h2>
            <p className="text-sm text-muted-foreground leading-relaxed">
              EIP-1559 introduces a dual-fee system for Ethereum transactions. The <strong>base fee</strong> is determined by 
              network demand and is burned, while the <strong>priority fee</strong> (tip) goes to validators. 
              Transactions set a <strong>max fee</strong> they are willing to pay, ensuring you never overpay.
            </p>
          </div>
        </>
      )}
    </div>
  );
}

function FeeCard({ title, value, unit, icon: Icon, color }: { title: string; value: string; unit: string; icon?: any; color?: string }) {
  return (
    <div className={cn("rounded-lg border p-4", color === 'blue' ? 'bg-blue-50 dark:bg-blue-950' : 'bg-card')}>
      <div className="flex items-center justify-between mb-2">
        <p className="text-sm text-muted-foreground">{title}</p>
        {Icon && <Icon className={cn("h-4 w-4", color === 'blue' ? 'text-blue-600' : 'text-muted-foreground')} />}
      </div>
      <p className="text-2xl font-bold">{value}</p>
      <p className="text-xs text-muted-foreground">{unit}</p>
    </div>
  );
}

function PresetCard({ name, description, value, icon: Icon, color }: { name: string; description: string; value: string; icon?: any; color?: string }) {
  return (
    <div className={cn("rounded-lg border p-4", 
      color === 'blue' ? 'bg-blue-50 dark:bg-blue-950 border-blue-200' : 
      color === 'green' ? 'bg-green-50 dark:bg-green-950 border-green-200' : 
      'bg-card')}> 
      <div className="flex items-center gap-2 mb-2">
        {Icon && <Icon className={cn("h-4 w-4", 
          color === 'blue' ? 'text-blue-600' : 
          color === 'green' ? 'text-green-600' : 
          'text-muted-foreground')} />}
        <h3 className="font-semibold">{name}</h3>
      </div>
      <p className="text-2xl font-bold mb-1">{value} <span className="text-sm font-normal">gwei</span></p>
      <p className="text-xs text-muted-foreground">{description}</p>
    </div>
  );
}