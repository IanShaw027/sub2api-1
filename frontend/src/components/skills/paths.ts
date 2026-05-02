export const skillPaths = {
  root: '/skills',
  market: '/skills/market',
  installed: '/skills/installed',
  my: '/skills/mine',
  create: '/skills/new',
  detail: (skillId: number | string) => `/skills/${skillId}`,
  edit: (skillId: number | string) => `/skills/${skillId}/edit`,
  versions: (skillId: number | string) => `/skills/${skillId}/versions`,
  runs: (skillId: number | string) => `/skills/${skillId}/runs`,
  revenue: (skillId: number | string) => `/skills/${skillId}/revenue`
} as const
