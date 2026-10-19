import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Send } from "lucide-react";
import {
  Badge,
  Button,
  Checkbox,
  ConfirmDialog,
  DataTable,
  FormField,
  FormSheet,
  PageHeader,
  SectionCard,
  SelectInput,
  Skeleton,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  TextInput,
} from "@/shared/components/ui";
import type { DataTableColumn } from "@/shared/components/ui";
import { ActionMenu, Page, Stepper, useUrlTab } from "@/shared/components/patterns";
import type { ActionMenuItem, StepperStep } from "@/shared/components/patterns";
import { ErrorState } from "@/shared/components/feedback/error-state";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { BranchPicker } from "@/modules/assistant/components/BranchPicker";
import SimulateCard from "@/modules/assistant/components/SimulateCard";
import { assistantService } from "@/modules/assistant/services/assistant.service";
import {
  authorizationSchema,
  chatMessageSchema,
} from "@/modules/assistant/schemas/assistant.schema";
import type {
  AssistantAuthorization,
  AssistantChannel,
  AssistantChatResult,
} from "@/modules/assistant/types";

const TABS = ["nomor", "konsol"] as const;

const CONNECTION_STEPS: StepperStep[] = [
  { key: "idle", label: "Belum terhubung" },
  { key: "qr", label: "Pindai QR" },
  { key: "connected", label: "Terhubung" },
];

function connectionStep(channel: AssistantChannel): string {
  if (channel.connected) return "connected";
  return channel.qrDataUrl ? "qr" : "idle";
}

function accessLevelLabel(v: string): string {
  if (v === "owner") return "Pemilik";
  if (v === "authorized_party") return "Pihak terotorisasi";
  return v || "–";
}

function authStatusLabel(v: string): string {
  const s = (v ?? "").toLowerCase();
  if (s === "active") return "Aktif";
  if (s === "revoked") return "Dicabut";
  return v || "–";
}

function authStatusTone(v: string): "success" | "danger" | "neutral" {
  const s = (v ?? "").toLowerCase();
  if (s === "active") return "success";
  if (s === "revoked") return "danger";
  return "neutral";
}

interface ChatTurn {
  id: number;
  question: string;
  result: AssistantChatResult;
}

export default function SetupPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("assistant.manage");
  const canView = can("assistant.view");

  const [tab, setTab] = useUrlTab(TABS, "nomor");

  // ---- Kanal WhatsApp ----
  const {
    data: channel,
    loading: channelLoading,
    error: channelError,
    reload: reloadChannel,
  } = useAsyncData(() => assistantService.getChannel(), []);
  const [channelBusy, setChannelBusy] = useState<"connect" | "disconnect" | "reset" | null>(null);
  const [confirmDisconnect, setConfirmDisconnect] = useState(false);
  const [confirmReset, setConfirmReset] = useState(false);

  const disconnected = channel !== null && !channel.connected;

  // Selagi terputus, pantau status tiap 5 detik (berhenti saat terhubung).
  useEffect(() => {
    if (!disconnected) return;
    const timer = window.setInterval(() => {
      reloadChannel();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [disconnected, reloadChannel]);

  async function handleConnect() {
    setChannelBusy("connect");
    try {
      const res = await assistantService.connectChannel();
      reloadChannel();
      toast.fromServer(res.message, "Kanal dihubungkan");
    } catch (err) {
      toast.danger("Gagal menghubungkan kanal", toApiError(err).message);
    } finally {
      setChannelBusy(null);
    }
  }

  async function handleDisconnectConfirm() {
    setConfirmDisconnect(false);
    setChannelBusy("disconnect");
    try {
      const res = await assistantService.disconnectChannel();
      reloadChannel();
      toast.fromServer(res.message, "Kanal diputuskan");
    } catch (err) {
      toast.danger("Gagal memutuskan kanal", toApiError(err).message);
    } finally {
      setChannelBusy(null);
    }
  }

  async function handleResetConfirm() {
    setConfirmReset(false);
    setChannelBusy("reset");
    try {
      const res = await assistantService.resetChannel();
      reloadChannel();
      toast.fromServer(res.message, "Sesi kanal direset");
    } catch (err) {
      toast.danger("Gagal mereset sesi kanal", toApiError(err).message);
    } finally {
      setChannelBusy(null);
    }
  }

  // ---- Nomor terotorisasi ----
  const [includeRevoked, setIncludeRevoked] = useState(false);
  const {
    data: authsData,
    loading: authsLoading,
    error: authsError,
    reload: reloadAuths,
  } = useAsyncData(
    () => assistantService.listAuthorizations(includeRevoked).then((rows) => rows ?? []),
    [includeRevoked],
  );
  const auths = authsData ?? [];
  const [sheetOpen, setSheetOpen] = useState(false);
  const [editing, setEditing] = useState<AssistantAuthorization | null>(null);
  const [phone, setPhone] = useState("");
  const [authName, setAuthName] = useState("");
  const [accessLevel, setAccessLevel] = useState<"owner" | "authorized_party">("authorized_party");
  const [isPrimaryOwner, setIsPrimaryOwner] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [formSaving, setFormSaving] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<AssistantAuthorization | null>(null);
  const [revoking, setRevoking] = useState(false);

  function openCreate() {
    setEditing(null);
    setPhone("");
    setAuthName("");
    setAccessLevel("authorized_party");
    setIsPrimaryOwner(false);
    setFormErrors({});
    setSheetOpen(true);
  }

  function openEdit(row: AssistantAuthorization) {
    setEditing(row);
    setPhone(row.phone ?? "");
    setAuthName(row.name ?? "");
    setAccessLevel(row.accessLevel === "owner" ? "owner" : "authorized_party");
    setIsPrimaryOwner(row.isPrimaryOwner);
    setFormErrors({});
    setSheetOpen(true);
  }

  function closeSheet() {
    if (!formSaving) {
      setSheetOpen(false);
      setEditing(null);
    }
  }

  async function handleAuthSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = authorizationSchema.safeParse({
      phone,
      name: authName,
      accessLevel,
      isPrimaryOwner,
    });
    if (!parsed.success) {
      const errs: Record<string, string> = {};
      for (const issue of parsed.error.issues) {
        errs[String(issue.path[0] ?? "")] ??= issue.message;
      }
      setFormErrors(errs);
      return;
    }
    setFormErrors({});
    setFormSaving(true);
    try {
      if (editing) {
        const res = await assistantService.updateAuthorization(editing.id, parsed.data);
        toast.fromServer(res.message, "Nomor terotorisasi diubah");
      } else {
        const res = await assistantService.createAuthorization(parsed.data);
        toast.fromServer(res.message, "Nomor terotorisasi ditambahkan");
      }
      setSheetOpen(false);
      setEditing(null);
      reloadAuths();
    } catch (err) {
      const apiErr = toApiError(err);
      const next: Record<string, string> = {};
      for (const fe of apiErr.errors ?? []) {
        if (fe.field) next[fe.field] ??= fe.message;
      }
      setFormErrors(next);
      toast.danger("Gagal menyimpan nomor", apiErr.message);
    } finally {
      setFormSaving(false);
    }
  }

  async function handleRevokeConfirm() {
    if (!revokeTarget) return;
    const target = revokeTarget;
    setRevokeTarget(null);
    setRevoking(true);
    try {
      const res = await assistantService.revokeAuthorization(target.id);
      toast.fromServer(res.message, "Otorisasi nomor dicabut");
      reloadAuths();
    } catch (err) {
      toast.danger("Gagal mencabut otorisasi", toApiError(err).message);
    } finally {
      setRevoking(false);
    }
  }

  const authColumns: DataTableColumn<AssistantAuthorization>[] = [
    { header: "Telepon", render: (r) => <span className="font-mono text-sm">{r.phone}</span> },
    { header: "Nama", render: (r) => r.name || "–" },
    { header: "Tingkat", render: (r) => accessLevelLabel(r.accessLevel) },
    {
      header: "Status",
      render: (r) => <Badge tone={authStatusTone(r.status)}>{authStatusLabel(r.status)}</Badge>,
    },
    {
      header: "Utama",
      render: (r) =>
        r.isPrimaryOwner ? <Badge tone="info">Pemilik utama</Badge> : <span className="text-sm text-muted-foreground">–</span>,
    },
    { header: "Terakhir terlihat", render: (r) => formatDateTime(r.lastSeenAt) },
  ];

  if (canManage) {
    authColumns.push({
      header: "Aksi",
      align: "right",
      width: 64,
      render: (r) => {
        const items: ActionMenuItem[] = [
          { key: "edit", label: "Ubah", onSelect: () => openEdit(r), disabled: revoking },
          {
            key: "revoke",
            label: "Cabut otorisasi",
            destructive: true,
            onSelect: () => setRevokeTarget(r),
            disabled: revoking || r.status.toLowerCase() === "revoked",
          },
        ];
        return (
          // Menu dirender di portal; klik itemnya menggelembung ke <tr> lewat pohon React.
          <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
            <ActionMenu items={items} label={`Aksi untuk ${r.phone}`} variant="ghost" />
          </div>
        );
      },
    });
  }

  // ---- Konsol uji (ala chat) ----
  const [chatMessage, setChatMessage] = useState("");
  const [chatBranch, setChatBranch] = useState("0");
  const [chatError, setChatError] = useState<string | null>(null);
  const [chatSending, setChatSending] = useState(false);
  const [turns, setTurns] = useState<ChatTurn[]>([]);
  const turnId = useRef(1);
  const endRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [turns.length]);

  async function handleChatSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const parsed = chatMessageSchema.safeParse({ message: chatMessage });
    if (!parsed.success) {
      setChatError(parsed.error.issues[0]?.message ?? "Pesan tidak valid");
      return;
    }
    setChatError(null);
    setChatSending(true);
    try {
      const res = await assistantService.chat({
        message: parsed.data.message,
        branchId: Number(chatBranch) || 0,
      });
      setTurns((prev) => [...prev, { id: turnId.current++, question: parsed.data.message, result: res.data }]);
      setChatMessage("");
    } catch (err) {
      toast.danger("Gagal mengirim pesan uji", toApiError(err).message);
    } finally {
      setChatSending(false);
    }
  }

  const busy = channelBusy !== null;

  const headerItems: ActionMenuItem[] = [
    {
      key: "disconnect",
      label: "Putuskan kanal",
      destructive: true,
      onSelect: () => setConfirmDisconnect(true),
      disabled: channelLoading || busy || channel?.connected !== true,
    },
  ];
  headerItems.unshift({
    key: "reset",
    label: channelBusy === "reset" ? "Mereset…" : "Reset sesi",
    onSelect: () => setConfirmReset(true),
    disabled: channelLoading || busy,
  });

  return (
    <Page>
      <PageHeader
        title="Penyiapan asisten"
        description="Status kanal WhatsApp, nomor terotorisasi, dan konsol uji balasan."
        actions={
          canManage ? (
            <>
              <ActionMenu items={headerItems} label="Aksi kanal" />
              <Button
                onClick={() => void handleConnect()}
                loading={channelBusy === "connect"}
                disabled={channelLoading || busy || channel?.connected === true}
              >
                {channelBusy === "connect" ? "Menghubungkan…" : "Hubungkan"}
              </Button>
            </>
          ) : undefined
        }
      />

      <SectionCard
        title="Kanal WhatsApp"
        description="Tautkan nomor WhatsApp perusahaan agar asisten bisa menerima pesan."
      >
        {channelError && !channel ? (
          <ErrorState message={channelError} onRetry={reloadChannel} />
        ) : channelLoading && !channel ? (
          <div role="status" aria-busy="true" aria-label="Memuat status kanal" className="space-y-3">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-4 w-48" />
          </div>
        ) : channel ? (
          <div className="space-y-5">
            <Stepper steps={CONNECTION_STEPS} current={connectionStep(channel)} />
            <div className="flex flex-wrap items-center gap-3">
              <Badge tone={channel.connected ? "success" : "danger"}>
                {channel.connected ? "Terhubung" : "Terputus"}
              </Badge>
              {channel.phone && <span className="text-sm font-medium text-foreground">{channel.phone}</span>}
              <span className="text-xs text-muted-foreground">Diperbarui {formatDateTime(channel.updatedAt)}</span>
            </div>

            {!channel.connected && channel.qrDataUrl && (
              <div className="flex flex-wrap items-start gap-4">
                <img
                  src={channel.qrDataUrl}
                  alt="QR tautan WhatsApp"
                  width={220}
                  height={220}
                  className="size-56 rounded-md border border-border"
                />
                <p className="max-w-xs text-sm text-muted-foreground">
                  Buka WhatsApp perusahaan &gt; Perangkat Tertaut &gt; Tautkan Perangkat, lalu pindai kode di samping.
                  Halaman ini memeriksa status otomatis tiap 5 detik selama terputus.
                </p>
              </div>
            )}

            {channel.lastError && (
              <Notice tone="danger" title="Kanal gagal terhubung">
                {channel.lastError}
              </Notice>
            )}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">Status kanal belum tersedia. Coba muat ulang halaman.</p>
        )}
      </SectionCard>

      <Tabs value={tab} onValueChange={(v) => setTab(v as (typeof TABS)[number])}>
        <TabsList>
          <TabsTrigger value="nomor">Nomor diizinkan</TabsTrigger>
          <TabsTrigger value="konsol">Konsol uji</TabsTrigger>
        </TabsList>

        <TabsContent value="nomor" className="mt-4">
          <SectionCard
            flush
            title="Nomor terotorisasi"
            description="Nomor WhatsApp yang boleh memakai asisten. Pemilik utama adalah penanggung jawab kanal."
            actions={
              canManage ? (
                <Button size="sm" onClick={openCreate}>
                  Tambah nomor
                </Button>
              ) : undefined
            }
          >
            <label
              htmlFor="assistant-include-revoked"
              className="flex cursor-pointer items-center gap-2.5 border-b border-border px-5 py-3 text-sm text-foreground"
            >
              <Checkbox
                id="assistant-include-revoked"
                checked={includeRevoked}
                onCheckedChange={(v) => setIncludeRevoked(v === true)}
              />
              Tampilkan nomor yang dicabut
            </label>
            <DataTable<AssistantAuthorization>
              columns={authColumns}
              rows={authsData ? auths : null}
              rowKey={(r) => r.id}
              loading={authsLoading}
              error={authsError}
              onRetry={reloadAuths}
              emptyTitle="Belum ada nomor terotorisasi"
              emptyDescription="Tambah nomor WhatsApp pemilik atau pihak terotorisasi untuk mulai memakai asisten."
              emptyAction={canManage ? <Button onClick={openCreate}>Tambah nomor</Button> : undefined}
            />
          </SectionCard>
        </TabsContent>

        <TabsContent value="konsol" className="mt-4 space-y-6">
          <SectionCard
            title="Konsol uji"
            description="Kirim pesan uji sebagai staf internal tanpa lewat WhatsApp. Pilih cabang, atau sebut nama cabang di pesan."
          >
            {!canView ? (
              <Notice tone="warning" title="Akses terbatas">
                Anda tidak memiliki izin assistant.view untuk memakai konsol uji.
              </Notice>
            ) : (
              <div className="flex flex-col gap-4">
                <div
                  role="log"
                  aria-label="Percakapan uji"
                  aria-live="polite"
                  className="flex max-h-96 min-h-40 flex-col gap-4 overflow-y-auto rounded-lg border border-border bg-background p-4"
                >
                  {turns.length === 0 && !chatSending && (
                    <p className="m-auto text-sm text-muted-foreground">
                      Belum ada percakapan. Coba tanya, mis. “berapa stok gula hari ini?”
                    </p>
                  )}
                  {turns.map((t) => (
                    <div key={t.id} className="flex flex-col gap-2">
                      <p className="max-w-xl self-end rounded-lg bg-primary px-3 py-2 text-sm whitespace-pre-wrap text-primary-foreground">
                        {t.question}
                      </p>
                      <div className="max-w-xl self-start rounded-lg border border-border bg-card px-3 py-2">
                        <p className="text-sm whitespace-pre-wrap text-foreground">{t.result.answer}</p>
                        <p className="mt-1.5 text-xs text-muted-foreground">
                          Intent: {t.result.intent || "–"} · Mode: {t.result.mode || "–"} · Cabang:{" "}
                          {t.result.branchName || "–"}
                        </p>
                      </div>
                    </div>
                  ))}
                  {chatSending && (
                    <p role="status" className="self-start text-sm text-muted-foreground">
                      Asisten sedang menjawab…
                    </p>
                  )}
                  <div ref={endRef} />
                </div>

                <form id="assistant-chat-form" onSubmit={(e) => void handleChatSubmit(e)} className="flex flex-col gap-4">
                  <BranchPicker id="assistant-chat-branch" value={chatBranch} onChange={setChatBranch} disabled={chatSending} />
                  <FormField label="Pesan" htmlFor="assistant-chat-message" required errorText={chatError ?? undefined}>
                    <div className="flex items-start gap-2">
                      <div className="flex-1">
                        <TextInput
                          id="assistant-chat-message"
                          value={chatMessage}
                          onChange={(e) => setChatMessage(e.target.value)}
                          placeholder="cth: berapa stok gula hari ini?"
                          disabled={chatSending}
                          autoComplete="off"
                        />
                      </div>
                      <Button type="submit" loading={chatSending}>
                        <Send aria-hidden /> {chatSending ? "Mengirim…" : "Kirim"}
                      </Button>
                    </div>
                  </FormField>
                </form>
              </div>
            )}
          </SectionCard>

          {can("assistant.simulate") && <SimulateCard />}
        </TabsContent>
      </Tabs>

      <FormSheet
        open={sheetOpen}
        title={editing ? "Ubah nomor terotorisasi" : "Tambah nomor terotorisasi"}
        description="Nomor yang dicabut tidak bisa memakai asisten sampai ditambahkan kembali."
        onClose={closeSheet}
        actions={
          <>
            <Button variant="secondary" onClick={closeSheet} disabled={formSaving}>
              Batal
            </Button>
            <Button type="submit" form="assistant-auth-form" loading={formSaving}>
              {formSaving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form id="assistant-auth-form" onSubmit={(e) => void handleAuthSubmit(e)}>
          <fieldset disabled={formSaving} className="grid gap-4">
            <FormField
              label="Nomor telepon"
              htmlFor="auth-phone"
              required
              helperText="8-15 digit angka, tanpa spasi atau tanda +."
              errorText={formErrors.phone}
            >
              <TextInput
                id="auth-phone"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="cth: 081234567890"
                inputMode="numeric"
              />
            </FormField>
            <FormField label="Nama" htmlFor="auth-name" required errorText={formErrors.name}>
              <TextInput
                id="auth-name"
                value={authName}
                onChange={(e) => setAuthName(e.target.value)}
                placeholder="cth: Budi Pemilik"
              />
            </FormField>
            <FormField label="Tingkat akses" htmlFor="auth-level" required errorText={formErrors.accessLevel}>
              <SelectInput
                id="auth-level"
                value={accessLevel}
                onChange={(e) => setAccessLevel(e.target.value as "owner" | "authorized_party")}
              >
                <option value="owner">Pemilik</option>
                <option value="authorized_party">Pihak terotorisasi</option>
              </SelectInput>
            </FormField>
            <label htmlFor="auth-primary" className="flex cursor-pointer items-center gap-2.5 text-sm text-foreground">
              <Checkbox id="auth-primary" checked={isPrimaryOwner} onCheckedChange={(v) => setIsPrimaryOwner(v === true)} />
              Pemilik utama kanal
            </label>
          </fieldset>
        </form>
      </FormSheet>

      <ConfirmDialog
        open={revokeTarget !== null}
        title="Cabut otorisasi"
        message={`Cabut akses nomor "${revokeTarget?.phone}"? Nomor tidak bisa memakai asisten sampai ditambahkan kembali.`}
        confirmLabel="Cabut"
        tone="danger"
        onConfirm={() => void handleRevokeConfirm()}
        onCancel={() => setRevokeTarget(null)}
      />

      <ConfirmDialog
        open={confirmDisconnect}
        title="Putuskan kanal"
        message="Putuskan tautan WhatsApp perusahaan? Asisten berhenti menerima pesan sampai dihubungkan kembali."
        confirmLabel="Putuskan"
        tone="danger"
        onConfirm={() => void handleDisconnectConfirm()}
        onCancel={() => setConfirmDisconnect(false)}
      />

      <ConfirmDialog
        open={confirmReset}
        title="Reset sesi kanal"
        message="Reset sesi kanal WhatsApp? Tautan perangkat ikut terputus dan perlu dipindai ulang."
        confirmLabel="Reset"
        tone="danger"
        onConfirm={() => void handleResetConfirm()}
        onCancel={() => setConfirmReset(false)}
      />
    </Page>
  );
}
