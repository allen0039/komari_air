import Loading from "@/components/loading";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Button, Card, Dialog, Flex, Switch, Text, TextField } from "@radix-ui/themes";
import { RefreshCw, Save, Trash2 } from "lucide-react";
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
      {targets?.map((target) => (
        <Card key={target.id}>
          <Flex direction="column" gap="3" p="2">
            <Flex align="center" justify="between">
              <Text size="4" weight="bold">{t(`returnRoute.${target.carrier}`)}</Text>
              <Flex align="center" gap="2">
                <Text size="2">{t("returnRoute.enabled")}</Text>
                <Switch checked={target.enabled} onCheckedChange={(enabled) => update(target.id, { enabled })} />
              </Flex>
            </Flex>
            <Flex gap="3" direction={{ initial: "column", sm: "row" }}>
              <label className="flex-1">
                <Text size="2" as="div" mb="1">{t("returnRoute.region")}</Text>
                <TextField.Root value={target.region} onChange={(event) => update(target.id, { region: event.target.value })} />
              </label>
              <label className="flex-[2]">
                <Text size="2" as="div" mb="1">{t("returnRoute.host")}</Text>
                <TextField.Root value={target.host} onChange={(event) => update(target.id, { host: event.target.value })} />
              </label>
            </Flex>
          </Flex>
        </Card>
      ))}
      <Text size="2" color="gray">{t("returnRoute.note")}</Text>
      <Flex justify="end">
        <Button onClick={() => void save()} disabled={saving || !targets}><Save size={16} />{t("returnRoute.save")}</Button>
      </Flex>
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
