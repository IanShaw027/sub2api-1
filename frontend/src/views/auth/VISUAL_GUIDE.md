# Authentication Views Visual Guide

The canonical visual contract is [UI standards](../../../../../openspec/changes/glass-ui-redesign/ui-standards.md). This guide covers authentication-specific usage, not a second palette or component implementation.

## Structure

- Preserve the existing `AuthLayout` shell and its public branding, theme control and footer.
- Use `TextInput` and `FieldLabel` for login, registration and password recovery fields. Preserve each input's ID, name, autocomplete, required/disabled state and validation handlers.
- Use the input suffix slot for the existing password visibility button. It must remain a non-submit button with a translated accessible label.
- Use the shared `Button` for submission and the shared error/notice treatment. Form error text must remain associated with the input and announced with `role="alert"`.
- Provider login controls, CAPTCHA, agreement, invitation/promotion codes, email verification and two-factor steps remain controlled by existing settings and auth logic. Visual migration must not remove or bypass them.

## Appearance

Colors come from `src/styles/tokens.css`; shared controls come from `src/style.css` and `src/components/ui`. Both `glass-light` and `glass-dark` are supported now, not a future enhancement.

Use `--foreground` for content, `--muted` for supporting text, `--info-text` for links on light surfaces, `--danger-text` for errors and `--border-strong` where a control boundary needs contrast. Do not introduce literal palette colors or component-local theme branches.

Use shared display/body/mono font stacks and explicit sizes. Labels, hints and errors must wrap without obscuring adjacent fields. At mobile widths the form remains full-width within the shell padding; provider actions and password controls remain reachable.

Focus uses the shared control halo; links use the shared outline. Reduced-motion preference disables nonessential motion while retaining status meaning.

## Acceptance

- Check light and dark at 1440, 1024, 768 and 390 pixels.
- Verify labels, inline error, pending submission, password visibility and keyboard submission.
- Verify conditional registration, provider, CAPTCHA, two-factor and recovery flows with appropriate fixtures. A default mock screenshot alone does not prove those integrations.
- Check keyboard reachability and visible focus; retain tests for state transitions and payloads.
- Do not treat historical performance targets, unmeasured bundle sizes or an unexecuted browser checklist as verified results.

Current screenshots and coverage limits are recorded in [quality hardening verification](../../../../../openspec/changes/glass-ui-quality-hardening/verification.md).
