/** Sidebar sections are open by default; explicit persisted user choices still win. */
export function defaultSectionOpen(_key: string): boolean {
  return true
}

/**
 * Static fallback for which sidebar section owns a driver.js step selector.
 * Runtime resolution should prefer the element's closest `[data-section]`.
 * `/keys` belongs to the user workspace, including for administrators.
 */
const SELECTOR_SECTION_KEYS: Record<string, string> = {
  '#sidebar-group-manage': 'usersResources',
  'sidebar-group-manage': 'usersResources',
  '#sidebar-channel-manage': 'usersResources',
  'sidebar-channel-manage': 'usersResources',
  '#sidebar-wallet': 'operations',
  'sidebar-wallet': 'operations',
  '[data-tour="sidebar-my-keys"]': 'workspace',
  'sidebar-my-keys': 'workspace'
}

export function sectionKeyForSelector(selector: string): string | undefined {
  return SELECTOR_SECTION_KEYS[selector.trim()]
}
