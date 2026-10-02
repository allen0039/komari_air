import React from "react";
import { Text, TextField } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { useNodeDetails } from "@/contexts/NodeDetailsContext";
import { useNodeList } from "@/contexts/NodeListContext";
import { formatBytes } from "@/utils/unitHelper";
import { parseTrafficCalibration } from "@/utils/trafficCalibration";

export type TrafficCalibrationHandle = {
  saveIfChanged: () => Promise<void>;
};

type Props = {
  uuid: string;
  initialBytes: number;
};

export const TrafficCalibrationField = React.forwardRef<TrafficCalibrationHandle, Props>(
  function TrafficCalibrationField({ uuid, initialBytes }, ref) {
    const { t } = useTranslation();
    const { call } = useRPC2Call();
    const { refresh } = useNodeDetails();
    const nodeList = useNodeList(false);
    const [value, setValue] = React.useState(`${initialBytes || 0} B`);
    const savedBytes = React.useRef(initialBytes || 0);

    React.useEffect(() => {
      savedBytes.current = initialBytes || 0;
      setValue(`${initialBytes || 0} B`);
    }, [initialBytes, uuid]);

    React.useImperativeHandle(ref, () => ({
      async saveIfChanged() {
        const bytes = parseTrafficCalibration(value);
        if (bytes === null) {
          throw new Error(t("admin.trafficCalibration.invalid", "请输入有效的非负流量，例如 120 GB"));
        }
        if (bytes === savedBytes.current) return;
        await call("admin:editClient", { uuid, traffic_used_offset: bytes });
        savedBytes.current = bytes;
        refresh();
        nodeList?.refresh();
      },
    }), [call, nodeList, refresh, t, uuid, value]);

    return (
      <div>
        <label className="mb-1 block text-sm font-bold">
          {t("admin.trafficCalibration.label", "已用流量校准值")}
        </label>
        <TextField.Root
          value={value}
          placeholder="0 B"
          onChange={(event) => setValue(event.target.value)}
        />
        <Text as="div" size="1" color="gray" mt="1">
          {t("admin.trafficCalibration.current", "当前校准值")}: {formatBytes(savedBytes.current)} ·{" "}
          {t(
            "admin.trafficCalibration.description",
            "填写重装前已用的流量，如 120 GB。此值加到当前 Agent 用量，仅用于显示和流量提醒；不写入历史流量或计费。设置为 0 可清除。",
          )}
        </Text>
      </div>
    );
  },
);
