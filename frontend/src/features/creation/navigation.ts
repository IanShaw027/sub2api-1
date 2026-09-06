export const creationModes = ['chat', 'image', 'video', 'voice', 'gallery'] as const
export type WorkspaceMode = typeof creationModes[number]

export function creationMode(value: unknown): WorkspaceMode {
  return creationModes.find(mode => mode === value) ?? 'chat'
}

export function creationPath(mode: WorkspaceMode): string {
  return `/studio/${mode}`
}

export function creationTitleKey(mode: WorkspaceMode): string {
  return `nav.creation.${mode}`
}
