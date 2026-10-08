<template>
  <BaseDialog :show="!!item" :title="item ? t('packages.freezeDialog.title', { name: item.name }) : ''" width="narrow" @close="emit('close')">
    <div v-if="item" class="fd">
      <p>{{ t('packages.freezeDialog.desc') }}</p>
      <dl>
        <dt>{{ t('packages.freezeDialog.today') }}</dt>
        <dd>{{ todayLabel }}</dd>
        <dt>{{ t('packages.freezeDialog.expires') }}</dt>
        <dd class="pkg-mono">{{ formatDateTimeToMinute(item.expires_at) }}</dd>
        <dt>{{ t('packages.freezeDialog.left') }}</dt>
        <dd>{{ t('packages.freezeDialog.leftValue', { left: leftLabel, max: item.max_freeze_days }) }}</dd>
        <dt>{{ t('packages.freezeDialog.unfreeze') }}</dt>
        <dd>{{ t('packages.freezeDialog.unfreezeValue', { max: item.max_freeze_days }) }}</dd>
        <dt>{{ t('packages.freezeDialog.after') }}</dt>
        <dd>{{ t('packages.freezeDialog.afterValue') }}</dd>
      </dl>
    </div>
    <template #footer>
      <div class="fd-actions">
        <button type="button" class="pkg-btn pkg-btn-ghost" @click="emit('close')">{{ t('packages.notice.cancel') }}</button>
        <button type="button" class="pkg-btn pkg-btn-ice" :disabled="busy" data-test="package-freeze-confirm" @click="item && emit('confirm', item)">
          {{ t('packages.freezeDialog.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { UserPackageView } from '@/api/packages'
import { formatDateTimeToMinute } from '@/utils/format'
import { splitDuration } from '../packageUtils'

const props = defineProps<{
  item: UserPackageView | null
  todayLabel: string
  busy: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: [item: UserPackageView]
}>()

const { t } = useI18n()

const leftLabel = computed(() => {
  const { days, hours } = splitDuration(props.item?.freeze_left_seconds ?? 0)
  if (days && hours) return t('packages.mine.duration.dayHours', { d: days, h: hours })
  if (days) return t('packages.mine.duration.days', { d: days })
  return t('packages.mine.duration.hours', { h: hours })
})
</script>

<style scoped>
.fd { display: grid; gap: 14px; font-size: 14px; color: var(--lc-ink-2); }
.fd p { margin: 0; }
.fd dl {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 8px 16px;
  margin: 0;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--lc-line);
  background: var(--lc-surface-2);
}
.fd dt { color: var(--lc-ink-3); }
.fd dd { margin: 0; font-weight: 600; color: var(--lc-ink); }
.fd-actions { display: flex; justify-content: flex-end; gap: 8px; width: 100%; }
</style>
