import Loading from "@/components/loading";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { Button, Card, Flex, Switch, Text, TextField } from "@radix-ui/themes";
import { Save } from "lucide-react";
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
};

export default function ReturnRoutes() {
  const { t } = useTranslation();
  const { call } = useRPC2Call();
  const [targets, setTargets] = useState<Target[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

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

  const update = (id: string, patch: Partial<Target>) => {
    setTargets((current) => current?.map((item) => item.id === id ? { ...item, ...patch } : item) ?? null);
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
    </Flex>
  );
}
