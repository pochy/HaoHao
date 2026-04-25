<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  message?: string
  roleLabel?: string
}>(), {
  title: 'Machine client admin role required',
  message: '',
  roleLabel: 'machine_client_admin',
})
</script>

<template>
  <section class="panel stack">
    <span class="status-pill danger">Forbidden</span>
    <h2>{{ title }}</h2>
    <p v-if="message">
      {{ message }}
    </p>
    <p v-else>
      この画面を使うには global role
      <code>{{ roleLabel }}</code>
      が必要です。
    </p>
  </section>
</template>
