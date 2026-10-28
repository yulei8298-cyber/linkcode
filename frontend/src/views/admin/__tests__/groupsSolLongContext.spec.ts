import { describe, expect, it } from "vitest";
import {
  normalizeSolContextWindow,
  SOL_CONTEXT_WINDOW_MAX,
  SOL_CONTEXT_WINDOW_MIN,
} from "../groupsSolLongContext";

describe("normalizeSolContextWindow", () => {
  it("留空或无效输入按 0（沿用上游默认值）提交", () => {
    for (const value of ["", null, undefined, "abc", -1, 0, Number.NaN]) {
      expect(normalizeSolContextWindow(value as never)).toBe(0);
    }
  });

  it("保留常用的 272k / 500k 并对小数取整", () => {
    expect(normalizeSolContextWindow(272000)).toBe(272000);
    expect(normalizeSolContextWindow("500000")).toBe(500000);
    expect(normalizeSolContextWindow(499999.6)).toBe(500000);
  });

  it("范围与后端一致，越界值原样提交交由后端拒绝", () => {
    expect(SOL_CONTEXT_WINDOW_MIN).toBe(16000);
    expect(SOL_CONTEXT_WINDOW_MAX).toBe(872000);
    expect(normalizeSolContextWindow(900000)).toBe(900000);
  });
});
