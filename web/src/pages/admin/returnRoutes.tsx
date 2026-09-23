import Loading from "@/components/loading";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button, Card, Dialog, Flex, Switch, Tabs, Text, TextField } from "@radix-ui/themes";
import { GripVertical, RefreshCw, Save, Trash2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

type Target = {
  id: string;
  carrier: "telecom" | "unicom" | "mobile";
  region: string;
  host: string;
  ip_family: string;
  protocol: string;
  enabled: boolean;
  probe_enabled: boolean;
};

type ProbeLog = {
  id: number;
  task_id: string;
  client_id: string;
  client_name: string;
  carrier: Target["carrier"];
  target_host: string;
  route_type: string;
  confidence: string;
  reason: string;
  ok: boolean;
  tested_at: string;
};

type LogResponse = { logs: ProbeLog[]; total: number };
type ServerRoute = { uuid: string; name: string; return_routes?: Array<{ carrier: string; route_type: string; stale: boolean; tested_at: string }> };
type SettingsInfo = { schedule_time: string; storage_bytes: number; log_count: number };

const formatBytes = (value: number) => {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(2)} MB`;
};

export default function ReturnRoutes() {
  const { t } = useTranslation();
  const { call } = useRPC2Call();
  const [targets, setTargets] = useState<Target[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [logs, setLogs] = useState<ProbeLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [logsLoading, setLogsLoading] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [clearOpen, setClearOpen] = useState(false);
  const [featureSaving, setFeatureSaving] = useState(false);
  const [schedule, setSchedule] = useState("04:20");
  const [storageBytes, setStorageBytes] = useState(0);
  const [scheduleSaving, setScheduleSaving] = useState(false);
  const [servers, setServers] = useState<ServerRoute[]>([]);
  const [dragId, setDragId] = useState<string | null>(null);
  const pageSize = 20;

  const load = useCallback(async () => {
    try {
      const result = await call<undefined, Target[]>("admin:listReturnRouteTargets");
      setTargets(result);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [call]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => { fetch("/api/admin/client/list").then((r) => r.json()).then((value) => setServers(Array.isArray(value) ? value : [])).catch(() => undefined); }, []);

  useEffect(() => {
    void call<undefined, SettingsInfo>("admin:getReturnRouteSettings").then((value) => {
      setSchedule(value.schedule_time || "04:20");
      setStorageBytes(value.storage_bytes || 0);
    }).catch(() => undefined);
  }, [call]);

  const saveSchedule = async () => {
    setScheduleSaving(true);
    try {
      const value = await call<{ time: string }, { time: string }>("admin:setReturnRouteSchedule", { time: schedule });
      setSchedule(value.time);
      toast.success(t("returnRoute.scheduleSaved"));
    } catch (err) { toast.error(err instanceof Error ? err.message : String(err)); }
    finally { setScheduleSaving(false); }
  };

  const loadLogs = useCallback(async () => {
    setLogsLoading(true);
    try {
      const result = await call<{ limit: number; offset: number }, LogResponse>("admin:listReturnRouteLogs", { limit: pageSize, offset: page * pageSize });
      setLogs(result.logs ?? []);
      setTotal(result.total ?? 0);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setLogsLoading(false);
    }
  }, [call, page]);

  useEffect(() => { void loadLogs(); }, [loadLogs]);

  const clearLogs = async () => {
    setClearing(true);
    try {
      await call<{ confirm: boolean }, { cleared: boolean }>("admin:clearReturnRouteLogs", { confirm: true });
      setClearOpen(false);
      setPage(0);
      setLogs([]);
      setTotal(0);
      toast.success(t("returnRoute.logsCleared"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setClearing(false);
    }
  };

  const update = (id: string, patch: Partial<Target>) => {
    setTargets((current) => current?.map((item) => item.id === id ? { ...item, ...patch } : item) ?? null);
  };

  const setFeatureEnabled = async (enabled: boolean) => {
    setFeatureSaving(true);
    try {
      await call<{ enabled: boolean }, { enabled: boolean }>("admin:setReturnRouteEnabled", { enabled });
      setTargets((current) => current?.map((item) => ({ ...item, probe_enabled: enabled })) ?? null);
      toast.success(enabled ? t("returnRoute.enabledSaved") : t("returnRoute.disabledSaved"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setFeatureSaving(false);
    }
  };

  const save = async () => {
    if (!targets) return;
    setSaving(true);
    try {
      const result = await call<{ targets: Target[] }, Target[]>("admin:saveReturnRouteTargets", { targets });
      setTargets(result);
      toast.success(t("returnRoute.saved"));
    } catch (err) {
      toast.error(`${t("returnRoute.failed")}: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setSaving(false);
    }
  };

  const moveTarget = (overId: string) => {
    if (!dragId || dragId === overId || !targets) return;
    const next = [...targets]; const from = next.findIndex((x) => x.id === dragId); const to = next.findIndex((x) => x.id === overId);
    if (from < 0 || to < 0) return; const [item] = next.splice(from, 1); next.splice(to, 0, item); setTargets(next.map((x, index) => ({ ...x, sort_order: index }))); setDragId(null);
  };

  if (loading) return <Loading />;
  if (error) return <div className="p-4">{error}</div>;

  return (
    <Flex direction="column" gap="4" className="p-4">
      <div>
        <h1 className="text-2xl font-bold">{t("returnRoute.title")}</h1>
        <Text color="gray">{t("returnRoute.description")}</Text>
      </div>
      <Card>
        <Flex align="center" justify="between" gap="3" p="2">
          <div>
            <Text size="4" weight="bold" as="div">{t("returnRoute.featureSwitch")}</Text>
            <Text size="2" color="gray">{t("returnRoute.featureSwitchDescription")}</Text>
          </div>
          <Switch checked={Boolean(targets?.[0]?.probe_enabled)} disabled={featureSaving} onCheckedChange={(enabled) => void setFeatureEnabled(enabled)} />
        </Flex>
      </Card>
      <Card>
        <Flex align="end" gap="3" wrap="wrap" p="2">
          <label>
            <Text size="2" as="div" mb="1">{t("returnRoute.schedule")}</Text>
            <TextField.Root type="time" value={schedule} onChange={(event) => setSchedule(event.target.value)} />
          </label>
          <Button onClick={() => void saveSchedule()} disabled={scheduleSaving}>{t("returnRoute.saveSchedule")}</Button>
          <Text size="2" color="gray">{t("returnRoute.storage", { size: formatBytes(storageBytes) })}</Text>
        </Flex>
      </Card>
      <Tabs.Root defaultValue="targets">
        <Tabs.List><Tabs.Trigger value="targets">{t("returnRoute.targetView")}</Tabs.Trigger><Tabs.Trigger value="servers">{t("returnRoute.serverView")}</Tabs.Trigger></Tabs.List>
        <Tabs.Content value="targets" className="pt-3">
          <Card>
            <Table><TableHeader><TableRow><TableHead className="w-8"></TableHead><TableHead>{t("returnRoute.carrier")}</TableHead><TableHead>{t("returnRoute.region")}</TableHead><TableHead>{t("returnRoute.host")}</TableHead><TableHead>{t("returnRoute.enabled")}</TableHead></TableRow></TableHeader><TableBody>
              {targets?.map((target) => <TableRow key={target.id} draggable onDragStart={() => setDragId(target.id)} onDragOver={(event) => event.preventDefault()} onDrop={() => moveTarget(target.id)}>
                <TableCell><GripVertical size={15} className="cursor-grab text-gray-400" /></TableCell><TableCell>{t(`returnRoute.${target.carrier}`)}</TableCell><TableCell><TextField.Root size="1" value={target.region} onChange={(event) => update(target.id, { region: event.target.value })} /></TableCell><TableCell className="min-w-80"><TextField.Root size="1" value={target.host} onChange={(event) => update(target.id, { host: event.target.value })} /></TableCell><TableCell><Switch checked={target.enabled} onCheckedChange={(enabled) => update(target.id, { enabled })} /></TableCell>
              </TableRow>)}
            </TableBody></Table>
            <Flex justify="between" align="center" p="3"><Text size="2" color="gray">{t("returnRoute.note")}</Text><Button onClick={() => void save()} disabled={saving || !targets}><Save size={16} />{t("returnRoute.save")}</Button></Flex>
          </Card>
        </Tabs.Content>
        <Tabs.Content value="servers" className="pt-3"><Card><Table><TableHeader><TableRow><TableHead>{t("returnRoute.server")}</TableHead><TableHead>{t("returnRoute.telecom")}</TableHead><TableHead>{t("returnRoute.unicom")}</TableHead><TableHead>{t("returnRoute.mobile")}</TableHead></TableRow></TableHeader><TableBody>{servers.map((server) => <TableRow key={server.uuid}><TableCell>{server.name}</TableCell>{["telecom", "unicom", "mobile"].map((carrier) => { const route = server.return_routes?.find((item) => item.carrier === carrier); return <TableCell key={carrier}><span className={route?.stale ? "text-orange-600" : "text-green-700"}>{route?.route_type || "—"}</span></TableCell>; })}</TableRow>)}</TableBody></Table></Card></Tabs.Content>
      </Tabs.Root>
      <Card>
        <Flex direction="column" gap="3" p="2">
          <Flex align="center" justify="between" gap="3" wrap="wrap">
            <div>
              <Text size="4" weight="bold" as="div">{t("returnRoute.logs")}</Text>
              <Text size="2" color="gray">{t("returnRoute.logCount", { count: total })}</Text>
            </div>
            <Flex gap="2" wrap="wrap">
              <Button variant="soft" onClick={() => void loadLogs()} disabled={logsLoading}><RefreshCw size={16} />{t("returnRoute.refresh")}</Button>
              <Dialog.Root open={clearOpen} onOpenChange={setClearOpen}>
                <Dialog.Trigger><Button color="red" variant="soft" disabled={clearing}><Trash2 size={16} />{t("returnRoute.clearLogs")}</Button></Dialog.Trigger>
                <Dialog.Content maxWidth="440px">
                  <Dialog.Title>{t("returnRoute.clearLogs")}</Dialog.Title>
                  <Dialog.Description>{t("returnRoute.clearLogsConfirm")}</Dialog.Description>
                  <Flex justify="end" gap="2" mt="4">
                    <Dialog.Close><Button variant="soft" color="gray">{t("returnRoute.cancel")}</Button></Dialog.Close>
                    <Button color="red" onClick={() => void clearLogs()} disabled={clearing}>{t("returnRoute.clearLogs")}</Button>
                  </Flex>
                </Dialog.Content>
              </Dialog.Root>
            </Flex>
          </Flex>
          <Table>
            <TableHeader><TableRow>
              <TableHead>{t("returnRoute.time")}</TableHead>
              <TableHead>{t("returnRoute.server")}</TableHead>
              <TableHead>{t("returnRoute.carrier")}</TableHead>
              <TableHead>{t("returnRoute.host")}</TableHead>
              <TableHead>{t("returnRoute.result")}</TableHead>
              <TableHead>{t("returnRoute.detail")}</TableHead>
            </TableRow></TableHeader>
            <TableBody>
              {logs.map((log) => <TableRow key={log.id}>
                <TableCell>{new Date(log.tested_at).toLocaleString()}</TableCell>
                <TableCell>{log.client_name || log.client_id}</TableCell>
                <TableCell>{t(`returnRoute.${log.carrier}`)}</TableCell>
                <TableCell>{log.target_host}</TableCell>
                <TableCell>{log.ok ? log.route_type : t("returnRoute.probeFailed")}</TableCell>
                <TableCell className="whitespace-normal min-w-64 max-w-xl break-words">{log.reason}</TableCell>
              </TableRow>)}
              {!logs.length && <TableRow><TableCell colSpan={6} className="text-center">{logsLoading ? t("returnRoute.loading") : t("returnRoute.empty")}</TableCell></TableRow>}
            </TableBody>
          </Table>
          <Flex justify="end" align="center" gap="2">
            <Button variant="soft" disabled={page === 0 || logsLoading} onClick={() => setPage((value) => value - 1)}>{t("returnRoute.previous")}</Button>
            <Text size="2">{page + 1} / {Math.max(1, Math.ceil(total / pageSize))}</Text>
            <Button variant="soft" disabled={(page + 1) * pageSize >= total || logsLoading} onClick={() => setPage((value) => value + 1)}>{t("returnRoute.next")}</Button>
          </Flex>
        </Flex>
      </Card>
    </Flex>
  );
}
