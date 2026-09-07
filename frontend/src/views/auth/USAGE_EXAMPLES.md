# Authentication UI Composition

Use existing authentication routes and handlers. This example illustrates shared
component composition, not an alternative authentication implementation:

```vue
<AuthLayout>
  <form @submit.prevent="handleLogin">
    <TextInput
      id="email"
      v-model="formData.email"
      type="email"
      autocomplete="email"
      :label="t('auth.emailLabel')"
      :error="errors.email"
      :disabled="authActionDisabled"
      required
    />
    <!-- Preserve password, agreement, captcha and provider controls. -->
    <Button native-type="submit" :loading="isLoading" :disabled="authActionDisabled">
      {{ t('auth.signIn') }}
    </Button>
  </form>
</AuthLayout>
```

Import `AuthLayout` from `@/components/layout/AuthLayout.vue`, and `TextInput` and
`Button` from `@/components/ui/`. The owning view defines the bindings above.

For the complete password suffix, feature gates, failure recovery and submission
flow, use [LoginView.vue](./LoginView.vue). Current API signatures and session
behavior live in [auth.ts](../../stores/auth.ts); do not duplicate them here.

Visual changes follow [UI standards](../../../../../openspec/changes/glass-ui-redesign/ui-standards.md).
Verify keyboard focus, field errors, password visibility, disabled/loading states
and mobile layout without altering the existing authentication contract.
