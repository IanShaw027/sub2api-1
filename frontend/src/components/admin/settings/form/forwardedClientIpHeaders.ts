// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：转发客户端 IP 请求头名称的
// 校验与归一化（Header-Case 大小写规范化 + 去重 + 数量上限）。纯函数，无状态依赖，直接导出。
export const forwardedClientIpHeaderTokenPattern = /^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/;

export const maxForwardedClientIpHeaders = 16;

export function normalizeForwardedClientIpHeader(raw: string): string {
  const header = raw.trim();
  if (!forwardedClientIpHeaderTokenPattern.test(header)) {
    return "";
  }

  return header
    .toLowerCase()
    .split("-")
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join("-");
}

export function normalizeForwardedClientIpHeaders(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const headers: string[] = [];
  const seen = new Set<string>();
  for (const raw of value) {
    if (typeof raw !== "string") {
      continue;
    }
    const header = normalizeForwardedClientIpHeader(raw);
    const key = header.toLowerCase();
    if (!header || seen.has(key) || headers.length >= maxForwardedClientIpHeaders) {
      continue;
    }
    seen.add(key);
    headers.push(header);
  }
  return headers;
}

