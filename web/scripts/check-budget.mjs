// Budget-Prüfung (Auftrag 5.3, ADR-022): initiales JS < 300 KB gzip.
import { readFileSync, readdirSync } from 'node:fs'
import { gzipSync } from 'node:zlib'
import { join } from 'node:path'

const dist = 'dist'
const html = readFileSync(join(dist, 'index.html'), 'utf8')
const initial = [...html.matchAll(/(?:src|href)="\/(assets\/[^"]+\.js)"/g)].map((m) => m[1])
let total = 0
for (const f of initial) total += gzipSync(readFileSync(join(dist, f)), { level: 9 }).length
const all = readdirSync(join(dist, 'assets')).filter((f) => f.endsWith('.js'))
const kb = (n) => (n / 1024).toFixed(1)
console.log(`initiales JS: ${kb(total)} KB gzip (${initial.length} Dateien), JS-Dateien gesamt: ${all.length}`)
if (total > 300 * 1024) {
  console.error('Budget überschritten: initiales JS muss unter 300 KB gzip bleiben.')
  process.exit(1)
}
