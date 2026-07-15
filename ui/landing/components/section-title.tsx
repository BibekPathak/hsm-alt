import { cn } from '@/lib/utils'

interface SectionTitleProps {
  label?: string
  title: string
  description?: string
  className?: string
  align?: 'center' | 'left'
}

export function SectionTitle({
  label,
  title,
  description,
  className,
  align = 'center',
}: SectionTitleProps) {
  return (
    <div
      className={cn(
        'max-w-2xl',
        align === 'center' && 'mx-auto text-center',
        className
      )}
    >
      {label && (
        <p className="mb-4 text-sm font-medium text-gray-500 uppercase tracking-wider">
          {label}
        </p>
      )}
      <h2 className="text-3xl font-bold tracking-tight sm:text-4xl lg:text-5xl">
        {title}
      </h2>
      {description && (
        <p className="mt-4 text-base leading-relaxed text-gray-600 sm:text-lg">
          {description}
        </p>
      )}
    </div>
  )
}
