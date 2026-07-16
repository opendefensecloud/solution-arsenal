import { useMemo, useState } from 'react'

export function useListState(options?: { defaultPerPage?: number }) {
  const [listDefaultPerPage] = useState(options?.defaultPerPage ?? 5)
  const [search, setSearchRaw] = useState('')
  const [sortField, setSortFieldRaw] = useState('name')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const [page, setPage] = useState(1)
  const [perPage, setPerPageRaw] = useState(listDefaultPerPage)
  const [tileView, setTileViewRaw] = useState(false)

  const perPageOptions = useMemo(() => (tileView ? [6, 9, 12] : [5, 10, 15, 25]), [tileView])

  function setSearch(v: string) {
    setSearchRaw(v)
    setPage(1)
  }

  function setPerPage(v: number) {
    setPerPageRaw(v)
    setPage(1)
  }

  function setTileView(v: boolean) {
    if (v === tileView) return
    setTileViewRaw(v)
    setPerPageRaw(v ? 9 : listDefaultPerPage)
    setPage(1)
  }

  function toggleSort(field: string) {
    if (field === sortField) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortFieldRaw(field)
      setSortDir('asc')
    }
    setPage(1)
  }

  return {
    search,
    setSearch,
    sortField,
    sortDir,
    toggleSort,
    page,
    setPage,
    perPage,
    setPerPage,
    tileView,
    setTileView,
    perPageOptions,
  }
}
