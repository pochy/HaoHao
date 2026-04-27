<script setup lang="ts">
import type { SectionSideNavItem } from './section-side-nav'

defineProps<{
  navLabel: string
  title?: string
  description?: string
  items: SectionSideNavItem[]
}>()
</script>

<template>
  <aside class="section-side-nav" :aria-label="navLabel">
    <div v-if="title || description" class="section-side-nav-header">
      <strong v-if="title">{{ title }}</strong>
      <span v-if="description">{{ description }}</span>
    </div>

    <nav class="section-local-nav">
      <RouterLink
        v-for="item in items"
        :key="item.key"
        class="section-local-link"
        active-class="active"
        exact-active-class="active"
        :to="item.to"
      >
        <component
          :is="item.icon"
          v-if="item.icon"
          :size="17"
          stroke-width="1.9"
          aria-hidden="true"
        />
        <span>
          <strong>{{ item.label }}</strong>
          <small v-if="item.description">{{ item.description }}</small>
        </span>
      </RouterLink>
    </nav>

    <slot name="footer" />
  </aside>
</template>
