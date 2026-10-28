<template>
  <div class="space-y-3" data-testid="video-account-fields">
    <div>
      <label class="input-label">{{ t('admin.video.protocol') }}</label>
      <div class="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
        <button
          v-for="item in protocols"
          :key="item.protocol"
          type="button"
          :data-testid="`video-protocol-${item.protocol}`"
          :class="[
            'flex items-center gap-2 rounded-lg border px-3 py-2 text-left text-sm transition-all',
            protocol === item.protocol
              ? 'border-fuchsia-500 bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/20 dark:text-fuchsia-300'
              : 'border-gray-200 text-gray-700 hover:border-fuchsia-300 dark:border-dark-600 dark:text-gray-300'
          ]"
          @click="selectProtocol(item.protocol)"
        >
          <BrandIcon v-if="vendorOf(item.protocol).icon" :brand="vendorOf(item.protocol).icon" size="md" />
          <span class="truncate">{{ item.display_name }}</span>
        </button>
      </div>
      <p class="input-hint">{{ t('admin.video.protocolHint') }}</p>
    </div>

    <div v-if="current" class="rounded-lg bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-400">
      <div class="flex flex-wrap items-center gap-2">
        <span>{{ t('admin.video.officialUrl') }}</span>
        <code class="rounded bg-white px-1.5 py-0.5 dark:bg-dark-800">{{ current.default_base_url }}</code>
        <button type="button" class="text-fuchsia-600 hover:underline dark:text-fuchsia-400" @click="emit('update:baseUrl', current.default_base_url)">
          {{ t('admin.video.useOfficialUrl') }}
        </button>
      </div>
      <p class="mt-1">{{ t('admin.video.baseUrlHint') }}</p>
      <p class="mt-1">
        {{ t('admin.video.nativePrefixHint') }}
        <code class="rounded bg-white px-1.5 py-0.5 dark:bg-dark-800">{{ current.native_prefix }}</code>
      </p>
      <div v-if="current.example_models?.length" class="mt-2 flex flex-wrap items-center gap-1.5">
        <span>{{ t('admin.video.exampleModels') }}</span>
        <span v-for="model in current.example_models" :key="model" class="rounded bg-white px-1.5 py-0.5 font-mono dark:bg-dark-800">{{ model }}</span>
        <button type="button" class="text-fuchsia-600 hover:underline dark:text-fuchsia-400" data-testid="video-apply-models" @click="emit('apply-models', [...(current.example_models ?? [])])">
          {{ t('admin.video.applyExampleModels') }}
        </button>
      </div>
    </div>

    <div v-if="current?.supports_secret_key">
      <label class="input-label">{{ t('admin.video.secretKey') }}</label>
      <input
        :value="secretKey"
        type="password"
        class="input font-mono"
        autocomplete="new-password"
        data-testid="video-secret-key"
        :placeholder="hasSecretKey ? t('admin.video.secretKeyKeep') : 'SecretKey'"
        @input="emit('update:secretKey', ($event.target as HTMLInputElement).value)"
      />
      <p class="input-hint">{{ t('admin.video.secretKeyHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BrandIcon from '@/components/icons/brands/BrandIcon.vue'
import { getVideoProtocol, listVideoProtocols, videoVendorStyle } from '@/constants/videoBrands'

interface Props {
  protocol: string
  secretKey: string
  baseUrl: string
  /** 编辑已有账号时为 true：Secret Key 留空表示保持不变。 */
  hasSecretKey?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:protocol': [value: string]
  'update:secretKey': [value: string]
  'update:baseUrl': [value: string]
  'apply-models': [models: string[]]
}>()
const { t } = useI18n()

const protocols = listVideoProtocols()
const current = computed(() => getVideoProtocol(props.protocol))
const vendorOf = (protocol: string) => videoVendorStyle(protocol)

/** 切换协议时，若 Base URL 仍是上一个协议的官方地址（或为空），自动换成新协议的官方地址。 */
function selectProtocol(next: string) {
  const previousDefault = current.value?.default_base_url ?? ''
  emit('update:protocol', next)
  const nextDefault = getVideoProtocol(next)?.default_base_url ?? ''
  if (!props.baseUrl.trim() || props.baseUrl.trim() === previousDefault) {
    emit('update:baseUrl', nextDefault)
  }
}
</script>
