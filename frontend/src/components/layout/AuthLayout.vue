<template>
  <div class="public-page auth-page">
    <AuthBrandPanel>
      <template v-if="$slots.brand" #default>
        <slot name="brand" />
      </template>
    </AuthBrandPanel>

    <div class="auth-form-wrap">
      <div class="auth-card glass-card glass-ring">
        <slot />
      </div>
      <div v-if="$slots.footer" class="auth-footer">
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import AuthBrandPanel from './AuthBrandPanel.vue'
</script>

<style scoped>
.auth-page {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
}

/* ---------- Right form column ---------- */
.auth-form-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.auth-card {
  position: relative;
  width: 440px;
  max-width: 100%;
  padding: 36px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.auth-footer {
  margin-top: 16px;
  width: 440px;
  max-width: 100%;
  text-align: center;
  font-size: 12.5px;
  color: var(--muted);
}

@media (max-width: 900px) {
  .auth-page {
    grid-template-columns: 1fr;
  }

  .auth-brand {
    display: none;
  }

  .auth-form-wrap {
    padding: 24px 16px;
  }

  .auth-card,
  .auth-footer {
    width: 100%;
  }

  .auth-card {
    padding: 24px;
  }

  .auth-card :deep(.btn) {
    min-height: 44px;
  }
}
</style>
