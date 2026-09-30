// Fahrzeugbild (Hauptbild) mit Ändern-Knopf für Bearbeiter.
import { useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import { Icon } from './Icon'
import { errorText } from '../lib/money'
import { fileUrl, uploadFile, useVehicleImages } from '../lib/files'
import type { Vehicle } from '../lib/state'

export function VehiclePhoto({ vehicle, size = 'preview' }: { vehicle: Vehicle; size?: 'thumbnail' | 'preview' }) {
  const qc = useQueryClient()
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string>()
  const images = useVehicleImages(vehicle.id).data?.items ?? []
  const primary = images.find((i) => i.primary) ?? images[0]
  const canEdit = vehicle.my_role === 'owner' || vehicle.my_role === 'editor'
  async function change(file: File) {
    setBusy(true)
    setError(undefined)
    try {
      const r = await uploadFile(vehicle.id, file, 'gallery')
      await api.post(`/vehicles/${vehicle.id}/images`, { file_id: r.file.id, primary: true })
      qc.invalidateQueries({ queryKey: ['files', vehicle.id] })
    } catch (e) { setError(errorText(e)) } finally { setBusy(false) }
  }
  return (
    <div className="group relative flex h-full w-full items-center justify-center overflow-hidden bg-soft text-muted">
      {primary ? <img src={fileUrl(vehicle.id, primary.file_id, size)} alt={`Bild von ${vehicle.display_name}`} className="h-full w-full object-cover" /> : <Icon name="car" size={64} />}
      {canEdit && (
        <button type="button" disabled={busy} onClick={(e) => { e.stopPropagation(); input.current?.click() }}
          className="absolute right-3 bottom-3 flex h-9 items-center gap-1.5 rounded-[10px] bg-ink/70 px-3 text-xs font-semibold text-paper opacity-90 hover:opacity-100">
          <Icon name="camera" size={16} />{busy ? 'Lädt …' : primary ? 'Bild ändern' : 'Bild hinzufügen'}
        </button>
      )}
      {error && <div role="alert" className="absolute inset-x-3 top-12 rounded-[8px] bg-card px-2 py-1 text-xs font-semibold text-bad">{error}</div>}
      <input ref={input} type="file" accept="image/jpeg,image/png,image/webp" hidden onClick={(e) => e.stopPropagation()}
        onChange={(e) => { const f = e.target.files?.[0]; if (f) change(f); e.target.value = '' }} />
    </div>
  )
}
