import { useQuery } from '@tanstack/react-query'
import { api, upload, type Schemas } from '../api/client'

export type FileMeta = Schemas['FileMeta']
export type Doc = Schemas['Document'] & { id: string; version: number }

export const fileUrl = (vid: string, fid: string, kind: 'content' | 'original' | 'thumbnail' | 'preview' = 'content') =>
  kind === 'thumbnail' || kind === 'preview' ? `/api/v1/vehicles/${vid}/files/${fid}/preview?size=${kind}` : `/api/v1/vehicles/${vid}/files/${fid}/${kind}`

export const hasPreview = (f?: FileMeta) => !!f?.derivatives?.some((d) => d.kind === 'thumbnail')

export function useFiles(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: ['files', vid, 'list'], queryFn: () => api.get<{ items: FileMeta[] }>(`/vehicles/${vid}/files?limit=200`) })
}

export function useVehicleImages(vid?: string) {
  return useQuery({ enabled: !!vid, queryKey: ['files', vid, 'images'], queryFn: () => api.get<{ items: Schemas['VehicleImage'][] }>(`/vehicles/${vid}/images`) })
}

/** Datei hochladen; bei Dublette (DO-02) liefert der Server die vorhandene Datei mit duplicate_of. */
export async function uploadFile(vid: string, file: File, captureSource = 'upload') {
  const r = await upload<FileMeta>(`/vehicles/${vid}/files`, file, file.name, { capture_source: captureSource })
  return { file: r.data, duplicate: r.status === 200 && !!r.data.duplicate_of }
}

export const fmtSize = (b: number) => b < 1024 ? `${b} B` : b < 1 << 20 ? `${(b / 1024).toFixed(0)} KB` : `${(b / (1 << 20)).toFixed(1).replace('.', ',')} MB`

export const docTypeLabel: Record<string, string> = {
  maintenance_manual: 'Wartungshandbuch', owner_manual: 'Bedienungsanleitung', technical_doc: 'Technische Dokumentation', service_book: 'Checkheft',
  invoice: 'Rechnung', workshop_report: 'Werkstattbericht', inspection_report: 'HU/TÜV-Bericht', registration: 'Zulassung', insurance: 'Versicherung', other: 'Sonstiges',
}

export const natureLabel: Record<string, string> = { specification: 'Herstellervorgabe', record: 'Nachweis', other: 'Sonstiges' }
