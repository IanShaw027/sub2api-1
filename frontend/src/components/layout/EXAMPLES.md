# Layout Component Examples

Use [UI standards](../../../../../openspec/changes/glass-ui-redesign/ui-standards.md) as the single visual contract. Existing views own API calls, permissions and workflow state; layouts only arrange slots.

## Dashboard

```vue
<AppLayout>
  <DashboardPageLayout>
    <template #header><!-- Existing title and actions --></template>
    <template #stats><!-- Shared StatCard components --></template>
    <template #charts><!-- Existing charts, loading/error/empty states --></template>
    <template #lists><!-- Existing recent activity --></template>
  </DashboardPageLayout>
</AppLayout>
```

Import layouts from `@/components/layout` where exported, otherwise use their existing file paths. Keep current data and error handling; do not replace a populated view with decorative sample statistics.

## Table Page

```vue
<AppLayout>
  <TablePageLayout>
    <template #actions><!-- Existing create, export and batch actions --></template>
    <template #filters><!-- Existing filters and column preferences --></template>
    <template #table><!-- DataTable with unchanged columns and handlers --></template>
    <template #pagination><!-- Existing Pagination and page-size state --></template>
  </TablePageLayout>
</AppLayout>
```

`TablePageLayout` places pagination inside its table container footer. Do not create another floating pagination section or add nested card shadows. Wide data may scroll inside the table, never by overflowing the viewport. Responsive presentation must not silently hide selected columns or existing actions.

## Settings

```vue
<AppLayout>
  <SettingsPageLayout>
    <template #header><!-- Title and one primary save action --></template>
    <template #nav><!-- Existing section navigation --></template>
    <template #content><!-- Existing settings sections and semantic rows --></template>
  </SettingsPageLayout>
</AppLayout>
```

Without `#nav`, content uses the full width. Keep save/reset/dirty/loading behavior in the owning view. `SettingRow` is available for appropriate new rows; do not claim all legacy settings are converted without checking their markup.

## Authentication

Retain `AuthLayout` and use shared `TextInput`, `FieldLabel` and `Button` primitives. See [authentication guide](../../views/auth/VISUAL_GUIDE.md). Authentication handlers and provider controls are not layout responsibilities.

## Rules

- Use semantic tokens and shared components rather than literal colors or custom control skins.
- `AppHeader` keeps a stable single row; overflow actions retain an explicit More entry and keyboard access.
- Sidebar user preferences persist, while new/default navigation remains discoverable.
- Use local skeleton, empty and failure states without replacing useful loaded content with a global spinner.
- Validate the full mobile workflow, not only the initial viewport.
