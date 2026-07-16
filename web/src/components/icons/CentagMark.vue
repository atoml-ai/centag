<template>
  <img
    class="centag-mark"
    :width="size"
    :height="size"
    :style="{ color: color }"
    src="@/components/icons/centag-mark.svg"
    alt="Centag"
  />
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: number | string
    color?: string
  }>(),
  {
    size: 20,
    color: 'currentColor'
  }
)
</script>

<style scoped>
.centag-mark {
  display: block;
  flex-shrink: 0;
  object-fit: contain;
}
</style>