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
  background: linear-gradient(180deg, transparent, var(--background) 70%);
}
</style>
