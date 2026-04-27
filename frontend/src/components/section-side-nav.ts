import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

export type SectionSideNavItem = {
  key: string
  label: string
  description?: string
  to: RouteLocationRaw
  icon?: Component
}
