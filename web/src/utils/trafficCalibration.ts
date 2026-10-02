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
  offset: number,
): number {
  return trafficUsedByType(up, down, type) + Math.max(0, offset || 0);
}
