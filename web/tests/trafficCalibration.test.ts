import { test } from "node:test";
import assert from "node:assert/strict";
import { parseTrafficCalibration, calibratedTrafficUsed, formatTrafficCalibration } from "../src/utils/trafficCalibration.ts";

test("accepts fractional usage and rounds to a whole byte", () => {
  assert.equal(parseTrafficCalibration("71.95 GB"), 77255724237);
  assert.equal(parseTrafficCalibration(" 1.5 GiB "), 1610612736);
  assert.equal(parseTrafficCalibration("0.5 B"), 1);
  assert.equal(parseTrafficCalibration("0"), 0);
  assert.equal(parseTrafficCalibration("120 GB"), 120 * 1024 ** 3);
});

test("rejects invalid and unsafe values", () => {
  for (const value of ["-1 GB", "Infinity", "1e9", "71.95 GB extra", "", "8 PB", "9007199254740992 B"]) {
    assert.equal(parseTrafficCalibration(value), null, value);
  }
});

test("preserves compatibility with additive values from older panels", () => {
  for (const [type, used] of [["up", 100], ["down", 200], ["min", 100], ["max", 200], ["sum", 300]] as const) {
    assert.equal(calibratedTrafficUsed(100, 200, type, 700), used + 700);
    assert.equal(calibratedTrafficUsed(0, 0, type, 700), 700);
  }
});

test("shows saved calibration in readable units without changing a byte", () => {
  assert.equal(formatTrafficCalibration(76235669504), "71 GB");
  assert.equal(formatTrafficCalibration(77255724237), "71.95 GB");
  for (const bytes of [0, 1, 1023, 1024, 1025, 1024 ** 3 - 1, 76235669504, 77255724237, 1024 ** 4, Number.MAX_SAFE_INTEGER]) {
    assert.equal(parseTrafficCalibration(formatTrafficCalibration(bytes)), bytes);
  }
});

test("calibration overrides current usage and counts only subsequent traffic", () => {
  for (const [type, baseline] of [["up", 100], ["down", 200], ["min", 100], ["max", 200], ["sum", 300]] as const) {
    assert.equal(calibratedTrafficUsed(100, 200, type, 168, baseline), 168);
    assert.equal(calibratedTrafficUsed(110, 210, type, 168, baseline), type === "sum" ? 188 : 178);
    assert.equal(calibratedTrafficUsed(100, 200, type, 168, null), 168);
    assert.equal(calibratedTrafficUsed(100, 200, type, 0, null), baseline);
  }
});
