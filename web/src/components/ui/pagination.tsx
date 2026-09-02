import { ChevronLeft, ChevronRight } from 'lucide-react'

export function Pagination({
  page,
  totalPages,
  perPage,
  filteredCount,
  perPageOptions,
  onPage,
  onPerPage,
}: {
  page: number
  totalPages: number
  perPage: number
  filteredCount: number
  perPageOptions: number[]
  onPage: (p: number) => void
  onPerPage: (n: number) => void
}) {
  if (filteredCount === 0) return null
  const isAll = perPage === Infinity
  const start = isAll ? 1 : (page - 1) * perPage + 1
  const end = isAll ? filteredCount : Math.min(page * perPage, filteredCount)

  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-3">
        <p className="text-sm text-muted-foreground">
          {isAll ? `Showing all ${filteredCount}` : `Showing ${start}–${end} of ${filteredCount}`}
        </p>
        <select
          aria-label="Items per page"
          value={isAll ? 'all' : perPage}
          onChange={(e) => onPerPage(e.target.value === 'all' ? Infinity : Number(e.target.value))}
          className="rounded-md border border-input bg-background px-2 py-1 text-xs text-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
        >
          {perPageOptions.map((n) => (
            <option key={n} value={n}>
              {n} / page
            </option>
          ))}
          <option value="all">All</option>
        </select>
      </div>
      {!isAll && (
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label="Previous page"
            disabled={page <= 1}
            onClick={() => onPage(page - 1)}
            className="rounded-md border border-border p-2 text-muted-foreground hover:bg-accent hover:text-accent-foreground disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <span className="min-w-[3rem] text-center text-sm text-foreground">
            {page} / {totalPages}
          </span>
          <button
            type="button"
            aria-label="Next page"
            disabled={page >= totalPages}
            onClick={() => onPage(page + 1)}
            className="rounded-md border border-border p-2 text-muted-foreground hover:bg-accent hover:text-accent-foreground disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  )
}
