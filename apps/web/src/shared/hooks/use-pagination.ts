import { useState } from "react";

export function usePagination(limit = 20) {
  const [page, setPage] = useState(1);

  const reset = () => setPage(1);

  return { page, limit, setPage, reset };
}
