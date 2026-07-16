import { computed } from 'vue'
import { editionRef, type Edition } from '@/utils/edition'

export function useEdition() {
  const edition = computed<Edition>(() => editionRef.value)
  const isPersonal = computed(() => edition.value === 'personal')
  const isTeam = computed(() => edition.value === 'team')
  const isMinimal = computed(() => edition.value === 'minimal')
  return { edition, isPersonal, isTeam, isMinimal }
}
