import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ProblemError, type Anomaly, type Schemas } from '../api/client'
import { Header } from '../components/Header'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { PlausibilityAlert } from '../components/Plausibility'
import { Button, Chip, Field, inputClass } from '../components/ui'
import { fmtNumber } from '../lib/format'
import { useCurrent } from '../lib/odometer'
import { useApp, type Vehicle } from '../lib/state'
import { VehiclePhoto } from '../components/VehiclePhoto'

const roleLabel: Record<string, string> = { owner: 'Eigentümer', editor: 'Bearbeiter', viewer: 'Leser' }
const carrierLabel: Record<string, string> = { petrol: 'Benzin', diesel: 'Diesel', lpg: 'Autogas', electricity: 'Strom' }

function VehicleCard({ v }: { v: Vehicle }) {
  const { setVehicleId } = useApp()
  const navigate = useNavigate()
  const cur = useCurrent(v.id).data
  const status = v.status === 'active' ? { tone: 'ok' as const, icon: 'check' as const, text: 'Aktiv' } : { tone: 'neutral' as const, icon: 'info' as const, text: v.status === 'sold' ? 'Verkauft' : 'Archiviert' }
  return (
    <div className="flex flex-col overflow-hidden rounded-[16px] border border-line bg-card text-left text-text hover:border-teal">
      <div className="relative h-[170px] w-full">
        <VehiclePhoto vehicle={v} />
        <span className="absolute top-3 left-3"><Chip tone={status.tone} icon={status.icon}>{status.text}</Chip></span>
      </div>
      <button type="button" onClick={() => { setVehicleId(v.id); navigate('/') }} aria-label={`${v.display_name} öffnen`}
        className="flex flex-col gap-3 px-4.5 py-4 text-left text-text">
        <div className="flex flex-col gap-0.5">
          <div className="font-display text-lg font-semibold">{v.display_name}</div>
          <div className="text-[13px] text-muted">{[v.license_plate, v.energy_carriers.map((c) => carrierLabel[c]).join(' / '), v.model_year ? `Baujahr ${v.model_year}` : ''].filter(Boolean).join(' · ')}</div>
        </div>
        <div className="grid grid-cols-2 gap-2 text-[13px]">
          <div className="flex items-center gap-1.5"><span className="text-muted"><Icon name="gauge" size={18} /></span>
            <span className="tabular font-semibold">{cur?.meter_value ? `${fmtNumber(cur.meter_value.canonical / 1000)} km` : 'kein Stand'}</span></div>
          <div className="flex items-center gap-1.5"><span className="text-muted"><Icon name="user" size={18} /></span>{roleLabel[v.my_role ?? 'viewer']}</div>
        </div>
      </button>
    </div>
  )
}

const emptyForm = { display_name: '', make: '', model: '', model_year: '', license_plate: '', vin: '', body_type: 'car', usage_meter: 'distance', energy_carriers: ['petrol'] as string[] }

export function NewVehicleDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const qc = useQueryClient()
  const { setVehicleId } = useApp()
  const [f, setF] = useState(emptyForm)
  const [anomalies, setAnomalies] = useState<Anomaly[] | null>(null)
  const [error, setError] = useState<string>()
  const create = useMutation({
    mutationFn: (extra: object) => {
      const body: Record<string, unknown> = { display_name: f.display_name || [f.make, f.model].filter(Boolean).join(' '), body_type: f.body_type, usage_meter: f.usage_meter, energy_carriers: f.energy_carriers, ...extra }
      if (f.make) body.make = f.make
      if (f.model) body.model = f.model
      if (f.model_year) body.model_year = Number(f.model_year)
      if (f.license_plate) body.license_plate = f.license_plate
      if (f.vin) body.vin = f.vin
      return api.post<Schemas['Vehicle']>('/vehicles', body)
    },
    onSuccess: (v) => { qc.invalidateQueries({ queryKey: ['vehicles'] }); setVehicleId(v.id); setF(emptyForm); setAnomalies(null); onOpenChange(false) },
    onError: (e) => {
      if (e instanceof ProblemError && e.isPlausibility) setAnomalies(e.problem.anomalies!)
      else setError(e instanceof ProblemError ? (e.problem.errors?.map((x) => x.message || x.pointer).join(', ') || e.problem.title) : 'Speichern fehlgeschlagen.')
    },
  })
  const set = (k: keyof typeof emptyForm) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const toggleCarrier = (c: string) => setF({ ...f, energy_carriers: f.energy_carriers.includes(c) ? f.energy_carriers.filter((x) => x !== c) : [...f.energy_carriers, c] })

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title="Fahrzeug hinzufügen">
      <form className="flex flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); setError(undefined); setAnomalies(null); create.mutate({}) }}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Hersteller" htmlFor="v-make"><input id="v-make" className={inputClass} value={f.make} onChange={set('make')} placeholder="VW" /></Field>
          <Field label="Modell" htmlFor="v-model"><input id="v-model" className={inputClass} value={f.model} onChange={set('model')} placeholder="Golf VII" /></Field>
          <Field label="Anzeigename" htmlFor="v-name" hint="Leer = Hersteller und Modell"><input id="v-name" className={inputClass} value={f.display_name} onChange={set('display_name')} /></Field>
          <Field label="Baujahr" htmlFor="v-year"><input id="v-year" inputMode="numeric" className={inputClass} value={f.model_year} onChange={set('model_year')} /></Field>
          <Field label="Kennzeichen" htmlFor="v-plate"><input id="v-plate" className={inputClass} value={f.license_plate} onChange={set('license_plate')} /></Field>
          <Field label="FIN" htmlFor="v-vin"><input id="v-vin" className={inputClass + ' uppercase'} value={f.vin} onChange={set('vin')} /></Field>
          <Field label="Fahrzeugart" htmlFor="v-body">
            <select id="v-body" className={inputClass} value={f.body_type} onChange={set('body_type')}>
              {[['car', 'Pkw'], ['motorcycle', 'Motorrad'], ['van', 'Transporter'], ['truck', 'Lkw'], ['camper', 'Wohnmobil'], ['trailer', 'Anhänger'], ['tractor', 'Traktor'], ['boat', 'Boot'], ['other', 'Sonstiges']].map(([k, l]) => <option key={k} value={k}>{l}</option>)}
            </select>
          </Field>
          <Field label="Zähler" htmlFor="v-meter">
            <select id="v-meter" className={inputClass} value={f.usage_meter} onChange={set('usage_meter')}>
              <option value="distance">Kilometer</option><option value="engine_hours">Betriebsstunden</option>
            </select>
          </Field>
        </div>
        <fieldset className="m-0 flex flex-wrap gap-3 border-0 p-0">
          <legend className="mb-1.5 text-sm font-semibold">Energieträger</legend>
          {Object.entries(carrierLabel).map(([k, l]) => (
            <label key={k} className="flex h-10 items-center gap-2 rounded-[10px] border border-line px-3 text-sm">
              <input type="checkbox" className="h-4 w-4 accent-teal" checked={f.energy_carriers.includes(k)} onChange={() => toggleCarrier(k)} />{l}
            </label>
          ))}
        </fieldset>
        {error && <div role="alert" className="text-sm font-semibold text-bad">{error}</div>}
        {anomalies && <PlausibilityAlert anomalies={anomalies} busy={create.isPending} onEdit={() => setAnomalies(null)} onConfirm={(codes, r) => create.mutate({ confirm_anomalies: codes, anomaly_reason: r })} />}
        {!anomalies && <Button type="submit" disabled={create.isPending || !(f.display_name || f.make || f.model)} className="self-end">Fahrzeug anlegen</Button>}
      </form>
    </Dialog>
  )
}

export function VehiclesPage() {
  const { vehicles } = useApp()
  const [open, setOpen] = useState(false)
  return (
    <>
      <Header title="Fahrzeuge" sub={vehicles.length === 1 ? '1 Fahrzeug in deiner Garage' : `${vehicles.length} Fahrzeuge in deiner Garage`}
        actions={<Button icon="plus" onClick={() => setOpen(true)}>Fahrzeug hinzufügen</Button>} />
      <main className="flex flex-col gap-5 overflow-y-auto px-8 py-6">
        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
          {vehicles.map((v) => <VehicleCard key={v.id} v={v} />)}
          <button type="button" onClick={() => setOpen(true)} className="flex min-h-[300px] flex-col items-center justify-center gap-2.5 rounded-[16px] border-[1.5px] border-dashed border-line text-text hover:border-teal">
            <div className="flex h-13 w-13 items-center justify-center rounded-[16px] bg-info-bg text-link"><Icon name="plus" size={26} /></div>
            <div className="text-[15px] font-semibold">Fahrzeug hinzufügen</div>
            <div className="max-w-[240px] text-center text-[13px] leading-normal text-muted">Stammdaten von Hand eingeben.</div>
          </button>
        </div>
        <div className="flex items-start gap-2.5 rounded-[12px] bg-info-bg px-3.5 py-3 text-[13px] leading-normal">
          <span className="text-link"><Icon name="users" size={18} /></span>
          <div>Fahrzeuge lassen sich teilen. Jede Person bekommt pro Fahrzeug eine Rolle: <b>Eigentümer</b> (alles, inkl. Freigaben), <b>Bearbeiter</b> (Einträge anlegen und ändern) oder <b>Leser</b> (nur ansehen).</div>
        </div>
      </main>
      <NewVehicleDialog open={open} onOpenChange={setOpen} />
    </>
  )
}
