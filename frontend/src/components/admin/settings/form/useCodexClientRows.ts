// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：Codex 客户端黑白名单 /
// 指纹信号行的本地表格状态与 JSON 行 <-> 字符串互转。逐字保留原实现。
import { ref } from "vue";
import type { CodexClientRow } from "../useSettingsForm";
import type { FingerprintSignalRow } from "@/views/admin/codexFingerprintSignals";

export function useCodexClientRows() {
  const codexBlacklistRows = ref<CodexClientRow[]>([]);

  const codexWhitelistRows = ref<CodexClientRow[]>([]);

  const codexFingerprintRows = ref<FingerprintSignalRow[]>([]);

  function parseCodexEntriesToRows(raw: string): CodexClientRow[] {
    if (!raw || !raw.trim()) return [];
    try {
      const arr = JSON.parse(raw);
      if (!Array.isArray(arr)) return [];
      return arr.map((e) => ({
        originator: typeof e?.originator === "string" ? e.originator : "",
        uaContains: Array.isArray(e?.ua_contains)
          ? e.ua_contains
              .filter((x: unknown) => typeof x === "string")
              .join(", ")
          : "",
        skipEngineFingerprint: e?.skip_engine_fingerprint === true,
      }));
    } catch {
      return [];
    }
  }

  function serializeCodexRowsToJSON(rows: CodexClientRow[]): string {
    const entries = rows
      .map((r) => {
        const entry: {
          originator: string;
          ua_contains: string[];
          skip_engine_fingerprint?: boolean;
        } = {
          originator: r.originator.trim(),
          ua_contains: r.uaContains
            .split(",")
            .map((s) => s.trim())
            .filter((s) => s.length > 0),
        };
        if (r.skipEngineFingerprint) entry.skip_engine_fingerprint = true;
        return entry;
      })
      .filter((e) => e.originator !== "" || e.ua_contains.length > 0);
    return entries.length > 0 ? JSON.stringify(entries) : "";
  }

  return {
    codexBlacklistRows,
    codexWhitelistRows,
    codexFingerprintRows,
    parseCodexEntriesToRows,
    serializeCodexRowsToJSON,
  };
}
