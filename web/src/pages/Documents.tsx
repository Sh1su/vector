import { useRef, useState, type DragEvent, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { Button, Card, Chip, EmptyState, Field, inputClass } from '../components/ui'
import { errorText, etagOf, fmtDay } from '../lib/money'
import { docTypeLabel, fileUrl, fmtSize, hasPreview, natureLabel, uploadFile, useFiles, type Doc, type FileMeta } from '../lib/files'
import { useApp } from '../lib/state'

export function DocumentsPage() {
  const { vehicle } = useApp()
  const vid = vehicle?.id
  const qc = useQueryClient()
  const [tab, setTab] = useState<'docs' | 'files'>('docs')
  const [q, setQ] = useState('')
  const [type, setType] = useState('')
  const [editing, setEditing] = useState<Doc | 'new' | null>(null)
  const [preselect, setPreselect] = useState<string[]>([])
  const docs = useQuery({
    enabled: !!vid, queryKey: ['files', vid, 'docs', type],
    queryFn: () => api.get<{ items: Doc[] }>(`/vehicles/${vid}/documents?limit=500${type ? '&doc_type=' + type : ''}`),
  })
  const files = useFiles(vid)
  const canEdit = vehicle?.my_role === 'owner' || vehicle?.my_role === 'editor'
  const [notice, setNotice] = useState<string>()
  const byId = new Map((files.data?.items ?? []).map((f) => [f.id, f]))
  const setImage = useMutation({
    mutationFn: (fid: string) => api.post(`/vehicles/${vid}/images`, { file_id: fid, primary: true }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['files', vid] }); setNotice('Als Fahrzeugbild gesetzt.') },
    onError: (e) => setNotice(errorText(e)),
  })
  const del = useMutation({
    mutationFn: (fid: string) => api.del(`/vehicles/${vid}/files/${fid}`, '"1"'),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['files', vid] }), onError: (e) => setNotice(errorText(e)),
  })

  if (!vehicle) return <><Header title="Dokumente" /><main className="p-8"><EmptyState icon="car" title="Noch kein Fahrzeug" text="Lege zuerst ein Fahrzeug an." /></main></>
  const ql = q.trim().toLowerCase()
  const list = (docs.data?.items ?? []).filter((d) => !ql || [d.title, d.issuer, d.note, ...(d.tags ?? [])].some((x) => x?.toLowerCase().includes(ql)))

  async function quickUpload(fl: FileList | File[]) {
    const ids: string[] = []
    const dups: string[] = []
    for (const f of Array.from(fl)) {
      try {
        const r = await uploadFile(vehicle!.id, f)
        ids.push(r.file.id)
        if (r.duplicate) dups.push(f.name)
      } catch (e) { setNotice(`${f.name}: ${errorText(e)}`); return }
    }
    qc.invalidateQueries({ queryKey: ['files', vid] })
    setNotice(dups.length ? `Bereits vorhanden: ${dups.join(', ')} – die vorhandene Datei wird verwendet.` : undefined)
    setPreselect(ids)
    setEditing('new')
  }

  return (
    <>
      <Header title="Dokumente" sub={`${vehicle.display_name} · digitale Fahrzeugakte`}
        actions={canEdit && <Button icon="plus" onClick={() => { setPreselect([]); setEditing('new') }}>Dokument hinzufügen</Button>} />
      <main className="flex min-h-0 flex-grow flex-col gap-5 overflow-y-auto px-8 py-6">
        {canEdit && <DropZone onFiles={quickUpload} />}
        {notice && <div role="status" className="flex items-center justify-between gap-3 rounded-[12px] bg-info-bg px-3.5 py-2.5 text-sm"><span>{notice}</span>
          <button type="button" aria-label="Hinweis schließen" onClick={() => setNotice(undefined)} className="text-muted"><Icon name="x" size={16} /></button></div>}
        <div className="flex flex-wrap items-center gap-3">
          <div role="tablist" className="flex rounded-[12px] bg-soft p-1">
            {([['docs', 'Fahrzeugakte'], ['files', 'Alle Dateien']] as const).map(([k, l]) => (
              <button key={k} role="tab" type="button" aria-selected={tab === k} onClick={() => setTab(k)}
                className={`h-9 rounded-[10px] px-4 text-sm font-semibold ${tab === k ? 'bg-card text-text shadow-sm' : 'text-muted'}`}>{l}</button>
            ))}
          </div>
          {tab === 'docs' && <>
            <div className="relative min-w-[220px] flex-grow">
              <span className="absolute top-1/2 left-3.5 -translate-y-1/2 text-muted"><Icon name="search" size={18} /></span>
              <input aria-label="Dokumente durchsuchen" className={inputClass + ' h-11 pl-10'} placeholder="Titel, Aussteller, Schlagwort …" value={q} onChange={(e) => setQ(e.target.value)} />
            </div>
            <select aria-label="Dokumenttyp" className={inputClass + ' h-11 w-auto'} value={type} onChange={(e) => setType(e.target.value)}>
              <option value="">Alle Typen</option>
              {Object.entries(docTypeLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}
            </select>
          </>}
        </div>

        {tab === 'docs' && list.length === 0 && !docs.isPending && (
          <EmptyState icon="doc" title="Die Fahrzeugakte ist leer" text="Lade Rechnungen, HU-Berichte, Checkheft, Zulassung oder das Wartungshandbuch hoch – als PDF oder Foto." />
        )}
        {tab === 'docs' && list.length > 0 && (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {list.map((d) => {
              const first = byId.get(d.file_ids[0])
              return (
                <button key={d.id} type="button" onClick={() => setEditing(d)} className="flex overflow-hidden rounded-[16px] border border-line bg-card text-left text-text hover:border-teal">
                  <div className="flex h-[112px] w-[96px] shrink-0 items-center justify-center bg-soft text-muted">
                    {hasPreview(first) ? <img src={fileUrl(vehicle.id, first!.id, 'thumbnail')} alt="" className="h-full w-full object-cover" /> : <Icon name="doc" size={34} />}
                  </div>
                  <div className="flex min-w-0 flex-col gap-1 p-3.5">
                    <div className="truncate text-[15px] font-semibold">{d.title}</div>
                    <div className="flex flex-wrap gap-1.5"><Chip tone="info">{docTypeLabel[d.doc_type]}</Chip>{d.nature === 'specification' && <Chip>{natureLabel.specification}</Chip>}</div>
                    <div className="truncate text-xs text-muted">{[d.document_date && fmtDay(d.document_date), d.issuer, `${d.file_ids.length} ${d.file_ids.length === 1 ? 'Datei' : 'Dateien'}`].filter(Boolean).join(' · ')}</div>
                  </div>
                </button>
              )
            })}
          </div>
        )}

        {tab === 'files' && (
          <Card className="overflow-hidden">
            {(files.data?.items.length ?? 0) === 0 && <div className="px-5 py-8 text-sm text-muted">Noch keine Dateien.</div>}
            {files.data?.items.map((f, i) => (
              <div key={f.id} className={`grid grid-cols-[56px_minmax(0,1fr)_auto] items-center gap-3 px-5 py-2.5 ${i ? 'border-t border-line' : ''}`}>
                <div className="flex h-12 w-12 items-center justify-center overflow-hidden rounded-[10px] bg-soft text-muted">
                  {hasPreview(f) ? <img src={fileUrl(vehicle.id, f.id, 'thumbnail')} alt="" className="h-full w-full object-cover" /> : <Icon name="doc" />}
                </div>
                <div className="flex min-w-0 flex-col">
                  <a href={fileUrl(vehicle.id, f.id)} className="truncate text-sm font-semibold text-text hover:underline">{f.original_name}</a>
                  <span className="text-xs text-muted">{fmtDay(f.received_at)} · {fmtSize(f.size_bytes)} · {f.media_type}{f.has_location ? ' · enthält Standort (nur im Original)' : ''}</span>
                </div>
                <div className="flex items-center gap-1">
                  {canEdit && hasPreview(f) && <Button variant="ghost" onClick={() => setImage.mutate(f.id)}>Als Fahrzeugbild</Button>}
                  {canEdit && <a href={fileUrl(vehicle.id, f.id, 'original')} className="flex h-9 items-center rounded-[10px] px-3 text-sm font-semibold text-link hover:bg-soft">Original</a>}
                  {canEdit && <button type="button" aria-label="Datei löschen" onClick={() => del.mutate(f.id)} className="flex h-9 w-9 items-center justify-center rounded-[10px] text-muted hover:bg-soft"><Icon name="x" size={16} /></button>}
                </div>
              </div>
            ))}
          </Card>
        )}
      </main>
      {editing && <DocDialog vid={vehicle.id} doc={editing === 'new' ? null : editing} files={files.data?.items ?? []} preselect={preselect} canEdit={canEdit}
        onClose={() => setEditing(null)} />}
    </>
  )
}

export function DropZone({ onFiles, compact }: { onFiles: (f: File[]) => void; compact?: boolean }) {
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const drop = (e: DragEvent) => { e.preventDefault(); setOver(false); if (e.dataTransfer.files.length) onFiles(Array.from(e.dataTransfer.files)) }
  return (
    <div onDragOver={(e) => { e.preventDefault(); setOver(true) }} onDragLeave={() => setOver(false)} onDrop={drop}
      className={`flex items-center gap-3 rounded-[16px] border-[1.5px] border-dashed px-4 ${compact ? 'py-2.5' : 'py-4'} ${over ? 'border-teal bg-info-bg' : 'border-line'}`}>
      <span className="text-link"><Icon name="scan" size={compact ? 20 : 26} /></span>
      <div className="flex flex-grow flex-col">
        <span className="text-sm font-semibold">Dateien hierher ziehen</span>
        {!compact && <span className="text-xs text-muted">PDF, JPEG, PNG, WebP oder HEIC – geprüft am Inhalt, Fotos werden ohne Standortdaten angezeigt.</span>}
      </div>
      <Button type="button" variant="outline" onClick={() => input.current?.click()}>Auswählen</Button>
      <input ref={input} type="file" multiple hidden accept="application/pdf,image/*,text/plain" onChange={(e) => { if (e.target.files?.length) onFiles(Array.from(e.target.files)); e.target.value = '' }} />
    </div>
  )
}

function DocDialog({ vid, doc, files, preselect, canEdit, onClose }: { vid: string; doc: Doc | null; files: FileMeta[]; preselect: string[]; canEdit: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState({
    title: doc?.title ?? '', type: doc?.doc_type ?? 'invoice', date: doc?.document_date ?? '', issuer: doc?.issuer ?? '', note: doc?.note ?? '',
    tags: (doc?.tags ?? []).join(', '),
  })
  const [pages, setPages] = useState<string[]>(doc?.file_ids ?? preselect)
  const [extra, setExtra] = useState<FileMeta[]>([])
  const [error, setError] = useState<string>()
  const all = new Map([...files, ...extra].map((x) => [x.id, x]))
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const done = () => { qc.invalidateQueries({ queryKey: ['files', vid] }); onClose() }
  const save = useMutation({
    mutationFn: () => {
      const body = { title: f.title || all.get(pages[0])?.original_name || 'Dokument', doc_type: f.type, document_date: f.date || null, issuer: f.issuer || null,
        note: f.note, tags: f.tags.split(',').map((t) => t.trim()).filter(Boolean), file_ids: pages }
      return doc ? api.patch(`/vehicles/${vid}/documents/${doc.id}`, body, etagOf(doc)) : api.post(`/vehicles/${vid}/documents`, body)
    },
    onSuccess: done, onError: (e) => setError(errorText(e)),
  })
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vid}/documents/${doc!.id}`, etagOf(doc!)), onSuccess: done, onError: (e) => setError(errorText(e)) })
  async function add(fl: File[]) {
    for (const file of fl) {
      try {
        const r = await uploadFile(vid, file)
        setExtra((x) => [...x, r.file])
        setPages((p) => (p.includes(r.file.id) ? p : [...p, r.file.id]))
      } catch (e) { setError(`${file.name}: ${errorText(e)}`) }
    }
  }
  const move = (i: number, d: number) => setPages((p) => { const n = [...p]; [n[i], n[i + d]] = [n[i + d], n[i]]; return n })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={doc ? doc.title : 'Dokument hinzufügen'}>
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); save.mutate() }}>
        <fieldset disabled={!canEdit} className="m-0 flex flex-col gap-4 border-0 p-0">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Titel" htmlFor="d-title"><input id="d-title" className={inputClass} value={f.title} onChange={set('title')} placeholder="Rechnung Inspektion" /></Field>
            <Field label="Typ" htmlFor="d-type">
              <select id="d-type" className={inputClass} value={f.type} onChange={set('type')}>{Object.entries(docTypeLabel).map(([k, l]) => <option key={k} value={k}>{l}</option>)}</select>
            </Field>
            <Field label="Datum" htmlFor="d-date"><input id="d-date" type="date" className={inputClass} value={f.date} onChange={set('date')} /></Field>
            <Field label="Aussteller" htmlFor="d-issuer"><input id="d-issuer" className={inputClass} value={f.issuer} onChange={set('issuer')} placeholder="Werkstatt, Versicherung …" /></Field>
          </div>
          <Field label="Schlagwörter" htmlFor="d-tags" hint="durch Komma getrennt"><input id="d-tags" className={inputClass} value={f.tags} onChange={set('tags')} /></Field>
        </fieldset>
        <div className="flex flex-col gap-2">
          <div className="text-sm font-semibold">Dateien (Seitenreihenfolge)</div>
          {pages.map((id, i) => {
            const file = all.get(id)
            return (
              <div key={id} className="flex items-center gap-2.5 rounded-[12px] border border-line px-2.5 py-1.5">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-[8px] bg-soft text-muted">
                  {hasPreview(file) ? <img src={fileUrl(vid, id, 'thumbnail')} alt="" className="h-full w-full object-cover" /> : <Icon name="doc" size={18} />}
                </div>
                <a href={fileUrl(vid, id)} className="min-w-0 flex-grow truncate text-sm text-text hover:underline">{i + 1}. {file?.original_name ?? 'Datei'}</a>
                {canEdit && <>
                  <button type="button" aria-label="Nach oben" disabled={i === 0} onClick={() => move(i, -1)} className="h-8 w-8 rounded-[8px] text-muted hover:bg-soft disabled:opacity-30">↑</button>
                  <button type="button" aria-label="Nach unten" disabled={i === pages.length - 1} onClick={() => move(i, 1)} className="h-8 w-8 rounded-[8px] text-muted hover:bg-soft disabled:opacity-30">↓</button>
                  <button type="button" aria-label="Datei entfernen" onClick={() => setPages(pages.filter((x) => x !== id))} className="flex h-8 w-8 items-center justify-center rounded-[8px] text-muted hover:bg-soft"><Icon name="x" size={14} /></button>
                </>}
              </div>
            )
          })}
          {canEdit && <DropZone compact onFiles={add} />}
          {canEdit && files.filter((x) => !pages.includes(x.id)).length > 0 && (
            <select aria-label="Vorhandene Datei hinzufügen" className={inputClass + ' h-10 text-sm'} value="" onChange={(e) => e.target.value && setPages([...pages, e.target.value])}>
              <option value="">Vorhandene Datei hinzufügen …</option>
              {files.filter((x) => !pages.includes(x.id)).map((x) => <option key={x.id} value={x.id}>{x.original_name}</option>)}
            </select>
          )}
        </div>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        {canEdit && (
          <div className="flex flex-wrap justify-between gap-3">
            {doc ? <Button type="button" variant="outline" disabled={remove.isPending} onClick={() => remove.mutate()}>Löschen</Button> : <span />}
            <Button type="submit" disabled={save.isPending || pages.length === 0}>Speichern</Button>
          </div>
        )}
      </form>
    </Dialog>
  )
}
