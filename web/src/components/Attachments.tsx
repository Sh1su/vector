// Belege an einem Eintrag (DO-01): Datei hochladen und verknüpfen oder vorhandenes Dokument anhängen.
import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Schemas } from '../api/client'
import { Icon } from './Icon'
import { Button } from './ui'
import { errorText } from '../lib/money'
import { fileUrl, hasPreview, uploadFile, useFiles, type Doc } from '../lib/files'

export function Attachments({ vid, targetType, targetId, role, canEdit }: { vid: string; targetType: string; targetId: string; role: string; canEdit: boolean }) {
  const qc = useQueryClient()
  const input = useRef<HTMLInputElement>(null)
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const key = ['files', vid, 'attachments', targetType, targetId]
  const atts = useQuery({ queryKey: key, queryFn: () => api.get<{ items: Schemas['Attachment'][] }>(`/vehicles/${vid}/attachments?target_type=${targetType}&target_id=${targetId}`) })
  const files = useFiles(vid).data?.items ?? []
  const docs = useQuery({ queryKey: ['files', vid, 'docs', ''], queryFn: () => api.get<{ items: Doc[] }>(`/vehicles/${vid}/documents?limit=500`) }).data?.items ?? []
  const attach = (body: object) => api.post(`/vehicles/${vid}/attachments`, { target_type: targetType, target_id: targetId, role, ...body })
  const detach = useMutation({ mutationFn: (id: string) => api.del(`/vehicles/${vid}/attachments/${id}`, '"1"'), onSuccess: () => qc.invalidateQueries({ queryKey: key }) })
  async function add(list: FileList) {
    setBusy(true)
    setError(undefined)
    try {
      for (const f of Array.from(list)) {
        const r = await uploadFile(vid, f)
        await attach({ file_id: r.file.id })
      }
      qc.invalidateQueries({ queryKey: ['files', vid] })
    } catch (e) { setError(errorText(e)) } finally { setBusy(false) }
  }
  const items = atts.data?.items ?? []
  return (
    <fieldset className="m-0 flex flex-col gap-2 border-0 p-0">
      <legend className="mb-1.5 text-sm font-semibold">Belege</legend>
      {items.length === 0 && <div className="text-xs text-muted">Noch kein Beleg angehängt.</div>}
      {items.map((a) => {
        const file = a.file_id ? files.find((f) => f.id === a.file_id) : undefined
        const doc = a.document_id ? docs.find((d) => d.id === a.document_id) : undefined
        const fid = a.file_id ?? doc?.file_ids[0]
        return (
          <div key={a.id} className="flex items-center gap-2.5 rounded-[12px] border border-line px-2.5 py-1.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-[8px] bg-soft text-muted">
              {fid && hasPreview(file) ? <img src={fileUrl(vid, fid, 'thumbnail')} alt="" className="h-full w-full object-cover" /> : <Icon name="doc" size={16} />}
            </div>
            {fid ? <a href={fileUrl(vid, fid)} className="min-w-0 flex-grow truncate text-sm text-text hover:underline">{doc?.title ?? file?.original_name ?? 'Datei'}</a>
              : <span className="flex-grow text-sm">{doc?.title}</span>}
            {canEdit && <button type="button" aria-label="Beleg lösen" onClick={() => detach.mutate(a.id!)} className="flex h-8 w-8 items-center justify-center rounded-[8px] text-muted hover:bg-soft"><Icon name="x" size={14} /></button>}
          </div>
        )
      })}
      {canEdit && (
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" variant="outline" icon="scan" disabled={busy} onClick={() => input.current?.click()}>{busy ? 'Lade hoch …' : 'Beleg hochladen'}</Button>
          {docs.length > 0 && (
            <select aria-label="Dokument anhängen" className="h-10 rounded-[12px] border-[1.5px] border-line bg-card px-3 text-sm" value=""
              onChange={async (e) => { if (!e.target.value) return; try { await attach({ document_id: e.target.value }); qc.invalidateQueries({ queryKey: key }) } catch (x) { setError(errorText(x)) } }}>
              <option value="">Dokument aus der Akte …</option>
              {docs.map((d) => <option key={d.id} value={d.id}>{d.title}</option>)}
            </select>
          )}
          <input ref={input} type="file" multiple hidden accept="application/pdf,image/*" onChange={(e) => { if (e.target.files?.length) add(e.target.files); e.target.value = '' }} />
        </div>
      )}
      {error && <div role="alert" className="text-xs font-semibold text-bad">{error}</div>}
    </fieldset>
  )
}
