/** Sidebar sections are open by default except `myAccount`. */
export function defaultSectionOpen(key: string): boolean {
  return key !== 'myAccount'
}

/**
 * Static fallback for which sidebar section owns a driver.js step selector.
 * Runtime resolution should prefer the element's closest `[data-section]`.
 * `/keys` is assumed admin "My Account"; user nav puts it in `workspace`.
 */
const SELECTOR_SECTION_KEYS: Record<string, string> = {
  '#sidebar-group-manage': 'usersResources',
  'sidebar-group-manage': 'usersResources',
  '#sidebar-channel-manage': 'usersResources',
  'sidebar-channel-manage': 'usersResources',
  '#sidebar-wallet': 'operations',
  'sidebar-wallet': 'operations',
  '[data-tour="sidebar-my-keys"]': 'myAccount',
  'sidebar-my-keys': 'myAccount'
}

export function sectionKeyForSelector(selector: string): string | undefined {
  return SELECTOR_SECTION_KEYS[selector.trim()]
}
