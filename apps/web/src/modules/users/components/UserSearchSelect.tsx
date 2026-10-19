import { useCallback, useEffect, useState } from "react";
import {
  AsyncSearchSelect,
  type AsyncSearchLoadResult,
  type AsyncSearchOption,
} from "@/shared/components/ui";
import { userService } from "@/modules/users/services/user.service";
import type { UserItem } from "@/modules/users/types";

export interface UserSearchSelectProps {
  id?: string;
  value: number | null;
  onChange: (value: number | null, user?: UserItem) => void;
  disabled?: boolean;
  placeholder?: string;
  limit?: number;
}

function formatUserOption(user: UserItem): AsyncSearchOption {
  return {
    value: user.id,
    label: user.fullName || user.username,
    description: [user.username, user.email].filter(Boolean).join(" · ") || undefined,
  };
}

/**
 * UserSearchSelect — pemilih pengguna dengan pencarian SERVER-SIDE
 * (GET /users?search, butuh users.view; pemanggil menyembunyikannya bila
 * izin tidak ada).
 */
export function UserSearchSelect({
  id,
  value,
  onChange,
  disabled,
  placeholder = "Semua pengguna",
  limit = 20,
}: UserSearchSelectProps) {
  const [resolved, setResolved] = useState<AsyncSearchOption | null>(null);
  if ((value === null || value === undefined) && resolved !== null) setResolved(null);

  const loadOptions = useCallback(
    async (search: string, signal?: AbortSignal): Promise<AsyncSearchLoadResult> => {
      const res = await userService.list({ search, page: 1, limit }, signal);
      return {
        options: res.items.map(formatUserOption),
        hasMore: res.meta.total > res.items.length,
      };
    },
    [limit],
  );

  // Nilai awal dari URL tanpa pilihan: ambil satu pengguna agar namanya tampil.
  useEffect(() => {
    if (!value || resolved?.value === value) return;
    let active = true;
    userService
      .get(value)
      .then((user) => {
        if (active) setResolved(formatUserOption(user));
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [value, resolved]);

  return (
    <AsyncSearchSelect
      id={id}
      value={value}
      disabled={disabled}
      allowClear
      placeholder={placeholder}
      searchPlaceholder="Ketik nama, username, atau email…"
      selectedOption={resolved}
      loadOptions={loadOptions}
      onChange={(next, option) => {
        setResolved(option ?? null);
        onChange(
          typeof next === "number" ? next : next === null ? null : Number(next) || null,
        );
      }}
    />
  );
}
