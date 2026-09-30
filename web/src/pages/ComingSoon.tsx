import { Header } from '../components/Header'
import { EmptyState } from '../components/ui'
import type { IconName } from '../components/Icon'

export function ComingSoon({ title, icon, iteration }: { title: string; icon: IconName; iteration: number }) {
  return (
    <>
      <Header title={title} />
      <main className="p-8">
        <EmptyState icon={icon} title={`${title} ist in Arbeit`} text={`Dieses Modul ist fachlich spezifiziert und folgt in Iteration ${iteration} (docs/phase-3/00-plan.md).`} />
      </main>
    </>
  )
}
