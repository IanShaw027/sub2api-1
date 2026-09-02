<template>
  <div class="ui-list-fade" aria-hidden="true" />
</template>

<style scoped>
.ui-list-fade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 120px;
  pointer-events: none;
  background: linear-gradient(
    to bottom,
    color-mix(in oklch, var(--background) 0%, transparent),
    color-mix(in oklch, var(--background) 92%, transparent)
  );
}
</style>
