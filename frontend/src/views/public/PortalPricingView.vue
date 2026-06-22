<template>
  <PortalLayout>
    <HomePricingSection :show-channel-detail-link="false" />
  </PortalLayout>
</template>

<script setup lang="ts">
import PortalLayout from './components/PortalLayout.vue'
import HomePricingSection from './components/HomePricingSection.vue'
</script>
