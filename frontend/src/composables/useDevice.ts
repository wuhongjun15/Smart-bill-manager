import { onBeforeUnmount, onMounted, ref } from 'vue'

const addMqListener = (mq: MediaQueryList, handler: () => void) => {
  if (typeof mq.addEventListener === 'function') mq.addEventListener('change', handler)
  else if (typeof mq.addListener === 'function') mq.addListener(handler)
}

const removeMqListener = (mq: MediaQueryList, handler: () => void) => {
  if (typeof mq.removeEventListener === 'function') mq.removeEventListener('change', handler)
  else if (typeof mq.removeListener === 'function') mq.removeListener(handler)
}

export const useIsMobileUI = (breakpointPx = 768) => {
  const isMobileUI = ref(false)

  let mqWidth: MediaQueryList | null = null
  let mqCoarse: MediaQueryList | null = null
  let mqHoverNone: MediaQueryList | null = null

  const update = () => {
    if (!mqWidth || !mqCoarse || !mqHoverNone) return
    isMobileUI.value = mqWidth.matches && (mqCoarse.matches || mqHoverNone.matches)
  }

  const bind = () => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
    mqWidth = window.matchMedia(`(max-width: ${breakpointPx}px)`)
    mqCoarse = window.matchMedia('(pointer: coarse)')
    mqHoverNone = window.matchMedia('(hover: none)')
    update()
    addMqListener(mqWidth, update)
    addMqListener(mqCoarse, update)
    addMqListener(mqHoverNone, update)
  }

  const unbind = () => {
    if (!mqWidth || !mqCoarse || !mqHoverNone) return
    removeMqListener(mqWidth, update)
    removeMqListener(mqCoarse, update)
    removeMqListener(mqHoverNone, update)
  }

  onMounted(bind)
  onBeforeUnmount(unbind)

  return { isMobileUI }
}
