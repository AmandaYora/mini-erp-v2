import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  Badge,
  Button,
  ConfirmDialog,
  DataTable,
  FormField,
  Modal,
  PageHeader,
  SectionCard,
  SelectInput,
  TextArea,
  TextInput,
} from "@/shared/components/ui";
import type { DataTableColumn } from "@/shared/components/ui";
import { Notice } from "@/shared/components/feedback/notice";
import { formatDateTime } from "@/shared/lib/format";
import { toApiError } from "@/shared/services/http-client";
import { toast } from "@/shared/stores/toast.store";
import { useAsyncData } from "@/shared/hooks/use-async-data";
import { useAuthStore } from "@/modules/auth/stores/auth.store";
import { assistantService } from "@/modules/assistant/services/assistant.service";
import {
  authorizationSchema,
  chatMessageSchema,
} from "@/modules/assistant/schemas/assistant.schema";
import type {
  AssistantAuthorization,
  AssistantChatResult,
} from "@/modules/assistant/types";

function accessLevelLabel(v: string): string {
  if (v === "owner") return "Pemilik";
  if (v === "authorized_party") return "Pihak Terotorisasi";
  return v || "-";
}

function authStatusLabel(v: string): string {
  const s = (v ?? "").toLowerCase();
  if (s === "active") return "Aktif";
  if (s === "revoked") return "Dicabut";
  return v || "-";
}

function authStatusTone(v: string): "success" | "danger" | "neutral" {
  const s = (v ?? "").toLowerCase();
  if (s === "active") return "success";
  if (s === "revoked") return "danger";
  return "neutral";
}

export default function SetupPage() {
  const can = useAuthStore((s) => s.can);
  const canManage = can("assistant.manage");
  const canView = can("assistant.view");

  // ---- Kanal WhatsApp ----
  const {
    data: channel,
    loading: channelLoading,
    reload: reloadChannel,
  } = useAsyncData(() => assistantService.getChannel(), []);
  const [channelBusy, setChannelBusy] = useState<
    "connect" | "disconnect" | "reset" | null
  >(null);
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
    reload: reloadAuths,
  } = useAsyncData(
    () =>
      assistantService
        .listAuthorizations(includeRevoked)
        .then((rows) => rows ?? [])
        .catch((err) => {
          toast.danger(
            "Gagal memuat nomor terotorisasi",
            toApiError(err).message,
          );
          throw err;
        }),
    [includeRevoked],
  );
  const auths = authsData ?? [];
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<AssistantAuthorization | null>(null);
  const [phone, setPhone] = useState("");
  const [authName, setAuthName] = useState("");
  const [accessLevel, setAccessLevel] = useState<
    "owner" | "authorized_party"
  >("authorized_party");
  const [isPrimaryOwner, setIsPrimaryOwner] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [formSaving, setFormSaving] = useState(false);
  const [revokeTarget, setRevokeTarget] =
    useState<AssistantAuthorization | null>(null);
  const [revoking, setRevoking] = useState(false);

  function openCreate() {
    setEditing(null);
    setPhone("");
    setAuthName("");
    setAccessLevel("authorized_party");
    setIsPrimaryOwner(false);
    setFormErrors({});
    setModalOpen(true);
  }

  function openEdit(row: AssistantAuthorization) {
    setEditing(row);
    setPhone(row.phone ?? "");
    setAuthName(row.name ?? "");
    setAccessLevel(
      row.accessLevel === "owner" ? "owner" : "authorized_party",
    );
    setIsPrimaryOwner(row.isPrimaryOwner);
    setFormErrors({});
    setModalOpen(true);
  }

  function closeModal() {
    if (!formSaving) {
      setModalOpen(false);
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
      setModalOpen(false);
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
    { header: "Telepon", render: (r) => r.phone },
    { header: "Nama", render: (r) => r.name || "-" },
    { header: "Tingkat", render: (r) => accessLevelLabel(r.accessLevel) },
    {
      header: "Status",
      render: (r) => (
        <Badge tone={authStatusTone(r.status)}>{authStatusLabel(r.status)}</Badge>
      ),
    },
    {
      header: "Utama",
      render: (r) =>
        r.isPrimaryOwner ? (
          <Badge tone="success">Pemilik Utama</Badge>
        ) : (
          <span className="text-muted text-sm">-</span>
        ),
    },
    {
      header: "Terakhir Terlihat",
      render: (r) => formatDateTime(r.lastSeenAt),
    },
  ];

  if (canManage) {
    authColumns.push({
      header: "Aksi",
      align: "right",
      render: (r) => (
        <div className="flex justify-end gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => openEdit(r)}
            disabled={revoking}
          >
            Ubah
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={() => setRevokeTarget(r)}
            disabled={revoking || r.status.toLowerCase() === "revoked"}
          >
            Cabut
          </Button>
        </div>
      ),
    });
  }

  // ---- Konsol uji ----
  const [chatMessage, setChatMessage] = useState("");
  const [chatError, setChatError] = useState<string | null>(null);
  const [chatSending, setChatSending] = useState(false);
  const [chatResult, setChatResult] = useState<AssistantChatResult | null>(
    null,
  );

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
        branchId: 0,
      });
      setChatResult(res.data);
    } catch (err) {
      toast.danger("Gagal mengirim pesan uji", toApiError(err).message);
    } finally {
      setChatSending(false);
    }
  }

  const busy = channelBusy !== null;

  return (
    <div>
      <PageHeader
        eyebrow="Asisten"
        title="Penyiapan"
        description="Status kanal WhatsApp, nomor terotorisasi, dan konsol uji balasan."
      />

      <div className="space-y-6">
        <SectionCard
          title="Kanal WhatsApp"
          description="Tautkan nomor WhatsApp perusahaan agar asisten bisa menerima pesan."
          actions={
            canManage ? (
              <>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => void handleConnect()}
                  disabled={channelLoading || busy || channel?.connected === true}
                >
                  {channelBusy === "connect" ? "Menghubungkan…" : "Hubungkan"}
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setConfirmDisconnect(true)}
                  disabled={
                    channelLoading || busy || channel?.connected !== true
                  }
                >
                  Putuskan
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setConfirmReset(true)}
                  disabled={channelLoading || busy}
                >
                  {channelBusy === "reset" ? "Mereset…" : "Reset Sesi"}
                </Button>
              </>
            ) : undefined
          }
        >
          {channelLoading && !channel ? (
            <p className="text-muted py-2 text-sm">Memuat status kanal…</p>
          ) : channel ? (
            <div>
              <div className="flex flex-wrap items-center gap-3">
                <Badge tone={channel.connected ? "success" : "danger"}>
                  {channel.connected ? "Terhubung" : "Terputus"}
                </Badge>
                {channel.phone && (
                  <span className="text-sm font-medium text-ink">
                    {channel.phone}
                  </span>
                )}
                <span className="text-muted text-xs">
                  Diperbarui {formatDateTime(channel.updatedAt)}
                </span>
              </div>

              {!channel.connected && channel.qrDataUrl && (
                <div className="mt-4 flex flex-wrap items-start gap-4">
                  <img
                    src={channel.qrDataUrl}
                    alt="QR tautan WhatsApp"
                    width={220}
                    height={220}
                    style={{ width: 220, height: 220 }}
                    className="rounded-md border border-hairline"
                  />
                  <p className="text-muted max-w-[320px] text-sm">
                    Buka WhatsApp perusahaan &gt; Perangkat Tertaut &gt;
                    Tautkan Perangkat, lalu pindai kode di samping. Halaman
                    ini memeriksa status otomatis tiap 5 detik selama terputus.
                  </p>
                </div>
              )}

              {channel.lastError && (
                <div className="mt-4">
                  <Notice tone="danger" title="Kanal gagal terhubung">
                    {channel.lastError}
                  </Notice>
                </div>
              )}
            </div>
          ) : (
            <p className="text-muted text-sm">
              Status kanal belum tersedia. Coba muat ulang halaman.
            </p>
          )}
        </SectionCard>

        <SectionCard
          title="Nomor Terotorisasi"
          description="Nomor WhatsApp yang boleh memakai asisten. Pemilik utama adalah penanggung jawab kanal."
          actions={
            canManage ? (
              <Button size="sm" onClick={openCreate} disabled={authsLoading}>
                Tambah Nomor
              </Button>
            ) : undefined
          }
        >
          <label className="mb-4 flex cursor-pointer items-center gap-2 text-sm text-ink">
            <input
              type="checkbox"
              className="accent-brand h-4 w-4"
              checked={includeRevoked}
              onChange={(e) => setIncludeRevoked(e.target.checked)}
            />
            Tampilkan nomor yang dicabut
          </label>
          {authsLoading ? (
            <p className="text-muted py-2 text-sm">
              Memuat nomor terotorisasi…
            </p>
          ) : (
            <DataTable<AssistantAuthorization>
              columns={authColumns}
              rows={auths}
              rowKey={(r) => r.id}
              emptyTitle="Belum ada nomor terotorisasi"
              emptyDescription="Tambah nomor WhatsApp pemilik atau pihak terotorisasi untuk mulai memakai asisten."
            />
          )}
        </SectionCard>

        <SectionCard
          title="Konsol Uji"
          description="Kirim pesan uji sebagai staf internal (cabang 0) tanpa lewat WhatsApp."
        >
          {!canView ? (
            <Notice tone="warning" title="Akses terbatas">
              Anda tidak memiliki izin assistant.view untuk memakai konsol uji.
            </Notice>
          ) : (
            <div>
              <form
                id="assistant-chat-form"
                onSubmit={(e) => void handleChatSubmit(e)}
              >
                <FormField
                  label="Pesan"
                  required
                  errorText={chatError ?? undefined}
                >
                  <TextArea
                    rows={3}
                    value={chatMessage}
                    onChange={(e) => setChatMessage(e.target.value)}
                    placeholder="cth: berapa stok gula hari ini?"
                    disabled={chatSending}
                  />
                </FormField>
                <div className="mt-3 flex justify-end">
                  <Button
                    type="submit"
                    form="assistant-chat-form"
                    disabled={chatSending}
                  >
                    {chatSending ? "Mengirim…" : "Kirim"}
                  </Button>
                </div>
              </form>
              {chatResult && (
                <div className="mt-4 rounded-md border border-hairline bg-surface-subtle p-4">
                  <p className="text-sm whitespace-pre-wrap text-ink">
                    {chatResult.answer}
                  </p>
                  <p className="text-muted mt-2 text-xs">
                    Intent: {chatResult.intent || "-"} • Mode:{" "}
                    {chatResult.mode || "-"} • Cabang:{" "}
                    {chatResult.branchName || "-"}
                  </p>
                </div>
              )}
            </div>
          )}
        </SectionCard>
      </div>

      <Modal
        open={modalOpen}
        title={editing ? "Ubah Nomor Terotorisasi" : "Tambah Nomor Terotorisasi"}
        description="Nomor yang dicabut tidak bisa memakai asisten sampai ditambahkan kembali."
        size="md"
        onClose={closeModal}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={closeModal}
              disabled={formSaving}
            >
              Batal
            </Button>
            <Button
              type="submit"
              form="assistant-auth-form"
              disabled={formSaving}
            >
              {formSaving ? "Menyimpan…" : "Simpan"}
            </Button>
          </>
        }
      >
        <form
          id="assistant-auth-form"
          onSubmit={(e) => void handleAuthSubmit(e)}
        >
          <fieldset disabled={formSaving}>
            <div className="grid gap-4">
              <FormField
                label="Nomor Telepon"
                required
                helperText="8-15 digit angka, tanpa spasi atau tanda +."
                errorText={formErrors.phone}
              >
                <TextInput
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="cth: 081234567890"
                  inputMode="numeric"
                />
              </FormField>
              <FormField
                label="Nama"
                required
                errorText={formErrors.name}
              >
                <TextInput
                  value={authName}
                  onChange={(e) => setAuthName(e.target.value)}
                  placeholder="cth: Budi Pemilik"
                />
              </FormField>
              <FormField
                label="Tingkat Akses"
                required
                errorText={formErrors.accessLevel}
              >
                <SelectInput
                  value={accessLevel}
                  onChange={(e) =>
                    setAccessLevel(
                      e.target.value as "owner" | "authorized_party",
                    )
                  }
                >
                  <option value="owner">Pemilik</option>
                  <option value="authorized_party">Pihak Terotorisasi</option>
                </SelectInput>
              </FormField>
              <label className="flex cursor-pointer items-center gap-2 text-sm text-ink">
                <input
                  type="checkbox"
                  className="accent-brand h-4 w-4"
                  checked={isPrimaryOwner}
                  onChange={(e) => setIsPrimaryOwner(e.target.checked)}
                />
                Pemilik utama kanal
              </label>
            </div>
          </fieldset>
        </form>
      </Modal>

      <ConfirmDialog
        open={revokeTarget !== null}
        title="Cabut Otorisasi"
        message={`Cabut akses nomor "${revokeTarget?.phone}"? Nomor tidak bisa memakai asisten sampai ditambahkan kembali.`}
        confirmLabel="Cabut"
        tone="danger"
        onConfirm={() => void handleRevokeConfirm()}
        onCancel={() => setRevokeTarget(null)}
      />

      <ConfirmDialog
        open={confirmDisconnect}
        title="Putuskan Kanal"
        message="Putuskan tautan WhatsApp perusahaan? Asisten berhenti menerima pesan sampai dihubungkan kembali."
        confirmLabel="Putuskan"
        tone="danger"
        onConfirm={() => void handleDisconnectConfirm()}
        onCancel={() => setConfirmDisconnect(false)}
      />

      <ConfirmDialog
        open={confirmReset}
        title="Reset Sesi Kanal"
        message="Reset sesi kanal WhatsApp? Tautan perangkat ikut terputus dan perlu dipindai ulang."
        confirmLabel="Reset"
        tone="danger"
        onConfirm={() => void handleResetConfirm()}
        onCancel={() => setConfirmReset(false)}
      />
    </div>
  );
}
