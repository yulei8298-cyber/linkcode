import { onBeforeUnmount, onMounted, ref } from 'vue'

export const useDocumentDarkMode = () => {
  const isDarkMode = ref(document.documentElement.classList.contains('dark'))
  let observer: MutationObserver | null = null

  const syncDarkMode = () => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
  }

  onMounted(() => {
    syncDarkMode()
    observer = new MutationObserver(syncDarkMode)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class']
    })
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
  })

  return { isDarkMode }
}
