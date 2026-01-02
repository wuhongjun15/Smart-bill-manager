import { computed, ref } from 'vue'

export type ThemeMode = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'sbm.theme.mode'

const mode = ref<ThemeMode>('system')
const prefersDark = ref(false)

let mql: MediaQueryList | null = null
let mqlHandler: ((e: MediaQueryListEvent) => void) | null = null

const effectiveDark = computed(() => mode.value === 'dark' || (mode.value === 'system' && prefersDark.value))

function readStoredMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw === 'light' || raw === 'dark') return raw
  } catch {
    // ignore
  }
  return 'system'
}

function writeStoredMode(next: ThemeMode) {
  try {
    if (next === 'system') {
      localStorage.removeItem(STORAGE_KEY)
    } else {
      localStorage.setItem(STORAGE_KEY, next)
    }
  } catch {
    // ignore
  }
}

function applyDOM() {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('sbm-dark', effectiveDark.value)
  document.documentElement.dataset.sbmThemeMode = mode.value
}

function ensureMql() {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
  if (mql) return
  mql = window.matchMedia('(prefers-color-scheme: dark)')
  prefersDark.value = !!mql.matches
  mqlHandler = (e) => {
    prefersDark.value = !!e.matches
    if (mode.value === 'system') applyDOM()
  }
  if (typeof mql.addEventListener === 'function') {
    mql.addEventListener('change', mqlHandler)
  } else {
    // Safari fallback
    ;(mql as any).addListener(mqlHandler)
  }
}

export function initTheme() {
  mode.value = readStoredMode()
  ensureMql()
  applyDOM()
}

export function setThemeMode(next: ThemeMode) {
  mode.value = next
  writeStoredMode(next)
  ensureMql()
  applyDOM()
}

export function toggleTheme() {
  const next = effectiveDark.value ? 'light' : 'dark'
  setThemeMode(next)
}

export function useTheme() {
  return {
    mode,
    prefersDark,
    effectiveDark,
    initTheme,
    setThemeMode,
    toggleTheme,
  }
}
