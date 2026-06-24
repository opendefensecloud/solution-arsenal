import { renderHook, act } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { useListState } from "./useListState";

describe("useListState", () => {
  it("resets page to 1 when search changes", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setPage(3); });
    act(() => { result.current.setSearch("foo"); });
    expect(result.current.page).toBe(1);
  });

  it("resets page to 1 when perPage changes", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setPage(3); });
    act(() => { result.current.setPerPage(10); });
    expect(result.current.page).toBe(1);
  });

  it("toggleSort: same field flips direction", () => {
    const { result } = renderHook(() => useListState());
    expect(result.current.sortDir).toBe("asc");
    act(() => { result.current.toggleSort("name"); });
    expect(result.current.sortDir).toBe("desc");
  });

  it("toggleSort: different field resets to asc and updates sortField", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.toggleSort("age"); });
    expect(result.current.sortField).toBe("age");
    expect(result.current.sortDir).toBe("asc");
  });

  it("returns tile perPageOptions when tileView is true", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setTileView(true); });
    expect(result.current.perPageOptions).toEqual([6, 9, 12]);
  });

  it("switches perPage to 9 when entering tile view", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setTileView(true); });
    expect(result.current.perPage).toBe(9);
  });

  it("switches perPage to 5 when leaving tile view", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setTileView(true); });
    act(() => { result.current.setTileView(false); });
    expect(result.current.perPage).toBe(5);
  });

  it("toggleSort resets page to 1", () => {
    const { result } = renderHook(() => useListState());
    act(() => { result.current.setPage(4); });
    act(() => { result.current.toggleSort("age"); });
    expect(result.current.page).toBe(1);
  });
});
