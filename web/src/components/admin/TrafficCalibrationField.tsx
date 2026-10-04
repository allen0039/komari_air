import React from "react";
import { Text, TextField } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import { useRPC2Call } from "@/contexts/RPC2Context";
import { useNodeDetails } from "@/contexts/NodeDetailsContext";
import { useNodeList } from "@/contexts/NodeListContext";
import { formatBytes } from "@/utils/unitHelper";
import { formatTrafficCalibration, parseTrafficCalibration } from "@/utils/trafficCalibration";

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
    const [value, setValue] = React.useState(formatTrafficCalibration(initialBytes || 0));
    const savedBytes = React.useRef(initialBytes || 0);
    const edited = React.useRef(false);

    React.useEffect(() => {
      savedBytes.current = initialBytes || 0;
      edited.current = false;
      setValue(formatTrafficCalibration(initialBytes || 0));
    }, [initialBytes, uuid]);

    React.useImperativeHandle(ref, () => ({
      async saveIfChanged() {
        const bytes = parseTrafficCalibration(value);
        if (bytes === null) {
          throw new Error(t("admin.trafficCalibration.invalid", "请输入有效的非负流量，例如 120 GB"));
        }
        if (!edited.current) return;
        await call("admin:editClient", { uuid, traffic_used_offset: bytes });
        savedBytes.current = bytes;
        edited.current = false;
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
          onChange={(event) => { edited.current = true; setValue(event.target.value); }}
        />
        <Text as="div" size="1" color="gray" mt="1">
          {t("admin.trafficCalibration.current", "当前校准值")}: {formatBytes(savedBytes.current)} ·{" "}
          {t(
            "admin.trafficCalibration.description",
            "填写当前已用总量，如 120 GB。保存后以此值作为当前用量，仅累计之后新增的流量，用于显示和流量提醒；不写入历史流量或计费。设置为 0 可清除。启用网络统计月重置后，校准值按面板时区在下次重置日自动清零；未启用时需手动清除。",
          )}
        </Text>
      </div>
    );
  },
);
