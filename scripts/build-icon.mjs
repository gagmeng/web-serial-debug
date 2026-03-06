import sharp from 'sharp'
import toIco from 'to-ico'
import { readFileSync, writeFileSync, mkdirSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const srcPng = resolve(__dirname, '../window/icon-src.png')
const destIco = resolve(__dirname, '../window/app.ico')

const sizes = [16, 32, 48, 256]

async function main() {
  const meta = await sharp(srcPng).metadata()
  const side = Math.min(meta.width, meta.height)
  const left = Math.floor((meta.width - side) / 2)
  const top = Math.floor((meta.height - side) / 2)

  const pngBuffers = await Promise.all(
    sizes.map(size =>
      sharp(srcPng)
        .extract({ left, top, width: side, height: side })
        .resize(size, size, { fit: 'fill' })
        .png()
        .toBuffer()
    )
  )

  const icoBuffer = await toIco(pngBuffers)
  writeFileSync(destIco, icoBuffer)
  console.log(`Icon written to ${destIco}`)
  console.log(`Sizes: ${sizes.join(', ')} px`)
}

main().catch(e => { console.error(e); process.exit(1) })
