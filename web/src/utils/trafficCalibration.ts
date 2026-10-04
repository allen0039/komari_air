// Accept a plain byte count or a binary unit such as 120 GB / 1.5 TB.
export function parseTrafficCalibration(input: string): number | null {
  const match = input.trim().match(/^([0-9]+(?:\.[0-9]+)?)\s*(B|K(?:I?B)?|M(?:I?B)?|G(?:I?B)?|T(?:I?B)?|P(?:I?B)?)?$/i);
  if (!match) return null;
  const unit = (match[2] || "B").toUpperCase()[0];
  const power = "BKMGTP".indexOf(unit);
  if (power < 0) return null;
  const bytes = Number(match[1]) * 1024 ** power;
  if (!Number.isFinite(bytes) || bytes < 0 || bytes > Number.MAX_SAFE_INTEGER) return null;
  return Math.round(bytes);
}

export function trafficUsedByType(
  up: number,
  down: number,
  type: "max" | "min" | "sum" | "up" | "down",
): number {
  switch (type) {
    case "max": return Math.max(up, down);
    case "min": return Math.min(up, down);
    case "up": return up;
    case "down": return down;
    case "sum": return up + down;
  }
}

export function calibratedTrafficUsed(
  up: number,
  down: number,
  type: "max" | "min" | "sum" | "up" | "down",
  target: number,
  baseline?: number | null,
): number {
  const raw = trafficUsedByType(up, down, type);
  if (target <= 0) return raw;
  // Older panels supplied an additive offset without a baseline field.
  if (baseline === undefined) return raw + target;
  return target + (baseline === null ? 0 : Math.max(0, raw - baseline));
}

// Return a human-readable unit without losing bytes when the unchanged field is saved.
export function formatTrafficCalibration(bytes: number): string {
  if (!Number.isSafeInteger(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const amount = bytes / 1024 ** power;
  for (let digits = 0; digits <= 15; digits++) {
    const value = `${amount.toFixed(digits).replace(/(\.\d*?)0+$/, "$1").replace(/\.$/, "")} ${units[power]}`;
    if (parseTrafficCalibration(value) === bytes) return value;
  }
  return `${bytes} B`;
}
