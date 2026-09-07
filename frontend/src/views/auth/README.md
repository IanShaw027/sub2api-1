# Authentication Views

Authentication pages use `AuthLayout` and shared Glass UI primitives. The visual
contract is [UI standards](../../../../../openspec/changes/glass-ui-redesign/ui-standards.md),
implemented by [tokens.css](../../styles/tokens.css), [style.css](../../style.css)
and [components/ui](../../components/ui/README.md).

## Current Entry Points

- [LoginView.vue](./LoginView.vue): email/password login, agreement and captcha
  checks, enabled identity providers and the existing two-factor flow.
- [RegisterView.vue](./RegisterView.vue): registration fields and verification
  governed by public settings.
- [ForgotPasswordView.vue](./ForgotPasswordView.vue) and
  [ResetPasswordView.vue](./ResetPasswordView.vue): password recovery.
- [Auth store](../../stores/auth.ts) owns session state; [router](../../router/index.ts)
  owns navigation guards. Read these implementations for current validation,
  redirects and API behavior instead of copying historical workflow snippets.

## UI Conventions

- Use `TextInput` for labels, input state and field errors. Put password visibility
  controls in `#suffix`, preserving `type="button"`, accessible names and state.
- Use `Button` with its loading/disabled props for submission, and shared
  `Checkbox` for binary choices. Keep provider controls and their existing gates.
- Consume semantic tokens for light/dark appearance. Do not introduce separate
  palette classes or page-owned field, focus and button skins.
- Keep submitted values and inline errors visible on failure. A submission loader
  must not replace the entire form.

See [VISUAL_GUIDE.md](./VISUAL_GUIDE.md) for visual ownership and
[USAGE_EXAMPLES.md](./USAGE_EXAMPLES.md) for a component composition example.
