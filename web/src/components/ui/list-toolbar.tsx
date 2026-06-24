import { Filter, Grid3X3, List, Search } from 'lucide-react'
import { cn } from '@/lib/utils'

export function ListToolbar({
  search,
  onSearch,
  showFilter,
  onToggleFilter,
  activeFilterCount,
  tileView,
  onSetTileView,
}: {
  search: string
  onSearch: (v: string) => void
  showFilter: boolean
  onToggleFilter: () => void
  activeFilterCount: number
  tileView: boolean
  onSetTileView: (v: boolean) => void
}) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="relative flex-1 max-w-md">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          placeholder="Search..."
          value={search}
          onChange={(e) => onSearch(e.target.value)}
          className="w-full rounded-lg border border-input bg-background py-2 pl-10 pr-4 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring transition-colors"
        />
      </div>
      <div className="ml-auto flex items-center gap-2">
        <button
          type="button"
          onClick={onToggleFilter}
          className={cn(
            'flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors',
            showFilter || activeFilterCount > 0
              ? 'border-primary/40 bg-primary/5 text-primary'
              : 'border-input bg-background text-muted-foreground hover:text-foreground'
          )}
        >
          <Filter className="h-4 w-4" />
          <span>
            Filter / Sort
            {activeFilterCount > 0 ? ` (${activeFilterCount})` : ''}
          </span>
        </button>
        <div className="flex items-center gap-1 rounded-lg border border-border p-0.5">
          <button
            type="button"
            aria-label="List view"
            aria-pressed={!tileView}
            onClick={() => onSetTileView(false)}
            title="List view"
            className={cn(
              'rounded-md p-1.5 transition-colors',
              !tileView
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <List className="h-4 w-4" />
          </button>
          <button
            type="button"
            aria-label="Tile view"
            aria-pressed={tileView}
            onClick={() => onSetTileView(true)}
            title="Tile view"
            className={cn(
              'rounded-md p-1.5 transition-colors',
              tileView
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            <Grid3X3 className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  )
}
