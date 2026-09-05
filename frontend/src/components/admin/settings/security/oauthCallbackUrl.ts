// 从 SecurityTab.vue 拆分而来：多个 OAuth 登录面板（LinuxDo / GitHub-Google / WeChat / OIDC）
// 共用的后端回调地址拼接逻辑，逐字保留原实现，避免行为变化。
export function buildApiCallbackUrl(
  apiBaseUrl: string,
  currentOrigin: string,
  path: string,
): string {
  const base = (apiBaseUrl || currentOrigin).replace(/\/+$/, "");
  const apiRoot = base.endsWith("/api/v1") ? base : `${base}/api/v1`;
  return `${apiRoot}${path.startsWith("/") ? path : `/${path}`}`;
}
