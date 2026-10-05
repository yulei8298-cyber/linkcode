<template>
  <BaseDialog :show="show" :title="t('packages.notice.title')" width="wide" @close="emit('close')">
    <div
      ref="bodyRef"
      class="notice-body"
      tabindex="0"
      data-test="package-notice-body"
      @scroll="checkRead"
    >
      <p v-for="(line, index) in lines" :key="index" :class="{ 'notice-warn': line.warn }">
        <b v-if="line.title">{{ line.title }}：</b>
        <template v-for="(seg, i) in line.segments" :key="i">
          <span v-if="seg.highlight" class="notice-hl">{{ seg.text }}</span>
          <template v-else>{{ seg.text }}</template>
        </template>
      </p>
    </div>

    <template #footer>
      <div class="notice-foot">
        <div v-if="purchase" :class="['notice-order pkg-hue', `pkg-h-${purchaseHue}`]">
          <b>{{ purchase.plan.name }}</b>
          <span>{{ t('packages.payment.planLine', { group: purchase.groupName, quota: `$${purchase.plan.quota_usd}`, days: purchase.plan.validity_days }) }}</span>
          <span class="notice-price pkg-mono">{{ formatCNY(purchase.plan.price) }}</span>
        </div>
        <label v-if="purchase" :class="['notice-agree', { locked: !read }]">
          <input v-model="agreed" type="checkbox" :disabled="!read" data-test="package-notice-agree" />
          <span>{{ read ? t('packages.notice.agree') : t('packages.notice.readToBottom') }}</span>
        </label>
        <div class="notice-actions">
          <button v-if="purchase" type="button" class="pkg-btn pkg-btn-ghost" @click="emit('close')">
            {{ t('packages.notice.cancel') }}
          </button>
          <button
            v-if="purchase"
            type="button"
            class="pkg-btn pkg-btn-warm"
            :disabled="!agreed"
            data-test="package-notice-confirm"
            @click="emit('confirm', purchase.plan)"
          >
            {{ t('packages.notice.agreeAndPay', { price: formatCNY(purchase.plan.price) }) }}
          </button>
          <button v-else type="button" class="pkg-btn pkg-btn-warm" @click="emit('close')">
            {{ t('packages.notice.gotIt') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { PackagePlan } from '@/api/packages'
import { formatCNY, packagePlanStyle, parsePackageNotice, type PackagePurchaseContext } from '../packageUtils'

const props = defineProps<{
  show: boolean
  text: string
  concurrency: number
  maxFreezeDays: number
  /** 购买模式：必须读到底并勾选同意；为空时只是查看 */
  purchase: PackagePurchaseContext | null
}>()

const emit = defineEmits<{
  close: []
  confirm: [plan: PackagePlan]
}>()

const { t } = useI18n()
const bodyRef = ref<HTMLElement | null>(null)
const read = ref(false)
const agreed = ref(false)

const lines = computed(() =>
  parsePackageNotice(props.text, { concurrency: props.concurrency, maxFreezeDays: props.maxFreezeDays }),
)
const purchaseHue = computed(() =>
  props.purchase ? packagePlanStyle(props.purchase.plan.cycle, props.purchase.plan.tier).hue : 'violet',
)

/** 滚动到底部（留 8px 容差）才允许勾选；内容不足一屏时打开即视为已读。 */
function checkRead() {
  const el = bodyRef.value
  if (!el || read.value) return
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 8) read.value = true
}

watch(
  () => props.show,
  async (open) => {
    if (!open) return
    read.value = false
    agreed.value = false
    await nextTick()
    if (bodyRef.value) bodyRef.value.scrollTop = 0
    checkRead()
  },
)
</script>

<style scoped>
.notice-body {
  max-height: min(52vh, 460px);
  overflow-y: auto;
  display: grid;
  gap: 12px;
  padding-right: 4px;
  font-size: 14px;
  line-height: 1.75;
  color: var(--lc-ink-2);
}
.notice-body p { margin: 0; }
.notice-body b { color: var(--lc-ink); }
.notice-warn b { color: #e11d48; }
.notice-hl {
  color: var(--lc-ink);
  font-weight: 700;
  background: linear-gradient(transparent 60%, rgba(255, 138, 76, 0.3) 60%);
}
.notice-foot { display: grid; gap: 12px; width: 100%; }
.notice-order {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 14px;
  padding: 10px 14px;
  border-radius: 12px;
  border: 1px solid var(--pkg-h-line);
  background: var(--pkg-h-soft);
  font-size: 13.5px;
  color: var(--lc-ink-2);
}
.notice-order b { color: var(--pkg-h-text); font-weight: 800; }
.notice-price { margin-left: auto; font-size: 18px; font-weight: 800; color: var(--pkg-h-text); }
.notice-agree { display: flex; align-items: center; gap: 10px; font-size: 13.5px; font-weight: 600; color: var(--lc-ink); }
.notice-agree input { width: 18px; height: 18px; accent-color: #ff6a3d; }
.notice-agree.locked { font-weight: 500; color: var(--lc-ink-3); }
.notice-actions { display: flex; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
</style>
